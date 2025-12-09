package main

import (
	"bufio"
	"context"
	"edge-grid/internal/api"
	"edge-grid/internal/p2p"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	port := flag.Int("port", 0, "UDP Port to listen on")
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
	node, err := p2p.NewNode(*port, dbPath, hub)
	if err != nil {
		panic(err)
	}
	defer node.Store.Close() // Remember to close DB when app is stopped

	fmt.Printf("Edge-Grid Node running on port %d\n", *port)
	fmt.Printf("Database stored at: %s\n", dbPath)

	if err := node.StartDiscovery(); err != nil {
		panic(err)
	}

	fmt.Println("💓 Starting Heartbeat System (GossipSub)...")
	if err := node.SetupHeartbeat(context.Background()); err != nil {
		panic(err)
	}

	// 3. Run HTTP Server for WebSocket (run on a different port from P2P port)
	// Example: P2P port 4001 -> WS port 8001
	wsPort := *port + 4000
	go func() {
		http.HandleFunc("/ws", hub.ServeWs)
		log.Printf("🔥 Dashboard WebSocket running at: ws://localhost:%d/ws", wsPort)
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", wsPort), nil))
	}()

	// CLI Loop
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		fmt.Println("Commands: [enter] to deploy, 'list' to show history")

		for scanner.Scan() {
			cmd := scanner.Text()

			// Lệnh xem lịch sử
			if strings.TrimSpace(cmd) == "list" {
				tasks, err := node.Store.ListTasks()
				if err != nil {
					fmt.Printf("Error listing tasks: %v\n", err)
					continue
				}
				fmt.Println("\n--- Task History ---")
				for _, t := range tasks {
					ts := time.Unix(t.Timestamp, 0).Format("15:04:05")
					fmt.Printf("[%s] ID: %s | Status: %s | Result: %s\n", ts, t.ID, t.Status, t.Result)
				}
				fmt.Println("--------------------")
				continue
			}

			// Lệnh deploy mặc định (Enter)
			inputData := cmd // Text người dùng gõ, ví dụ: "Hello Edge Grid"
			if inputData == "" {
				inputData = "Default Input"
			}

			wasmBytes, err := os.ReadFile("hello.wasm")
			if err != nil {
				fmt.Println("Error reading hello.wasm:", err)
				continue
			}

			peers := node.Host.Network().Peers()
			if len(peers) == 0 {
				fmt.Println("Waiting for peers...")
				continue
			}

			for _, p := range peers {
				fmt.Printf("Deploying task with input '%s' to %s...\n", inputData, p.ShortString())
				taskID := fmt.Sprintf("task-%d", time.Now().UnixNano())

				// Sửa hàm SendTask để nhận thêm InputData
				node.SendTask(context.Background(), p, wasmBytes, taskID, []byte(inputData))
			}
		}
	}()

	select {}
}
