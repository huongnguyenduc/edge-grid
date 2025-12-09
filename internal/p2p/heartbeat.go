package p2p

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

const HeartbeatTopic = "edge-grid-heartbeat"

// Heartbeat message contains the health information of the Node
type Heartbeat struct {
	NodeID    string  `json:"node_id"`
	CPUUsage  float64 `json:"cpu_usage"` // %
	RamUsage  uint64  `json:"ram_usage"` // MB
	Timestamp int64   `json:"timestamp"`
}

// SetupHeartbeat: Initialize GossipSub and start the heartbeat loop
func (n *Node) SetupHeartbeat(ctx context.Context) error {
	// 1. Tạo PubSub (GossipSub router)
	ps, err := pubsub.NewGossipSub(ctx, n.Host)
	if err != nil {
		return err
	}

	// 2. Join the "edge-grid-heartbeat" topic
	topic, err := ps.Join(HeartbeatTopic)
	if err != nil {
		return err
	}

	// 3. Subscribe to listen for heartbeats from other nodes
	sub, err := topic.Subscribe()
	if err != nil {
		return err
	}

	// Goroutine 1: Listen for heartbeats
	go n.listenHeartbeats(ctx, sub)

	// Goroutine 2: Broadcast heartbeats - Only Worker nodes need to send
	// For dynamic demo, we let all nodes send
	go n.broadcastHeartbeat(ctx, topic)

	return nil
}

func (n *Node) broadcastHeartbeat(ctx context.Context, topic *pubsub.Topic) {
	ticker := time.NewTicker(2 * time.Second) // Heartbeat every 2 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Get real metrics
			v, _ := mem.VirtualMemory()
			c, _ := cpu.Percent(0, false)
			cpuPercent := 0.0
			if len(c) > 0 {
				cpuPercent = c[0]
			}

			hb := Heartbeat{
				NodeID:    n.Host.ID().ShortString(),
				CPUUsage:  cpuPercent,
				RamUsage:  v.Used / 1024 / 1024, // Convert to megabytes
				Timestamp: time.Now().Unix(),
			}

			// Serialize to JSON
			data, _ := json.Marshal(hb)

			// Publish to P2P network
			if err := topic.Publish(ctx, data); err != nil {
				log.Printf("Heartbeat error: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (n *Node) listenHeartbeats(ctx context.Context, sub *pubsub.Subscription) {
	for {
		msg, err := sub.Next(ctx)
		if err != nil {
			return
		}

		// Skip messages from yourself
		if msg.ReceivedFrom == n.Host.ID() {
			continue
		}

		var hb Heartbeat
		if err := json.Unmarshal(msg.Data, &hb); err != nil {
			continue
		}

		// Push event to Dashboard through WebSocket
		// Wrap it in JSON with type="HEARTBEAT"
		eventJSON := fmt.Sprintf(`{"type": "HEARTBEAT", "node_id": "%s", "cpu": %.2f, "ram": %d}`,
			hb.NodeID, hb.CPUUsage, hb.RamUsage)

		if n.Hub != nil {
			n.Hub.BroadcastEvent(eventJSON)
		}
	}
}
