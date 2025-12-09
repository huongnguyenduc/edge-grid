package p2p

import (
	"context"
	pb "edge-grid/api/proto"
	"edge-grid/internal/api"
	"edge-grid/internal/runtime"
	"edge-grid/internal/storage"
	"fmt"
	"io"
	"log"
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
	"google.golang.org/protobuf/proto"
)

const ProtocolID = "/edge-grid/1.0.0"

type PeerMetric struct {
	CPUUsage float64
	RamUsage uint64
	LastSeen int64
}

type Node struct {
	Host      host.Host
	Store     *storage.Store
	Hub       *api.Hub
	PrivKey   crypto.PrivKey
	PeerStats sync.Map // Map[peer.ID]PeerMetric
}

func NewNode(listenPort int, dbPath string, hub *api.Hub) (*Node, error) {
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
		Host:    h,
		Store:   store,
		Hub:     hub,
		PrivKey: priv,
	}

	h.SetStreamHandler(ProtocolID, node.handleStream)

	sub, err := h.EventBus().Subscribe(new(event.EvtLocalReachabilityChanged))
	if err != nil {
		log.Printf("Failed to subscribe to reachability events: %v", err)
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
				log.Printf("📶 Network Status Changed: %s", status)
			}
		}()
	}

	return node, nil
}

// handleStream: Receive Task -> Run -> Return Result
func (n *Node) handleStream(s network.Stream) {
	defer s.Close() // Close stream when done

	// 1. Read Request
	data, err := io.ReadAll(s)
	if err != nil {
		return
	}

	req := &pb.TaskRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return
	}

	// --- VERIFICATION LOGIC ---
	log.Printf("🔒 Verifying signature for task %s...", req.TaskId)

	// 1. Parse Public Key from bytes
	senderPubKey, err := crypto.UnmarshalPublicKey(req.PublicKey)
	if err != nil {
		log.Printf("❌ Auth Failed: Invalid Public Key")
		return
	}

	// 2. Reconstruct original data (Wasm + Input)
	dataToVerify := append(req.WasmBinary, req.InputData...)

	// 3. Verify Signature
	valid, err := senderPubKey.Verify(dataToVerify, req.Signature)
	if err != nil || !valid {
		log.Printf("❌ SECURITY ALERT: Invalid Signature! Task rejected.")
		// Send error to Sender
		resp := &pb.TaskResponse{
			TaskId:   req.TaskId,
			WorkerId: n.Host.ID().String(),
			Error:    "Invalid Signature",
		}
		respBytes, _ := proto.Marshal(resp)
		s.Write(respBytes)
		return
	}

	log.Printf("🔓 Signature Valid. Sender is authenticated.")
	// --------------------------

	senderID := s.Conn().RemotePeer().String()
	log.Printf("📥 Processing Task %s from %s", req.TaskId, senderID[:10])

	// Check if task already exists
	existingTask, err := n.Store.GetTask(req.TaskId)
	if err == nil {
		log.Printf("⚠️ Task %s already exists (Status: %s). Idempotency check triggered.", req.TaskId, existingTask.Status)

		// Case 1: Task completed -> Return cached result immediately (Cache)
		if existingTask.Status == storage.StatusCompleted {
			log.Printf("⏩ Returning cached result for %s", req.TaskId)

			resp := &pb.TaskResponse{
				TaskId:     req.TaskId,
				WorkerId:   n.Host.ID().String(),
				OutputData: []byte(existingTask.Result), // Return cached result
			}
			data, _ := proto.Marshal(resp)
			s.Write(data)
			return // Stop processing, don't run Wasm again
		}

		// Case 2: Task running -> Ignore this request (Debounce)
		if existingTask.Status == storage.StatusRunning {
			log.Printf("⏳ Task %s is already running. Ignoring duplicate request.", req.TaskId)
			return
		}
	}

	// 2. Save status PENDING to DB (if not already exists)
	taskRecord := storage.TaskRecord{
		ID:        req.TaskId,
		Source:    senderID,
		Status:    storage.StatusRunning,
		Timestamp: time.Now().Unix(),
	}
	n.Store.SaveTask(taskRecord) // Save first time

	// 3. Run Runtime
	ctx := context.Background()
	rt, err := runtime.NewWasmRuntime(ctx)
	if err != nil {
		log.Printf("Runtime init error: %v", err)
		return
	}
	defer rt.Close(ctx)

	output, err := rt.Run(ctx, req.WasmBinary, req.InputData)

	resp := &pb.TaskResponse{
		TaskId:   req.TaskId,
		WorkerId: n.Host.ID().String(),
	}

	// 4. Update result
	if err != nil {
		// If we get here, it means there's a real error (Runtime crash, Out of memory...)
		taskRecord.Status = storage.StatusFailed
		taskRecord.Result = err.Error()
		resp.Error = err.Error()
		log.Printf("❌ Task Failed: %v", err)
	} else {
		// If we get here, it means Success (including Exit Code 0)
		taskRecord.Status = storage.StatusCompleted

		// Convert output byte to string for pretty logging
		resultStr := string(output)
		taskRecord.Result = resultStr

		resp.OutputData = output // Send original data back
		log.Printf("✅ Task Completed. Result: %s", resultStr)

		event := fmt.Sprintf(`{"type": "TASK_COMPLETED", "task_id": "%s", "worker": "%s", "result": "%s"}`,
			req.TaskId, n.Host.ID().ShortString(), resultStr)

		n.Hub.BroadcastEvent(event)
	}

	n.Store.SaveTask(taskRecord)

	// 5. Send Response back to Sender
	respBytes, err := proto.Marshal(resp)
	if err != nil {
		log.Printf("Marshal response failed: %v", err)
		return
	}

	// Write to stream
	_, err = s.Write(respBytes)
	if err != nil {
		log.Printf("Failed to send response back: %v", err)
	} else {
		log.Printf("📤 Sent result back to %s", s.Conn().RemotePeer().ShortString())
	}
}

func (n *Node) StartDiscovery() error {
	s := mdns.NewMdnsService(n.Host, "edge-grid-service", &discoveryNotifee{h: n.Host})
	return s.Start()
}

func (n *Node) SendTask(ctx context.Context, peerID peer.ID, wasmBytes []byte, taskID string, input []byte) ([]byte, error) {
	// 1. Open stream with timeout (example 3s)
	ctxConnect, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	s, err := n.Host.NewStream(ctxConnect, peerID, ProtocolID)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer s.Close()

	// --- SIGNING LOGIC ---
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
	// --------------------------

	req := &pb.TaskRequest{
		TaskId:     taskID,
		WasmBinary: wasmBytes,
		InputData:  input,
		Signature:  signature,
		PublicKey:  pubKeyBytes,
	}
	data, _ := proto.Marshal(req)

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
				log.Printf("🌍 Connected to bootstrap node: %s", peerinfo.ID.ShortString())
			}
		}()
	}

	// No need to wait for all, just run in the background
	return nil
}

type discoveryNotifee struct {
	h host.Host
}

func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	if pi.ID == n.h.ID() {
		return
	}
	log.Printf("Found peer: %s", pi.ID.ShortString()) // Print peer ID

	if err := n.h.Connect(context.Background(), pi); err != nil {
		log.Printf("Connection failed: %v", err)
	} else {
		log.Printf("Connected to %s via QUIC!", pi.ID.ShortString())
	}
}
