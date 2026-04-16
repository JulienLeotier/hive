package agent

import (
	"context"
	"testing"

	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/resilience"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeReassigner struct {
	calls  []string
	result int
}

func (f *fakeReassigner) ReassignAgentTasks(ctx context.Context, name, reason string) (int, error) {
	f.calls = append(f.calls, name+":"+reason)
	return f.result, nil
}

func TestHealthWatcherIsolatesOnOpen(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })

	_, err = st.DB.Exec(`INSERT INTO agents (id, name, type, config, capabilities, health_status) VALUES ('a1','worker','http','{}','{}','healthy')`)
	require.NoError(t, err)

	mgr := NewManager(st.DB)
	bus := event.NewBus(st.DB)
	reassigner := &fakeReassigner{result: 2}
	watcher := NewHealthWatcher(mgr, reassigner, bus)

	breakers := resilience.NewBreakerRegistry(resilience.BreakerConfig{Threshold: 2, ResetTimeout: 0})
	breakers.OnStateChange(watcher.Hook())

	cb := breakers.Get("worker")
	cb.RecordFailure()
	cb.RecordFailure() // trips open

	a, err := mgr.GetByName(context.Background(), "worker")
	require.NoError(t, err)
	assert.Equal(t, "unavailable", a.HealthStatus)

	assert.Len(t, reassigner.calls, 1)

	events, err := bus.Query(context.Background(), event.QueryOpts{Type: event.AgentIsolated})
	require.NoError(t, err)
	assert.Len(t, events, 1)
}

func TestHealthWatcherRestoresOnClosed(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })

	_, err = st.DB.Exec(`INSERT INTO agents (id, name, type, config, capabilities, health_status) VALUES ('a1','worker','http','{}','{}','unavailable')`)
	require.NoError(t, err)

	mgr := NewManager(st.DB)
	bus := event.NewBus(st.DB)
	watcher := NewHealthWatcher(mgr, &fakeReassigner{}, bus)

	breakers := resilience.NewBreakerRegistry(resilience.BreakerConfig{Threshold: 2, ResetTimeout: 0})
	breakers.OnStateChange(watcher.Hook())

	cb := breakers.Get("worker")
	cb.RecordFailure()
	cb.RecordFailure() // open
	_ = cb.Allow()     // transition to half-open (reset timeout is 0)
	cb.RecordSuccess() // close

	a, err := mgr.GetByName(context.Background(), "worker")
	require.NoError(t, err)
	assert.Equal(t, "healthy", a.HealthStatus)
}
