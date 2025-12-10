package main

import (
	"bufio"
	"context"
	"edge-grid/internal/api"
	"edge-grid/internal/core"
	"edge-grid/internal/p2p"
	"edge-grid/pkg/logger"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
)

func main() {
	logger.InitLogger(false)
	defer logger.Log.Sync()
	port := flag.Int("port", 0, "UDP Port to listen on")
	malicious := flag.Bool("malicious", false, "Be a bad node (returns wrong results)")
	flag.Parse()

	if *port == 0 {
		fmt.Println("Please provide --port (e.g., 4001)")
		return
	}

	hub := api.NewHub()
	go hub.Run()

	// Create a separate data directory for each port to avoid DB conflict
	dbPath := fmt.Sprintf("./data/node-%d", *port)
	os.MkdirAll(dbPath, 0755)

	// Initialize Node with DB Path
	node, err := p2p.NewNode(*port, dbPath, hub, *malicious)
	if err != nil {
		panic(err)
	}
	defer node.Store.Close() // Remember to close DB when app is stopped

	logger.Log.Info("Edge-Grid Node running on port", zap.Int("port", *port))
	logger.Log.Info("Database stored at", zap.String("path", dbPath))

	// Connect to the Internet (DHT)
	// Note: This may take 5-10s to find peers
	go func() {
		logger.Log.Info("Joining Global DHT Network")
		if err := node.JoinGlobalNetwork(context.Background()); err != nil {
			logger.Log.Error("DHT Error", zap.Error(err))
		}
	}()

	if err := node.StartDiscovery(); err != nil {
		panic(err)
	}

	logger.Log.Info("Starting Heartbeat System (GossipSub)")
	if err := node.SetupHeartbeat(context.Background()); err != nil {
		panic(err)
	}

	// 3. Run HTTP Server for WebSocket (run on a different port from P2P port)
	// Example: P2P port 4001 -> WS port 8001
	wsPort := *port + 4000
	go func() {
		http.HandleFunc("/ws", hub.ServeWs)
		logger.Log.Info("Dashboard WebSocket running at", zap.Int("port", wsPort))
		err := http.ListenAndServe(fmt.Sprintf(":%d", wsPort), nil)
		if err != nil {
			logger.Log.Error("Failed to start HTTP server", zap.Error(err))
		}
	}()

	orch := core.NewOrchestrator(node)

	// CLI Loop
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		logger.Log.Info("Commands: [enter] to deploy, 'list' to show history")

		for scanner.Scan() {
			cmd := scanner.Text()

			// Command to show history
			if strings.TrimSpace(cmd) == "list" {
				tasks, err := node.Store.ListTasks()
				if err != nil {
					logger.Log.Error("Error listing tasks", zap.Error(err))
					continue
				}
				logger.Log.Info("Task History")
				for _, t := range tasks {
					ts := time.Unix(t.Timestamp, 0).Format("15:04:05")
					logger.Log.Info("Task", zap.String("timestamp", ts), zap.String("id", t.ID), zap.String("status", string(t.Status)), zap.String("result", t.Result))
				}
				logger.Log.Info("--------------------")
				continue
			}

			// Default deploy command (Enter)
			inputData := cmd // Text user typed, example: "Hello Edge Grid"
			if inputData == "" {
				inputData = "Default Input"
			}

			wasmBytes, err := os.ReadFile("hello.wasm")
			if err != nil {
				logger.Log.Error("Error reading hello.wasm", zap.Error(err))
				continue
			}

			logger.Log.Info("Deploying task input", zap.String("input", inputData))

			if strings.HasPrefix(inputData, "consensus ") {
				realInput := strings.TrimPrefix(inputData, "consensus ")

				// Request 2 nodes to run (Demo)
				// In practice, we need at least 3 nodes to remove 1 bad node
				result, err := orch.DeployTaskWithConsensus(wasmBytes, []byte(realInput), 3)

				if err != nil {
					logger.Log.Error("Consensus Failed", zap.Error(err))
				} else {
					logger.Log.Info("Trusted Result", zap.String("result", string(result)))
				}
				continue
			}

			result, err := orch.DeployTaskWithRetry(wasmBytes, []byte(inputData))
			if err != nil {
				logger.Log.Error("Deployment Failed", zap.Error(err))
			} else {
				logger.Log.Info("Final Result", zap.String("result", string(result)))
			}
		}
	}()

	select {}
}
