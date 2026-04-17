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
	PingPeriod   = 30 * time.Second
	PongTimeout  = 60 * time.Second
	WriteTimeout = 10 * time.Second

	// sendBuffer caps queued broadcasts per client. A client that can't
	// keep up within this window is evicted rather than blocking the hub.
	// Rationale: 64 events covers a short network stall (~a few seconds of
	// backlog at typical rates) without letting one slow client bloat memory.
	sendBuffer = 64
)

// Hub manages WebSocket connections and broadcasts events to all clients.
type Hub struct {
	mu      sync.Mutex
	clients map[*client]bool
}

// client wraps a websocket.Conn with a bounded send channel. The writer
// goroutine is the only path that calls WriteJSON/WriteControl, so no write
// mutex is needed — serialisation is structural.
type client struct {
	conn *websocket.Conn
	send chan any    // broadcast queue; closed on eviction
	once sync.Once   // guards close(send) to make eviction idempotent
	done chan struct{}
}

// NewHub creates a WebSocket hub.
func NewHub() *Hub {
	return &Hub{clients: make(map[*client]bool)}
}

// evict closes the client's send channel and signals its writer/reader to
// exit. Safe to call multiple times.
func (c *client) evict() {
	c.once.Do(func() {
		close(c.send)
	})
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

	c := &client{
		conn: conn,
		send: make(chan any, sendBuffer),
		done: make(chan struct{}),
	}

	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()

	slog.Debug("websocket client connected", "remote", conn.RemoteAddr())

	// Pong handler resets the read deadline so the next ping has breathing room.
	_ = conn.SetReadDeadline(time.Now().Add(PongTimeout))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(PongTimeout))
	})

	// Writer goroutine — sole owner of write operations on this conn. Drains
	// the send channel + interleaves pings. Exits when send is closed (evict)
	// or the connection errors out.
	go func() {
		ticker := time.NewTicker(PingPeriod)
		defer func() {
			ticker.Stop()
			conn.Close()
			close(c.done)
		}()
		for {
			select {
			case msg, ok := <-c.send:
				if !ok {
					return // evicted
				}
				_ = conn.SetWriteDeadline(time.Now().Add(WriteTimeout))
				if err := conn.WriteJSON(msg); err != nil {
					slog.Debug("websocket write failed", "error", err)
					return
				}
			case <-ticker.C:
				if err := conn.WriteControl(
					websocket.PingMessage, nil, time.Now().Add(WriteTimeout),
				); err != nil {
					return
				}
			}
		}
	}()

	// Reader goroutine — triggers eviction when the peer closes or misses
	// a pong. Never writes to conn.
	go func() {
		defer func() {
			c.evict()
			<-c.done // wait for writer to exit before unregistering
			h.mu.Lock()
			delete(h.clients, c)
			h.mu.Unlock()
			slog.Debug("websocket client disconnected", "remote", conn.RemoteAddr())
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}

// Broadcast queues an event for delivery to every connected client. A slow
// client whose buffer is full is evicted rather than blocking the hub or
// other clients. The hub lock is held only long enough to snapshot the
// client set; writes happen after the lock is released.
func (h *Hub) Broadcast(evt event.Event) {
	msg := map[string]any{
		"id":         evt.ID,
		"type":       evt.Type,
		"source":     evt.Source,
		"payload":    evt.Payload,
		"created_at": evt.CreatedAt,
	}

	h.mu.Lock()
	clients := make([]*client, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.Unlock()

	for _, c := range clients {
		select {
		case c.send <- msg:
			// queued
		default:
			// Buffer full — slow client. Evict; the reader goroutine will
			// complete the unregistration once the writer exits.
			slog.Debug("websocket client buffer full, evicting", "remote", c.conn.RemoteAddr())
			c.evict()
		}
	}
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}
