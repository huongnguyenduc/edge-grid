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
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"google.golang.org/protobuf/proto"
)

const ProtocolID = "/edge-grid/1.0.0"

type Node struct {
	Host  host.Host
	Store *storage.Store
	Hub   *api.Hub
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
		// Enable NAT service and auto NAT v2 (Hole Punching)
		libp2p.EnableNATService(),
		libp2p.EnableAutoNATv2(),
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
		Host:  h,
		Store: store,
		Hub:   hub,
	}

	h.SetStreamHandler(ProtocolID, node.handleStream)

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

	senderID := s.Conn().RemotePeer().String()
	log.Printf("📥 Processing Task %s from %s", req.TaskId, senderID[:10])

	// 2. Save status PENDING to DB
	taskRecord := storage.TaskRecord{
		ID:        req.TaskId,
		Source:    senderID,
		Status:    storage.StatusRunning,
		Timestamp: time.Now().Unix(),
	}
	n.Store.SaveTask(taskRecord) // Save first time

	// 2. Run Runtime
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

	// 3. Update result
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

	// 4. Send Response back to Sender
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

	// 2. Send Request
	req := &pb.TaskRequest{
		TaskId:     taskID,
		WasmBinary: wasmBytes,
		InputData:  input,
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
