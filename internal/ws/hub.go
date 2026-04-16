package ws

import (
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/JulienLeotier/hive/internal/event"
	"github.com/gorilla/websocket"
)

// Hub manages WebSocket connections and broadcasts events to all clients.
type Hub struct {
	mu      sync.Mutex
	clients map[*client]bool
}

// client wraps a websocket.Conn with a write mutex for concurrency safety.
type client struct {
	conn *websocket.Conn
	wmu  sync.Mutex
}

// NewHub creates a WebSocket hub.
func NewHub() *Hub {
	return &Hub{clients: make(map[*client]bool)}
}

// AllowedOrigins configures which origins can connect. Empty means localhost only.
var AllowedOrigins []string

func checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // same-origin or non-browser
	}
	// Allow localhost always
	if strings.Contains(origin, "localhost") || strings.Contains(origin, "127.0.0.1") {
		return true
	}
	for _, allowed := range AllowedOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}

var upgrader = websocket.Upgrader{
	CheckOrigin: checkOrigin,
}

// HandleWS upgrades an HTTP connection to WebSocket and registers the client.
func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "error", err)
		return
	}

	c := &client{conn: conn}

	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()

	slog.Debug("websocket client connected", "remote", conn.RemoteAddr())

	// Read loop — just for detecting disconnects
	go func() {
		defer func() {
			h.mu.Lock()
			delete(h.clients, c)
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
	h.mu.Lock()
	defer h.mu.Unlock()

	msg := map[string]any{
		"id":         evt.ID,
		"type":       evt.Type,
		"source":     evt.Source,
		"payload":    evt.Payload,
		"created_at": evt.CreatedAt,
	}

	var failed []*client
	for c := range h.clients {
		c.wmu.Lock()
		err := c.conn.WriteJSON(msg)
		c.wmu.Unlock()
		if err != nil {
			slog.Debug("websocket write failed", "error", err)
			c.conn.Close()
			failed = append(failed, c)
		}
	}
	for _, c := range failed {
		delete(h.clients, c)
	}
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}
