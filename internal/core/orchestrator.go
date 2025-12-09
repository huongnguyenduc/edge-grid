package core

import (
	"context"
	"edge-grid/internal/p2p"
	"fmt"
	"log"
	"time"
)

type Orchestrator struct {
	Node *p2p.Node
}

func NewOrchestrator(node *p2p.Node) *Orchestrator {
	return &Orchestrator{Node: node}
}

// DeployTaskWithRetry: Try sending task to multiple nodes until success
func (o *Orchestrator) DeployTaskWithRetry(wasmBytes []byte, input []byte) ([]byte, error) {
	// Get list of peers
	peers := o.Node.Host.Network().Peers()
	if len(peers) == 0 {
		return nil, fmt.Errorf("no workers available")
	}

	taskID := fmt.Sprintf("task-%d", time.Now().UnixNano())
	maxRetries := 3

	// Loop through peers (Round-robin or Random)
	// In practice, it will choose based on the lowest CPU (from Heartbeat)
	for i, p := range peers {
		if i >= maxRetries {
			break
		}

		log.Printf("🔄 Attempt %d/%d: Deploying to %s...", i+1, maxRetries, p.ShortString())

		// Call upgraded SendTask function
		result, err := o.Node.SendTask(context.Background(), p, wasmBytes, taskID, input)

		if err == nil {
			log.Printf("✅ Success on attempt %d!", i+1)
			return result, nil
		}

		log.Printf("⚠️ Failed on %s: %v. Retrying next node...", p.ShortString(), err)
	}

	return nil, fmt.Errorf("all attempts failed")
}
