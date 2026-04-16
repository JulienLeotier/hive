package webhook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupDispatcher(t *testing.T) *Dispatcher {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })

	st.DB.Exec(`CREATE TABLE IF NOT EXISTS webhooks (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		url TEXT NOT NULL,
		type TEXT NOT NULL,
		event_filter TEXT,
		enabled INTEGER DEFAULT 1,
		created_at TEXT DEFAULT (datetime('now'))
	)`)

	return NewDispatcher(st.DB)
}

func TestAddAndList(t *testing.T) {
	d := setupDispatcher(t)

	cfg, err := d.Add(context.Background(), "slack-alerts", "https://hooks.slack.com/test", "slack", `["task.failed"]`)
	require.NoError(t, err)
	assert.Equal(t, "slack-alerts", cfg.Name)

	configs, err := d.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, configs, 1)
}

func TestDispatchMatchingEvent(t *testing.T) {
	d := setupDispatcher(t)

	var received atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d.Add(context.Background(), "test-hook", srv.URL, "generic", `["task.failed"]`)

	d.Dispatch(context.Background(), event.Event{
		ID: 1, Type: "task.failed", Source: "system", Payload: `{"task_id":"t1"}`,
		CreatedAt: time.Now(),
	})

	time.Sleep(200 * time.Millisecond) // async dispatch
	assert.Equal(t, int32(1), received.Load())
}

func TestDispatchNonMatchingEvent(t *testing.T) {
	d := setupDispatcher(t)

	var received atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d.Add(context.Background(), "test-hook", srv.URL, "generic", `["task.failed"]`)

	d.Dispatch(context.Background(), event.Event{
		Type: "task.completed", Source: "system", CreatedAt: time.Now(),
	})

	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, int32(0), received.Load())
}

func TestSlackFormat(t *testing.T) {
	payload := formatPayload("slack", event.Event{Type: "task.failed", Source: "agent-1", Payload: `{"err":"timeout"}`})
	var msg map[string]string
	json.Unmarshal(payload, &msg)
	assert.Contains(t, msg["text"], "task.failed")
	assert.Contains(t, msg["text"], "agent-1")
}

func TestGitHubFormat(t *testing.T) {
	payload := formatPayload("github", event.Event{Type: "task.completed", Source: "system"})
	var msg map[string]any
	json.Unmarshal(payload, &msg)
	assert.Equal(t, "task.completed", msg["event_type"])
	assert.NotNil(t, msg["client_payload"])
}

func TestMatchesFilter(t *testing.T) {
	assert.True(t, matchesFilter("task.failed", `["task.failed","agent.isolated"]`))
	assert.False(t, matchesFilter("task.completed", `["task.failed"]`))
	assert.True(t, matchesFilter("anything", ""))
	assert.True(t, matchesFilter("task.failed", "task.failed,agent.isolated"))
}
