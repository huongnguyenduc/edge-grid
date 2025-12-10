package p2p

import (
	"context"
	pb "edge-grid/api/proto"
	"edge-grid/internal/api"
	"edge-grid/internal/runtime"
	"edge-grid/internal/storage"
	"edge-grid/pkg/logger"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

const ProtocolID = "/edge-grid/1.0.0"

type PeerMetric struct {
	CPUUsage   float64
	RamUsage   uint64
	LastSeen   int64
	Reputation int
}

type Node struct {
	Host        host.Host
	Store       *storage.Store
	Hub         *api.Hub
	PrivKey     crypto.PrivKey
	PeerStats   sync.Map // Map[peer.ID]PeerMetric
	IsMalicious bool
}

func NewNode(listenPort int, dbPath string, hub *api.Hub, malicious bool) (*Node, error) {
	priv, _, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
	if err != nil {
		return nil, err
	}

	h, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic-v1", listenPort),
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort), // TCP fallback
		),

		// 1. NAT Port Mapping (UPnP)
		// Try to open port automatically
		libp2p.EnableNATService(),

		// 2. AutoNAT
		// Help node to ask "What is my public IP?"
		libp2p.EnableAutoNATv2(),

		// 3. Hole Punching
		// Technique to punch a hole to allow 2 nodes after NAT to connect directly
		libp2p.EnableHolePunching(),

		// 4. Circuit Relay (Client)
		// If hole punching fails, allow connection to go through public Relay Node
		libp2p.EnableRelay(),
	)

	if err != nil {
		return nil, err
	}

	// 1. Initialize Storage
	store, err := storage.NewStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to init db: %v", err)
	}

	node := &Node{
		Host:        h,
		Store:       store,
		Hub:         hub,
		PrivKey:     priv,
		IsMalicious: malicious,
	}

	h.SetStreamHandler(ProtocolID, node.handleStream)

	sub, err := h.EventBus().Subscribe(new(event.EvtLocalReachabilityChanged))
	if err != nil {
		logger.Log.Error("Failed to subscribe to reachability events", zap.Error(err))
	} else {
		go func() {
			defer sub.Close()
			for e := range sub.Out() {
				evt := e.(event.EvtLocalReachabilityChanged)
				// Reachability has 3 states: Unknown, Public, Private
				status := "Unknown"
				switch evt.Reachability {
				case network.ReachabilityPublic:
					status = "🌍 PUBLIC (Internet Accessible)"
				case network.ReachabilityPrivate:
					status = "🔒 PRIVATE (Behind NAT)"
				}
				logger.Log.Info("Network Status Changed", zap.String("status", status))
			}
		}()
	}

	return node, nil
}

// handleStream: Receive Task -> Run -> Return Result
func (n *Node) handleStream(s network.Stream) {
	defer s.Close()

	data, err := io.ReadAll(s)
	if err != nil {
		return
	}

	req := &pb.TaskRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return
	}

	logger.Log.Info("Verifying signature", zap.String("task_id", req.TaskId))

	// Parse Public Key from bytes
	senderPubKey, err := crypto.UnmarshalPublicKey(req.PublicKey)
	if err != nil {
		logger.Log.Error("Auth Failed: Invalid Public Key", zap.Error(err))
		return
	}

	// Reconstruct original data (Wasm + Input)
	dataToVerify := append(req.WasmBinary, req.InputData...)

	// Verify Signature
	valid, err := senderPubKey.Verify(dataToVerify, req.Signature)
	if err != nil || !valid {
		logger.Log.Error("Security Alert: Invalid Signature! Task rejected.")
		resp := &pb.TaskResponse{
			TaskId:   req.TaskId,
			WorkerId: n.Host.ID().String(),
			Error:    "Invalid Signature",
		}
		respBytes, _ := proto.Marshal(resp)
		s.Write(respBytes)
		return
	}

	logger.Log.Info("Signature Valid. Sender is authenticated.")

	senderID := s.Conn().RemotePeer().String()
	logger.Log.Info("Processing Task",
		zap.String("task_id", req.TaskId),
		zap.String("sender_id", senderID),
		zap.Int("wasm_size", len(req.WasmBinary)),
		zap.Int("input_size", len(req.InputData)),
	)

	// Check if task already exists
	existingTask, err := n.Store.GetTask(req.TaskId)
	if err == nil {
		logger.Log.Warn("Task already exists", zap.String("task_id", req.TaskId), zap.String("status", string(existingTask.Status)))

		// Case 1: Task completed -> Return cached result immediately (Cache)
		if existingTask.Status == storage.StatusCompleted {
			logger.Log.Info("Returning cached result", zap.String("task_id", req.TaskId))

			resp := &pb.TaskResponse{
				TaskId:     req.TaskId,
				WorkerId:   n.Host.ID().String(),
				OutputData: []byte(existingTask.Result), // Return cached result
			}
			data, err := proto.Marshal(resp)
			if err != nil {
				logger.Log.Error("Failed to marshal response", zap.Error(err))
				return
			}
			_, err = s.Write(data)
			if err != nil {
				logger.Log.Error("Failed to write response", zap.Error(err))
				return
			}
			return // Stop processing, don't run Wasm again
		}

		// Case 2: Task running -> Ignore this request (Debounce)
		if existingTask.Status == storage.StatusRunning {
			logger.Log.Info("Task is already running", zap.String("task_id", req.TaskId))
			return
		}
	}

	// Save status PENDING to DB (if not already exists)
	taskRecord := storage.TaskRecord{
		ID:        req.TaskId,
		Source:    senderID,
		Status:    storage.StatusRunning,
		Timestamp: time.Now().Unix(),
	}
	if err := n.Store.SaveTask(taskRecord); err != nil {
		logger.Log.Error("Failed to save task status", zap.Error(err))
		return
	}

	// Run Runtime
	ctx := context.Background()
	rt, err := runtime.NewWasmRuntime(ctx)
	if err != nil {
		logger.Log.Error("Runtime init error", zap.Error(err))
		return
	}
	defer rt.Close(ctx)

	output, err := rt.Run(ctx, req.WasmBinary, req.InputData)
	// MALICIOUS LOGIC
	if n.IsMalicious {
		logger.Log.Info("I am a Malicious Node! Altering result...")
		output = []byte("I HACKED THIS RESULT HEHE")
	}

	resp := &pb.TaskResponse{
		TaskId:   req.TaskId,
		WorkerId: n.Host.ID().String(),
	}

	// Update result
	if err != nil {
		// If we get here, it means there's a real error (Runtime crash, Out of memory...)
		taskRecord.Status = storage.StatusFailed
		taskRecord.Result = err.Error()
		resp.Error = err.Error()
		logger.Log.Error("Task Failed", zap.Error(err))
	} else {
		// If we get here, it means Success (including Exit Code 0)
		taskRecord.Status = storage.StatusCompleted

		// Convert output byte to string for pretty logging
		resultStr := string(output)
		taskRecord.Result = resultStr

		resp.OutputData = output // Send original data back
		logger.Log.Info("Task Completed", zap.String("result", resultStr))

		event := fmt.Sprintf(`{"type": "TASK_COMPLETED", "task_id": "%s", "worker": "%s", "result": "%s"}`,
			req.TaskId, n.Host.ID().ShortString(), resultStr)

		n.Hub.BroadcastEvent(event)
	}

	n.Store.SaveTask(taskRecord)

	// Send Response back to Sender
	respBytes, err := proto.Marshal(resp)
	if err != nil {
		logger.Log.Error("Marshal response failed", zap.Error(err))
		return
	}

	// Write to stream
	_, err = s.Write(respBytes)
	if err != nil {
		logger.Log.Error("Failed to send response back", zap.Error(err))
	} else {
		logger.Log.Info("Sent result back to", zap.String("peer", s.Conn().RemotePeer().ShortString()))
	}
}

func (n *Node) StartDiscovery() error {
	s := mdns.NewMdnsService(n.Host, "edge-grid-service", &discoveryNotifee{h: n.Host})
	return s.Start()
}

func (n *Node) SendTask(ctx context.Context, peerID peer.ID, wasmBytes []byte, taskID string, input []byte) ([]byte, error) {
	// Open stream with timeout (example 3s)
	ctxConnect, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	s, err := n.Host.NewStream(ctxConnect, peerID, ProtocolID)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer s.Close()

	// Signing Logic
	// 1. Get data to sign (Wasm + Input)
	dataToSign := append(wasmBytes, input...)

	// 2. Sign with Private Key
	signature, err := n.PrivKey.Sign(dataToSign)
	if err != nil {
		return nil, fmt.Errorf("signing failed: %w", err)
	}

	// 3. Get Public Key in bytes to send
	pubKeyBytes, err := crypto.MarshalPublicKey(n.PrivKey.GetPublic())
	if err != nil {
		return nil, fmt.Errorf("marshal pubkey failed: %w", err)
	}

	req := &pb.TaskRequest{
		TaskId:     taskID,
		WasmBinary: wasmBytes,
		InputData:  input,
		Signature:  signature,
		PublicKey:  pubKeyBytes,
	}
	data, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}

	// Set deadline for writing (avoid hanging if network lag)
	s.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err = s.Write(data); err != nil {
		return nil, fmt.Errorf("write failed: %w", err)
	}
	s.CloseWrite() // Notify done sending

	// 3. Read Response with Deadline (example wait for max 10s)
	s.SetReadDeadline(time.Now().Add(10 * time.Second))
	respData, err := io.ReadAll(s)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	resp := &pb.TaskResponse{}
	if err := proto.Unmarshal(respData, resp); err != nil {
		return nil, fmt.Errorf("invalid response proto: %w", err)
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("worker error: %s", resp.Error)
	}

	return resp.OutputData, nil
}

// Join global network (Global DHT)
func (n *Node) JoinGlobalNetwork(ctx context.Context) error {
	// 1. Initialize DHT (Distributed Hash Table)
	// ModeAuto: Both Client and Server if network is healthy
	kademliaDHT, err := dht.New(ctx, n.Host, dht.Mode(dht.ModeAuto))
	if err != nil {
		return err
	}

	if err = kademliaDHT.Bootstrap(ctx); err != nil {
		return err
	}

	// 2. Connect to the public bootstrap peers (IPFS/Libp2p)
	// These are the super stable nodes operated by Protocol Labs
	bootstrapPeers := dht.DefaultBootstrapPeers

	var wg sync.WaitGroup
	for _, peerAddr := range bootstrapPeers {
		peerinfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := n.Host.Connect(ctx, *peerinfo); err != nil {
				// If we can't connect to all, it's not a problem
			} else {
				logger.Log.Info("Connected to bootstrap node", zap.String("peer", peerinfo.ID.ShortString()))
			}
		}()
	}

	// No need to wait for all, just run in the background
	return nil
}

func (n *Node) UpdateReputation(pid peer.ID, delta int) {
	val, ok := n.PeerStats.Load(pid)
	if !ok {
		// If not exists, initialize default 100
		val = PeerMetric{Reputation: 100, LastSeen: time.Now().Unix()}
	}

	metric := val.(PeerMetric)
	metric.Reputation += delta

	// Limit points (Cap)
	if metric.Reputation > 100 {
		metric.Reputation = 100
	}

	logger.Log.Info("Reputation update", zap.String("peer", pid.ShortString()), zap.Int("reputation", metric.Reputation), zap.Int("delta", delta))

	n.PeerStats.Store(pid, metric)
}

func (n *Node) IsBanned(pid peer.ID) bool {
	val, ok := n.PeerStats.Load(pid)
	if !ok {
		return false
	} // Default is not banned
	return val.(PeerMetric).Reputation <= 0
}

type discoveryNotifee struct {
	h host.Host
}

func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	if pi.ID == n.h.ID() {
		return
	}
	logger.Log.Info("Found peer", zap.String("peer", pi.ID.ShortString())) // Print peer ID

	if err := n.h.Connect(context.Background(), pi); err != nil {
		logger.Log.Error("Connection failed", zap.Error(err))
	} else {
		logger.Log.Info("Connected to", zap.String("peer", pi.ID.ShortString()))
	}
}
