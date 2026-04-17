package notify

import (
	"context"
	"io"
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

func TestSlackEnabledRequiresURL(t *testing.T) {
	assert.False(t, SlackConfig{}.Enabled())
	assert.True(t, SlackConfig{WebhookURL: "https://hooks.slack.com/x"}.Enabled())
}

func TestSlackFormatContainsTypeAndSource(t *testing.T) {
	text := formatSlack(event.Event{Type: "task.failed", Source: "agent-1", Payload: "timeout"})
	assert.Contains(t, text, "task.failed")
	assert.Contains(t, text, "agent-1")
	assert.Contains(t, text, "timeout")
}

func TestSlackAttachPostsToWebhook(t *testing.T) {
	var got int32
	var lastBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastBody, _ = io.ReadAll(r.Body)
		atomic.AddInt32(&got, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()
	bus := event.NewBus(st.DB)

	n := NewSlackNotifier(SlackConfig{WebhookURL: srv.URL}).WithDebounce(0)
	n.Attach(bus)

	_, err = bus.Publish(context.Background(), "task.failed", "a1", map[string]any{"err": "boom"})
	require.NoError(t, err)

	assert.Eventually(t, func() bool { return atomic.LoadInt32(&got) == 1 },
		2*time.Second, 20*time.Millisecond)
	assert.Contains(t, string(lastBody), "task.failed")
	assert.Contains(t, string(lastBody), "\"text\":")
}

func TestSlackAttachNoopWhenDisabled(t *testing.T) {
	n := NewSlackNotifier(SlackConfig{}) // no URL
	n.Attach(nil)                        // must not panic
}
