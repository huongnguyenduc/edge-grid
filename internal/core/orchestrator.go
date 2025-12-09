package core

import (
	"context"
	"edge-grid/internal/p2p"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

type Orchestrator struct {
	Node *p2p.Node
}

func NewOrchestrator(node *p2p.Node) *Orchestrator {
	return &Orchestrator{Node: node}
}

type rankedPeer struct {
	ID    peer.ID
	Score float64 // The lower the better
}

// Best worker selection algorithm
func (o *Orchestrator) selectBestWorkers() []peer.ID {
	peers := o.Node.Host.Network().Peers()
	var ranked []rankedPeer

	for _, p := range peers {
		score := 100.0 // Default worst score (100% CPU)

		// Get metrics from map
		if val, ok := o.Node.PeerStats.Load(p); ok {
			stats := val.(p2p.PeerMetric)

			// Check if metric is too old (if over 10s, consider it lost)
			if time.Now().Unix()-stats.LastSeen < 10 {
				// Score formula: CPU
				// (Can add Latency or RAM here)
				score = stats.CPUUsage
			}
		}

		ranked = append(ranked, rankedPeer{ID: p, Score: score})
	}

	// Sort: Low score (CPU free) first
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Score < ranked[j].Score
	})

	// Return sorted list
	result := make([]peer.ID, len(ranked))
	for i, rp := range ranked {
		result[i] = rp.ID
	}
	return result
}

func (o *Orchestrator) DeployTaskWithRetry(wasmBytes []byte, input []byte) ([]byte, error) {
	// 1. Get list of Worker sorted nicely
	sortedPeers := o.selectBestWorkers()

	if len(sortedPeers) == 0 {
		return nil, fmt.Errorf("no workers found in swarm")
	}

	taskID := fmt.Sprintf("task-%d", time.Now().UnixNano())
	maxRetries := 3

	// 2. Try sending in order of priority (Best -> Worst)
	for i, p := range sortedPeers {
		if i >= maxRetries {
			break
		}

		// Get score to log for fun
		score := 100.0
		if val, ok := o.Node.PeerStats.Load(p); ok {
			score = val.(p2p.PeerMetric).CPUUsage
		}

		log.Printf("🤖 [Scheduler] Selecting Worker %s (CPU: %.1f%%)...", p.ShortString(), score)

		result, err := o.Node.SendTask(context.Background(), p, wasmBytes, taskID, input)

		if err == nil {
			return result, nil
		}

		log.Printf("⚠️ Node %s failed/busy. Failing over to next best node...", p.ShortString())
	}

	return nil, fmt.Errorf("all attempts failed")
}
