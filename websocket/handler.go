package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/coder/websocket"
)

// Handler manages websocket connections for real-time updates
type Handler struct {
	hub           *Hub
	connIDCounter atomic.Int64
}

// NewHandler creates a new websocket handler
func NewHandler(hub *Hub) *Handler {
	return &Handler{
		hub: hub,
	}
}

// ServeHTTP handles incoming websocket connections on /subscribe
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // Allow connections without proper CORS in dev
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("websocket accept error: %v", err), http.StatusInternalServerError)
		return
	}
	defer conn.Close(websocket.StatusInternalError, "connection closed")

	// Generate unique ID for this connection
	connID := fmt.Sprintf("conn_%d", h.connIDCounter.Add(1))

	// Subscribe to hub
	subscriber := h.hub.Subscribe(connID, &wsConn{conn: conn})
	defer h.hub.Unsubscribe(subscriber)

	// Create context for this connection
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Send initial connection confirmation
	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"type":"connected","connId":"`+connID+`"}`)); err != nil {
		return
	}

	// Run subscriber event loop until context is cancelled or connection is closed
	go subscriber.Run(ctx)

	// Keep connection alive by listening for client messages
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		messageType, data, err := conn.Read(ctx)
		if err != nil {
			// Connection closed or error
			return
		}

		// Echo pong for ping messages (keep-alive)
		if messageType == websocket.MessageText && string(data) == "ping" {
			if err := conn.Write(ctx, websocket.MessageText, []byte("pong")); err != nil {
				return
			}
		}
	}
}

// wsConn wraps github.com/coder/websocket.Conn to implement Connection interface
type wsConn struct {
	conn *websocket.Conn
}

// WriteJSON sends a JSON message
func (wc *wsConn) WriteJSON(v interface{}) error {
	ctx := context.Background()

	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return wc.conn.Write(ctx, websocket.MessageText, data)
}

// Close closes the connection
func (wc *wsConn) Close() error {
	return wc.conn.Close(websocket.StatusNormalClosure, "closing")
}
