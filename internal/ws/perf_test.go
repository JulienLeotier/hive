package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/JulienLeotier/hive/internal/event"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

// TestBroadcastLatency asserts Story 8.5 SLA: event delivery to a connected
// WebSocket client stays under 100ms. Stands up a real httptest server so we
// measure the full publish → broadcast → framing → client read path.
func TestBroadcastLatency(t *testing.T) {
	hub := NewHub()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.HandleWS)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"
	u.Path = "/ws"

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)
	defer conn.Close()

	// Give the hub a moment to register the new client.
	time.Sleep(20 * time.Millisecond)

	var worst time.Duration
	for i := 0; i < 20; i++ {
		start := time.Now()
		hub.Broadcast(event.Event{
			ID:        int64(i),
			Type:      "perf.tick",
			Source:    "bench",
			Payload:   "{}",
			CreatedAt: start,
		})

		conn.SetReadDeadline(time.Now().Add(time.Second))
		_, msg, err := conn.ReadMessage()
		require.NoError(t, err)
		var parsed event.Event
		require.NoError(t, json.Unmarshal(msg, &parsed))
		d := time.Since(start)
		if d > worst {
			worst = d
		}
	}
	t.Logf("websocket worst-case delivery = %s", worst)
	if worst > 100*time.Millisecond {
		t.Fatalf("broadcast delivery %s exceeds 100ms SLA", worst)
	}
}
