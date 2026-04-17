package notify

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func enabledCfg() EmailConfig {
	return EmailConfig{Host: "smtp.example.com", Port: 587, From: "a@x", To: []string{"b@y"}}
}

func TestEnabledRequiresCoreFields(t *testing.T) {
	assert.False(t, EmailConfig{}.Enabled())
	assert.False(t, EmailConfig{Host: "x", Port: 25}.Enabled(), "from + to required")
	assert.True(t, enabledCfg().Enabled())
}

func TestFormatIncludesPayload(t *testing.T) {
	e := event.Event{
		Type:      "task.failed",
		Source:    "agent-1",
		Payload:   `{"task_id":"t1","error":"timeout"}`,
		CreatedAt: time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
	}
	subject, body := format(e)
	assert.Contains(t, subject, "TASK.FAILED")
	assert.Contains(t, subject, "agent-1")
	assert.Contains(t, body, "timeout")
	assert.Contains(t, body, "2026-04-17")
}

func TestExtractAddressStripsDisplayName(t *testing.T) {
	assert.Equal(t, "alerts@hive.io", extractAddress("Hive <alerts@hive.io>"))
	assert.Equal(t, "alerts@hive.io", extractAddress("alerts@hive.io"))
	assert.Equal(t, "alerts@hive.io", extractAddress("  alerts@hive.io  "))
}

func TestAttachIsNoopWhenDisabled(t *testing.T) {
	n := NewNotifier(EmailConfig{}) // empty = disabled
	// With no bus calls, Attach returning silently is the whole contract.
	n.Attach(nil) // nil bus is safe because no subscribers are registered
}

func TestAttachSendsOnOpsEvents(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()

	bus := event.NewBus(st.DB)
	var sent int32
	var mu sync.Mutex
	var captured []string

	n := NewNotifier(enabledCfg())
	n.sendFunc = func(ctx context.Context, cfg EmailConfig, subject, body string) error {
		atomic.AddInt32(&sent, 1)
		mu.Lock()
		captured = append(captured, subject)
		mu.Unlock()
		return nil
	}
	n.debounce = 0 // disable debounce for deterministic test
	n.Attach(bus)

	_, err = bus.Publish(context.Background(), "task.failed", "agent-1", map[string]any{"error": "x"})
	require.NoError(t, err)
	_, err = bus.Publish(context.Background(), "cost.alert", "budget_tracker", map[string]any{"over": 100})
	require.NoError(t, err)
	_, err = bus.Publish(context.Background(), "agent.isolated", "breaker", map[string]any{"agent": "z"})
	require.NoError(t, err)
	// Unrelated event — should NOT fire.
	_, err = bus.Publish(context.Background(), "task.completed", "agent-1", map[string]any{})
	require.NoError(t, err)

	assert.Eventually(t, func() bool { return atomic.LoadInt32(&sent) == 3 },
		2*time.Second, 20*time.Millisecond, "ops-shaped events should trigger 3 sends")

	mu.Lock()
	defer mu.Unlock()
	joined := strings.Join(captured, "|")
	assert.Contains(t, joined, "TASK.FAILED")
	assert.Contains(t, joined, "COST.ALERT")
	assert.Contains(t, joined, "AGENT.ISOLATED")
}

func TestDebounceSuppressesBurst(t *testing.T) {
	fixedNow := time.Now()
	n := NewNotifier(enabledCfg()).WithDebounce(1 * time.Minute)
	n.now = func() time.Time { return fixedNow }

	assert.True(t, n.shouldSend("task.failed"), "first event passes")
	assert.False(t, n.shouldSend("task.failed"), "immediate repeat is debounced")
	assert.True(t, n.shouldSend("cost.alert"), "different type is not affected")

	// Advance time past the debounce window.
	n.now = func() time.Time { return fixedNow.Add(2 * time.Minute) }
	assert.True(t, n.shouldSend("task.failed"), "after window the next event passes")
}
