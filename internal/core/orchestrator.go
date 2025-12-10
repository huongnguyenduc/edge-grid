package core

import (
	"context"
	"crypto/sha256"
	"edge-grid/internal/p2p"
	"encoding/hex"
	"fmt"
	"log"
	"math/rand"
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

func (o *Orchestrator) getVerifiedWorkers() []peer.ID {
	allPeers := o.Node.Host.Network().Peers()
	var verified []peer.ID

	for _, p := range allPeers {
		if _, ok := o.Node.PeerStats.Load(p); !ok {
			continue
		}
		if o.Node.IsBanned(p) {
			continue
		}
		verified = append(verified, p)
	}
	return verified
}

type weightedCandidate struct {
	ID     peer.ID
	Weight int
}

// selectWeightedCommittee: Select size nodes based on reputation
func (o *Orchestrator) selectWeightedCommittee(size int) []peer.ID {
	workers := o.getVerifiedWorkers() // Filtered nodes banned (< 0 points)

	// If the number of workers is less than the requested size, get all
	if len(workers) <= size {
		return workers
	}

	var candidates []weightedCandidate
	totalWeight := 0

	// 1. Build the candidate list and total weight
	for _, p := range workers {
		score := 100 // Default
		if val, ok := o.Node.PeerStats.Load(p); ok {
			metric := val.(p2p.PeerMetric)
			score = metric.Reputation
		}

		// Only get nodes with positive points (Safety check)
		if score > 0 {
			candidates = append(candidates, weightedCandidate{ID: p, Weight: score})
			totalWeight += score
		}
	}

	if totalWeight == 0 || len(candidates) == 0 {
		return nil // No one is qualified
	}

	// 2. Roulette Wheel algorithm to select 'size' different nodes
	selectedSet := make(map[peer.ID]bool)
	var finalSelection []peer.ID

	for len(finalSelection) < size {
		// Random number from 0 -> totalWeight
		r := rand.Intn(totalWeight)

		currentSum := 0
		for _, c := range candidates {
			// If node is already selected, skip to avoid duplicates in the committee
			if selectedSet[c.ID] {
				continue
			}

			currentSum += c.Weight
			if r < currentSum {
				// WINNER!
				selectedSet[c.ID] = true
				finalSelection = append(finalSelection, c.ID)

				// Optimization trick:
				// When we select a node, in theory we should subtract totalWeight and remove candidate.
				// But for MVP simplicity, we just loop again (low probability of duplicates).
				break
			}
		}

		// Safety break: If we loop too long and don't find a new node (due to random chance)
		// then exit to avoid infinite loop.
		if len(finalSelection) < size && len(finalSelection) == len(candidates) {
			break
		}
	}

	return finalSelection
}

// Result of a node's execution
type ExecutionResult struct {
	WorkerID peer.ID
	Data     []byte
	Error    error
}

// DeployTaskWithConsensus: Send task to N nodes and compare results
func (o *Orchestrator) DeployTaskWithConsensus(wasmBytes []byte, input []byte, committeeSize int) ([]byte, error) {
	selectedWorkers := o.selectWeightedCommittee(committeeSize)

	// Minimum 3 nodes are required to run Consensus (to remove 1 malicious node)
	if len(selectedWorkers) < 3 {
		return nil, fmt.Errorf("network too small for consensus (need at least 3 active workers)")
	}

	quorumSize := len(selectedWorkers) // Number of nodes participating in this round
	taskID := fmt.Sprintf("task-%d", time.Now().UnixNano())
	log.Printf("⚖️  [Consensus] Dispatching task %s to %d workers...", taskID, quorumSize)

	resultsChan := make(chan ExecutionResult, quorumSize)

	// 1. Send tasks in parallel
	for _, p := range selectedWorkers {
		go func(worker peer.ID) {
			// Note: Do not use wg.Wait() to block the main thread anymore
			res, err := o.Node.SendTask(context.Background(), worker, wasmBytes, taskID, input)
			resultsChan <- ExecutionResult{WorkerID: worker, Data: res, Error: err}
		}(p)
	}

	// Map to track who voted for what
	// Key: PeerID, Value: HashString
	peerVotes := make(map[peer.ID]string)

	voteMap := make(map[string]int)
	dataMap := make(map[string][]byte)

	neededVotes := (quorumSize / 2) + 1
	var winnerHash string
	var winnerData []byte

	// Loop to collect results
	for i := 0; i < quorumSize; i++ {
		res := <-resultsChan

		if res.Error != nil {
			// Node error/timeout -> Subtract a little bit of points (-1) because not stable
			o.Node.UpdateReputation(res.WorkerID, -1)
			continue
		}

		hash := sha256.Sum256(res.Data)
		hashStr := hex.EncodeToString(hash[:])

		// Save the vote: Node has voted for this Hash
		peerVotes[res.WorkerID] = hashStr

		voteMap[hashStr]++
		dataMap[hashStr] = res.Data

		// If we find the winner
		if voteMap[hashStr] >= neededVotes {
			winnerHash = hashStr
			winnerData = dataMap[hashStr]
			// Do not return immediately, but break to down the punishment
			// (If you want to fast, run the punishment in the background goroutine)
			break
		}
	}

	if winnerHash == "" {
		return nil, fmt.Errorf("consensus failed")
	}

	// --- PUNISHMENT LOGIC ---
	// Run in the background to not block the result return to user
	go func() {
		for pid, votedHash := range peerVotes {
			if votedHash == winnerHash {
				// Reward: Good Node
				o.Node.UpdateReputation(pid, 1)
			} else {
				// Heavy Punishment: Node lying (Malicious)
				log.Printf("⚔️ SLASHING Malicious Node %s. Voted wrong hash!", pid.ShortString())
				o.Node.UpdateReputation(pid, -20) // Subtract 20 points one time
			}
		}
	}()
	// ------------------------------------

	log.Printf("✅ Consensus Reached! Reward/Slash applied.")
	return winnerData, nil
}
