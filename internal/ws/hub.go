package ws

import (
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/JulienLeotier/hive/internal/event"
	"github.com/gorilla/websocket"
)

// Story 8.5: ping every PingPeriod to detect dead TCP connections, and any
// client that doesn't pong within PongTimeout is evicted. Tuned to be
// responsive without hammering healthy clients.
const (
	PingPeriod  = 30 * time.Second
	PongTimeout = 60 * time.Second
	WriteTimeout = 10 * time.Second
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
// Story 8.5: runs a ping/pong keepalive; stale connections are evicted.
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

	// Pong handler resets the read deadline so the next ping has breathing room.
	_ = conn.SetReadDeadline(time.Now().Add(PongTimeout))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(PongTimeout))
	})

	done := make(chan struct{})

	// Ping writer — every PingPeriod. If the write fails (TCP dead) we close
	// the connection which unblocks the read loop and triggers eviction.
	go func() {
		ticker := time.NewTicker(PingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.wmu.Lock()
				err := c.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(WriteTimeout))
				c.wmu.Unlock()
				if err != nil {
					c.conn.Close()
					return
				}
			case <-done:
				return
			}
		}
	}()

	// Read loop — returns when the connection is closed or misses a pong.
	go func() {
		defer func() {
			close(done)
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
