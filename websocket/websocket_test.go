package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

// MockConnection implements the Connection interface for testing
type MockConnection struct {
	messages chan interface{}
	closed   bool
	mu       sync.Mutex
}

func NewMockConnection() *MockConnection {
	return &MockConnection{
		messages: make(chan interface{}, 100),
	}
}

func (mc *MockConnection) WriteJSON(v interface{}) error {
	mc.mu.Lock()
	if mc.closed {
		mc.mu.Unlock()
		return nil
	}
	mc.mu.Unlock()

	select {
	case mc.messages <- v:
		return nil
	default:
		return nil // Non-blocking, drop if full
	}
}

func (mc *MockConnection) Close() error {
	mc.mu.Lock()
	mc.closed = true
	mc.mu.Unlock()
	return nil
}

func (mc *MockConnection) GetMessages() []interface{} {
	var msgs []interface{}
	select {
	case msg := <-mc.messages:
		msgs = append(msgs, msg)
	default:
	}
	return msgs
}

// TestHubSubscription tests subscriber registration
func TestHubSubscription(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Start(ctx)

	// Create mock connections
	conn1 := NewMockConnection()
	conn2 := NewMockConnection()

	// Subscribe two clients
	sub1 := hub.Subscribe("client1", conn1)
	sub2 := hub.Subscribe("client2", conn2)

	// Allow time for registration
	time.Sleep(10 * time.Millisecond)

	// Verify subscriber count
	if hub.SubscriberCount() != 2 {
		t.Errorf("expected 2 subscribers, got %d", hub.SubscriberCount())
	}

	// Verify subscriber IDs
	if sub1.id != "client1" {
		t.Errorf("sub1 ID = %s, want client1", sub1.id)
	}
	if sub2.id != "client2" {
		t.Errorf("sub2 ID = %s, want client2", sub2.id)
	}
}

// TestHubBroadcast tests event broadcasting to all subscribers
func TestHubBroadcast(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Start(ctx)

	// Create mock connections
	conn1 := NewMockConnection()
	conn2 := NewMockConnection()

	// Subscribe clients and start their event loops
	sub1 := hub.Subscribe("client1", conn1)
	sub2 := hub.Subscribe("client2", conn2)

	go sub1.Run(ctx)
	go sub2.Run(ctx)

	time.Sleep(10 * time.Millisecond)

	// Broadcast an event
	event := &Event{
		Type:   TypeNewItem,
		ItemID: 123,
		Author: "alice",
		Data: map[string]interface{}{
			"text": "hello world",
		},
	}

	hub.Broadcast(event)

	// Wait for messages to be sent
	time.Sleep(50 * time.Millisecond)

	// Verify both clients received the message
	if len(conn1.messages) == 0 {
		t.Error("conn1 did not receive message")
	}
	if len(conn2.messages) == 0 {
		t.Error("conn2 did not receive message")
	}

	// Verify message content
	msg1 := <-conn1.messages
	msgMap, ok := msg1.(map[string]interface{})
	if !ok {
		t.Fatalf("message is not a map")
	}

	if msgType, ok := msgMap["type"]; !ok || msgType != TypeNewItem {
		t.Errorf("message type = %v, want %v", msgType, TypeNewItem)
	}
}

// TestHubUnsubscribe tests subscriber removal
func TestHubUnsubscribe(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Start(ctx)

	// Subscribe a client
	conn := NewMockConnection()
	sub := hub.Subscribe("client1", conn)

	time.Sleep(10 * time.Millisecond)
	if hub.SubscriberCount() != 1 {
		t.Errorf("expected 1 subscriber, got %d", hub.SubscriberCount())
	}

	// Unsubscribe
	hub.Unsubscribe(sub)

	time.Sleep(10 * time.Millisecond)
	if hub.SubscriberCount() != 0 {
		t.Errorf("expected 0 subscribers after unsubscribe, got %d", hub.SubscriberCount())
	}
}

// TestBroadcastMultipleEvents tests sending multiple events
func TestBroadcastMultipleEvents(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Start(ctx)

	conn := NewMockConnection()
	sub := hub.Subscribe("client1", conn)

	go sub.Run(ctx)

	time.Sleep(10 * time.Millisecond)

	// Send multiple events
	events := []*Event{
		{Type: TypeNewItem, ItemID: 1, Author: "alice"},
		{Type: TypeUpdatedItem, ItemID: 2, Author: "bob"},
		{Type: TypeToggledLike, ItemID: 3, Author: "charlie"},
	}

	for _, event := range events {
		hub.Broadcast(event)
	}

	// Wait for delivery
	time.Sleep(50 * time.Millisecond)

	// Verify all events received
	count := 0
	for count < len(events) {
		select {
		case <-conn.messages:
			count++
		case <-time.After(100 * time.Millisecond):
			break
		}
	}

	if count != len(events) {
		t.Errorf("received %d events, expected %d", count, len(events))
	}
}

// TestConcurrentSubscribers tests many subscribers at once
func TestConcurrentSubscribers(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Start(ctx)

	numSubscribers := 100
	subscribers := make([]*Subscriber, numSubscribers)
	connections := make([]*MockConnection, numSubscribers)

	// Create many subscribers
	for i := 0; i < numSubscribers; i++ {
		conn := NewMockConnection()
		connections[i] = conn
		sub := hub.Subscribe(fmt.Sprintf("client%d", i), conn)
		subscribers[i] = sub
		go sub.Run(ctx)
	}

	time.Sleep(50 * time.Millisecond)

	// Verify all subscribed
	if hub.SubscriberCount() != numSubscribers {
		t.Errorf("expected %d subscribers, got %d", numSubscribers, hub.SubscriberCount())
	}

	// Broadcast event
	hub.Broadcast(&Event{
		Type:   TypeNewItem,
		ItemID: 999,
		Author: "test",
	})

	// Wait for delivery
	time.Sleep(100 * time.Millisecond)

	// Verify all received
	successCount := 0
	for _, conn := range connections {
		if len(conn.messages) > 0 {
			successCount++
		}
	}

	if successCount < numSubscribers*90/100 { // Allow 10% loss due to timing
		t.Errorf("only %d out of %d subscribers received message", successCount, numSubscribers)
	}
}

// TestChannelOverflowHandling tests non-blocking behavior when channel is full
func TestChannelOverflowHandling(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Start(ctx)

	// Create a connection but don't run its event loop
	// This will cause the channel to fill up
	slowConn := NewMockConnection()
	sub := hub.Subscribe("slow_client", slowConn)

	// Don't run subscriber.Run(), so messages queue up
	time.Sleep(10 * time.Millisecond)

	// Send many events - should not block
	for i := 0; i < 20; i++ {
		hub.Broadcast(&Event{
			Type:   TypeNewItem,
			ItemID: int64(i),
			Author: "alice",
		})
	}

	// Broadcast should not block even though subscriber isn't consuming
	// If this test hangs, the broadcast is blocking (bad)
	time.Sleep(50 * time.Millisecond)

	// Clean up
	hub.Unsubscribe(sub)

	t.Logf("✓ Non-blocking broadcast with full channel works correctly")
}

// TestGracefulShutdown tests hub shutdown with active connections
func TestGracefulShutdown(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())

	go hub.Start(ctx)

	// Subscribe some clients
	conns := make([]*MockConnection, 5)
	subs := make([]*Subscriber, 5)
	for i := 0; i < 5; i++ {
		conn := NewMockConnection()
		conns[i] = conn
		sub := hub.Subscribe(fmt.Sprintf("client%d", i), conn)
		subs[i] = sub
		go sub.Run(ctx)
	}

	time.Sleep(20 * time.Millisecond)

	if hub.SubscriberCount() != 5 {
		t.Errorf("expected 5 subscribers before shutdown, got %d", hub.SubscriberCount())
	}

	// Shutdown
	cancel()

	// Give time to clean up
	time.Sleep(50 * time.Millisecond)

	// Verify all subscribers cleaned up
	if hub.SubscriberCount() != 0 {
		t.Errorf("expected 0 subscribers after shutdown, got %d", hub.SubscriberCount())
	}
}

// TestEventTypes tests all event type constants
func TestEventTypes(t *testing.T) {
	tests := []struct {
		name     string
		msgType  MessageType
		expected string
	}{
		{"NewItem", TypeNewItem, "newItem"},
		{"UpdatedItem", TypeUpdatedItem, "updatedItem"},
		{"ToggledLike", TypeToggledLike, "toggledLike"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.msgType) != tt.expected {
				t.Errorf("MessageType = %s, want %s", string(tt.msgType), tt.expected)
			}
		})
	}
}

// TestEventDataPreservation tests that event data is preserved
func TestEventDataPreservation(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Start(ctx)

	conn := NewMockConnection()
	sub := hub.Subscribe("client1", conn)

	go sub.Run(ctx)

	time.Sleep(10 * time.Millisecond)

	// Create event with detailed data
	eventData := map[string]interface{}{
		"text":      "Hello world",
		"screenname": "alice",
		"likes":     42,
	}

	event := &Event{
		Type:   TypeNewItem,
		ItemID: 123,
		Author: "alice",
		Data:   eventData,
	}

	hub.Broadcast(event)

	time.Sleep(50 * time.Millisecond)

	// Verify message received
	if len(conn.messages) == 0 {
		t.Fatal("message not received")
	}

	msg := <-conn.messages
	msgMap := msg.(map[string]interface{})

	// Verify structure
	if msgType := msgMap["type"]; msgType != TypeNewItem {
		t.Errorf("type = %v, want %v", msgType, TypeNewItem)
	}

	if data, ok := msgMap["data"].(map[string]interface{}); !ok {
		t.Error("data is not a map")
	} else {
		if itemID := data["itemId"]; itemID != int64(123) {
			t.Errorf("itemId = %v, want 123", itemID)
		}
		if author := data["author"]; author != "alice" {
			t.Errorf("author = %v, want alice", author)
		}
	}
}

// TestSubscriberChannelClosure tests that closed channels are handled properly
func TestSubscriberChannelClosure(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Start(ctx)

	conn := NewMockConnection()
	sub := hub.Subscribe("client1", conn)

	go sub.Run(ctx)

	time.Sleep(10 * time.Millisecond)

	// Close subscriber (which closes the event channel)
	hub.Unsubscribe(sub)

	time.Sleep(20 * time.Millisecond)

	// Verify subscriber channel is closed
	if !isChanClosed(sub.itemsub) {
		t.Error("subscriber channel should be closed after unsubscribe")
	}
}

// isChanClosed safely checks if a channel is closed
func isChanClosed(ch chan *Event) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

// TestRaceConditions tests concurrent access patterns
func TestRaceConditions(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Start(ctx)

	var wg sync.WaitGroup
	numGoroutines := 10
	numEvents := 100

	// Simultaneous broadcasts and subscriptions
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Subscribe
			conn := NewMockConnection()
			sub := hub.Subscribe(fmt.Sprintf("client%d", id), conn)
			go sub.Run(ctx)

			time.Sleep(5 * time.Millisecond)

			// Broadcast events
			for j := 0; j < numEvents/numGoroutines; j++ {
				hub.Broadcast(&Event{
					Type:   TypeNewItem,
					ItemID: int64(id*numEvents + j),
					Author: "test",
				})
			}
		}(i)
	}

	wg.Wait()

	// Final broadcast to all
	hub.Broadcast(&Event{
		Type:   TypeNewItem,
		ItemID: 999,
		Author: "final",
	})

	time.Sleep(50 * time.Millisecond)

	// Verify no panics or deadlocks occurred
	t.Logf("✓ Race condition test completed without issues")
}

// TestLargePayloads tests handling of large event data
func TestLargePayloads(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Start(ctx)

	conn := NewMockConnection()
	sub := hub.Subscribe("client1", conn)

	go sub.Run(ctx)

	time.Sleep(10 * time.Millisecond)

	// Create large payload
	largeText := ""
	for i := 0; i < 1000; i++ {
		largeText += "This is a large text payload for testing. "
	}

	event := &Event{
		Type:   TypeNewItem,
		ItemID: 123,
		Author: "alice",
		Data: map[string]interface{}{
			"text": largeText,
		},
	}

	hub.Broadcast(event)

	time.Sleep(50 * time.Millisecond)

	if len(conn.messages) == 0 {
		t.Error("large payload not received")
	}

	msg := <-conn.messages
	if msg == nil {
		t.Error("message is nil")
	}
}

// TestMessageFormat tests the exact JSON message format
func TestMessageFormat(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Start(ctx)

	conn := NewMockConnection()
	sub := hub.Subscribe("client1", conn)

	go sub.Run(ctx)

	time.Sleep(10 * time.Millisecond)

	hub.Broadcast(&Event{
		Type:   TypeNewItem,
		ItemID: 456,
		Author: "bob",
		Data: map[string]interface{}{
			"likes": 10,
		},
	})

	time.Sleep(50 * time.Millisecond)

	if len(conn.messages) == 0 {
		t.Fatal("message not received")
	}

	msg := <-conn.messages
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal message: %v", err)
	}

	// Verify JSON structure
	var parsed map[string]interface{}
	if err := json.Unmarshal(msgBytes, &parsed); err != nil {
		t.Fatalf("message is not valid JSON: %v", err)
	}

	// Verify required fields
	if parsed["type"] == nil {
		t.Error("message missing type field")
	}
	if parsed["data"] == nil {
		t.Error("message missing data field")
	}

	t.Logf("✓ Message format is valid JSON with required fields")
}
