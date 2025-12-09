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

	listenAddr := fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic-v1", listenPort)

	h, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrStrings(listenAddr),
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

// SendTask send Task and wait for result
func (n *Node) SendTask(ctx context.Context, peerID peer.ID, wasmBytes []byte, taskID string, inputData []byte) {
	// 1. Open Stream
	s, err := n.Host.NewStream(ctx, peerID, ProtocolID)
	if err != nil {
		log.Printf("Stream open failed: %v", err)
		return
	}
	defer s.Close() // Close stream when function ends

	// 2. Create Request
	req := &pb.TaskRequest{
		TaskId:     taskID,
		WasmBinary: wasmBytes,
		InputData:  inputData,
	}

	data, _ := proto.Marshal(req)

	// 3. Send Request
	_, err = s.Write(data)
	if err != nil {
		log.Printf("Write failed: %v", err)
		return
	}

	// IMPORTANT: Notify the other side that "I have sent the request content"
	// So that the other side can io.ReadAll() and start processing.
	// But don't close the stream yet, because we still need to read the response.
	s.CloseWrite()

	log.Printf("b Sent task %s to %s. Waiting for result...", taskID, peerID.ShortString())

	// 4. Read Response
	respData, err := io.ReadAll(s)
	if err != nil {
		log.Printf("Read response failed: %v", err)
		return
	}

	resp := &pb.TaskResponse{}
	if err := proto.Unmarshal(respData, resp); err != nil {
		log.Printf("Invalid response: %v", err)
		return
	}

	// 5. Print result
	if resp.Error != "" {
		log.Printf("❌ Remote Error from %s: %s", peerID.ShortString(), resp.Error)
	} else {
		log.Printf("✅ Result from %s: %s", peerID.ShortString(), string(resp.OutputData))
	}
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
