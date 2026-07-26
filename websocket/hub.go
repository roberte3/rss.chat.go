package websocket

import (
	"context"
	"sync"
)

// Message types for websocket events
type MessageType string

const (
	TypeNewItem    MessageType = "newItem"
	TypeUpdatedItem MessageType = "updatedItem"
	TypeToggledLike MessageType = "toggledLike"
)

// Event represents a change that subscribers should know about
type Event struct {
	Type   MessageType `json:"type"`
	ItemID int64       `json:"itemId"`
	Author string      `json:"author"`
	Data   interface{} `json:"data"`
}

// Subscriber represents a connected websocket client
type Subscriber struct {
	id       string
	conn     Connection
	itemsub  chan *Event
	done     chan struct{}
}

// Connection is the interface for websocket connections
type Connection interface {
	WriteJSON(v interface{}) error
	Close() error
}

// Hub manages all subscriber connections and broadcasts events
type Hub struct {
	mu          sync.RWMutex
	subscribers map[string]*Subscriber
	broadcast   chan *Event
	register    chan *Subscriber
	unregister  chan *Subscriber
}

// NewHub creates a new event hub
func NewHub() *Hub {
	return &Hub{
		subscribers: make(map[string]*Subscriber),
		broadcast:   make(chan *Event, 100),
		register:    make(chan *Subscriber),
		unregister:  make(chan *Subscriber, 10),
	}
}

// Start runs the hub's event loop (must be called in a goroutine)
func (h *Hub) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			h.closeAll()
			return
		case sub := <-h.register:
			h.mu.Lock()
			h.subscribers[sub.id] = sub
			h.mu.Unlock()
		case sub := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.subscribers[sub.id]; ok {
				delete(h.subscribers, sub.id)
				close(sub.itemsub)
			}
			h.mu.Unlock()
		case event := <-h.broadcast:
			h.broadcastEvent(event)
		}
	}
}

// broadcastEvent sends an event to all connected subscribers
func (h *Hub) broadcastEvent(event *Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, sub := range h.subscribers {
		select {
		case sub.itemsub <- event:
		default:
			// Drop event if subscriber's channel is full to avoid blocking
		}
	}
}

// Broadcast sends an event to all subscribers
func (h *Hub) Broadcast(event *Event) {
	select {
	case h.broadcast <- event:
	default:
		// Channel full, drop event to prevent deadlock
	}
}

// Subscribe registers a new subscriber and returns it
func (h *Hub) Subscribe(id string, conn Connection) *Subscriber {
	sub := &Subscriber{
		id:      id,
		conn:    conn,
		itemsub: make(chan *Event, 10),
		done:    make(chan struct{}),
	}
	h.register <- sub
	return sub
}

// Unsubscribe removes a subscriber from the hub
func (h *Hub) Unsubscribe(sub *Subscriber) {
	h.unregister <- sub
}

// Run starts the subscriber's event loop (must be called in a goroutine)
func (s *Subscriber) Run(ctx context.Context) {
	defer s.conn.Close()
	defer close(s.done)

	for {
		select {
		case <-ctx.Done():
			return
		case event := <-s.itemsub:
			if event == nil {
				// Channel closed, hub is shutting down
				return
			}
			// Send event to client as JSON
			msg := map[string]interface{}{
				"type":   event.Type,
				"itemId": event.ItemID,
				"author": event.Author,
				"data":   event.Data,
			}
			if err := s.conn.WriteJSON(msg); err != nil {
				// Connection error, stop
				return
			}
		}
	}
}

// closeAll closes all subscriber connections
func (h *Hub) closeAll() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, sub := range h.subscribers {
		close(sub.itemsub)
		sub.conn.Close()
	}
	h.subscribers = make(map[string]*Subscriber)
}

// SubscriberCount returns the number of active subscribers (for testing/monitoring)
func (h *Hub) SubscriberCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subscribers)
}
