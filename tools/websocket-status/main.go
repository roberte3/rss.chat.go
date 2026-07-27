package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"time"

	"github.com/coder/websocket"
)

// EventMessage represents a message from the server
type EventMessage struct {
	Type   string                 `json:"type"`
	ItemID int64                  `json:"itemId,omitempty"`
	Author string                 `json:"author,omitempty"`
	ConnID string                 `json:"connId,omitempty"`
	Data   map[string]interface{} `json:"data,omitempty"`
}

// stripHTML removes HTML tags from a string
func stripHTML(html string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	text := re.ReplaceAllString(html, "")
	text = strings.TrimSpace(text)
	return text
}

// truncate shortens a string to maxLen, adding "..." if truncated
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
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
		} else if msg.Type == "newItem" {
			// Display newItem with title/description preview
			fmt.Printf("[newItem] ")
			if msg.ItemID > 0 {
				fmt.Printf("#%d ", msg.ItemID)
			}
			if msg.Author != "" {
				fmt.Printf("by %s: ", msg.Author)
			}

			// Show title if present, otherwise show description preview
			if title, ok := msg.Data["title"].(string); ok && title != "" {
				fmt.Printf("\"%s\"", truncate(title, 60))
			} else if desc, ok := msg.Data["description"].(string); ok && desc != "" {
				plainText := stripHTML(desc)
				fmt.Printf("\"%s\"", truncate(plainText, 60))
			}
			fmt.Println()

			if *verbose {
				bytes, _ := json.MarshalIndent(msg, "", "  ")
				fmt.Printf("  Full message: %s\n", bytes)
			}
		} else {
			// Display other event types
			fmt.Printf("[%s] ", msg.Type)
			if msg.ItemID > 0 {
				fmt.Printf("ItemID=%d ", msg.ItemID)
			}
			if msg.Author != "" {
				fmt.Printf("Author=%s ", msg.Author)
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
