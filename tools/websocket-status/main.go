package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/coder/websocket"
)

// EventMessage represents a message from the server
type EventMessage struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data,omitempty"`
	ConnID string `json:"connId,omitempty"`
}

func main() {
	server := flag.String("server", "ws://localhost:8081", "websocket server URL")
	timeout := flag.Duration("timeout", 30*time.Second, "connection timeout")
	verbose := flag.Bool("v", false, "verbose output")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// Connect to websocket
	fmt.Printf("Connecting to %s...\n", *server)
	conn, _, err := websocket.Dial(ctx, *server+"/subscribe", nil)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	fmt.Println("Connected!")

	// Handle interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	// Create a context that cancels on interrupt
	interruptCtx, interruptCancel := context.WithCancel(context.Background())
	go func() {
		<-sigChan
		fmt.Println("\nDisconnecting...")
		interruptCancel()
	}()

	eventCount := 0
	startTime := time.Now()

	// Read messages
	for {
		select {
		case <-interruptCtx.Done():
			fmt.Printf("\nReceived %d events in %v\n", eventCount, time.Since(startTime))
			return
		default:
		}

		// Read with no timeout - just wait for messages
		messageType, data, err := conn.Read(interruptCtx)
		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				fmt.Println("Server closed connection")
				return
			}
			// Other error
			select {
			case <-interruptCtx.Done():
				return
			default:
				fmt.Printf("Connection error: %v\n", err)
				return
			}
		}

		if messageType != websocket.MessageText {
			continue
		}

		var msg EventMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			if *verbose {
				fmt.Printf("Failed to parse message: %v\n", err)
			}
			continue
		}

		// Skip pong messages from output
		if msg.Type == "pong" {
			continue
		}

		eventCount++

		// Display message
		if msg.Type == "connected" {
			fmt.Printf("[CONNECTED] Connection ID: %s\n", msg.ConnID)
		} else {
			fmt.Printf("[%s] ", msg.Type)
			if itemID, ok := msg.Data["itemId"]; ok {
				fmt.Printf("ItemID=%v ", itemID)
			}
			if author, ok := msg.Data["author"]; ok {
				fmt.Printf("Author=%v ", author)
			}
			if liked, ok := msg.Data["liked"]; ok {
				fmt.Printf("Liked=%v ", liked)
			}
			if ctLikes, ok := msg.Data["ctLikes"]; ok {
				fmt.Printf("Likes=%v ", ctLikes)
			}
			fmt.Println()

			if *verbose {
				bytes, _ := json.MarshalIndent(msg, "", "  ")
				fmt.Printf("  Full message: %s\n", bytes)
			}
		}
	}
}
