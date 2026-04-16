package ws

import (
	"log/slog"
	"net/http"
	"sync"

	"github.com/JulienLeotier/hive/internal/event"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Hub manages WebSocket connections and broadcasts events to all clients.
type Hub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]bool
}

// NewHub creates a WebSocket hub.
func NewHub() *Hub {
	return &Hub{clients: make(map[*websocket.Conn]bool)}
}

// HandleWS upgrades an HTTP connection to WebSocket and registers the client.
func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "error", err)
		return
	}

	h.mu.Lock()
	h.clients[conn] = true
	h.mu.Unlock()

	slog.Debug("websocket client connected", "remote", conn.RemoteAddr())

	// Read loop — just for detecting disconnects
	go func() {
		defer func() {
			h.mu.Lock()
			delete(h.clients, conn)
			h.mu.Unlock()
			conn.Close()
			slog.Debug("websocket client disconnected", "remote", conn.RemoteAddr())
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()
}

// Broadcast sends an event to all connected WebSocket clients.
func (h *Hub) Broadcast(evt event.Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	msg := map[string]any{
		"id":         evt.ID,
		"type":       evt.Type,
		"source":     evt.Source,
		"payload":    evt.Payload,
		"created_at": evt.CreatedAt,
	}

	for conn := range h.clients {
		if err := conn.WriteJSON(msg); err != nil {
			slog.Debug("websocket write failed", "error", err)
			conn.Close()
			delete(h.clients, conn)
		}
	}
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
