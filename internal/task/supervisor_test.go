package task

import (
	"context"
	"testing"
	"time"

	"github.com/JulienLeotier/hive/internal/adapter"
	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAdapter struct {
	checkpointCalls int
	resumeCalls     int
	resumedWith     adapter.Checkpoint
}

func (f *fakeAdapter) Declare(context.Context) (adapter.AgentCapabilities, error) {
	return adapter.AgentCapabilities{}, nil
}
func (f *fakeAdapter) Invoke(context.Context, adapter.Task) (adapter.TaskResult, error) {
	return adapter.TaskResult{}, nil
}
func (f *fakeAdapter) Health(context.Context) (adapter.HealthStatus, error) {
	return adapter.HealthStatus{Status: "healthy"}, nil
}
func (f *fakeAdapter) Checkpoint(context.Context) (adapter.Checkpoint, error) {
	f.checkpointCalls++
	return adapter.Checkpoint{Data: map[string]any{"step": 3}}, nil
}
func (f *fakeAdapter) Resume(_ context.Context, cp adapter.Checkpoint) error {
	f.resumeCalls++
	f.resumedWith = cp
	return nil
}

func setupSupervisor(t *testing.T) (*Store, *Router, *CheckpointSupervisor) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })

	bus := event.NewBus(st.DB)
	store := NewStore(st.DB, bus)
	router := NewRouter(st.DB).WithBus(bus)
	sup := NewCheckpointSupervisor(store, router, 10*time.Millisecond, 100*time.Millisecond)
	return store, router, sup
}

func TestSupervisorReassignsStaleTask(t *testing.T) {
	store, _, sup := setupSupervisor(t)
	ctx := context.Background()

	// Insert a running task with checkpoint_at an hour in the past
	past := time.Now().Add(-time.Hour).UTC().Format("2006-01-02 15:04:05")
	_, err := store.db.ExecContext(ctx,
		`INSERT INTO tasks (id, workflow_id, type, status, agent_id, input, started_at, checkpoint_at)
		 VALUES ('t1','w1','x','running','a1','{}', ?, ?)`, past, past)
	require.NoError(t, err)

	n, err := sup.Sweep(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	var status string
	store.db.QueryRow(`SELECT status FROM tasks WHERE id = 't1'`).Scan(&status)
	assert.Equal(t, "pending", status)
}

func TestSupervisorIgnoresFreshCheckpoint(t *testing.T) {
	store, _, sup := setupSupervisor(t)
	ctx := context.Background()

	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := store.db.ExecContext(ctx,
		`INSERT INTO tasks (id, workflow_id, type, status, agent_id, input, started_at, checkpoint_at)
		 VALUES ('t1','w1','x','running','a1','{}', ?, ?)`, now, now)
	require.NoError(t, err)

	n, err := sup.Sweep(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

func TestSupervisorStartStop(t *testing.T) {
	_, _, sup := setupSupervisor(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sup.Start(ctx)
	time.Sleep(30 * time.Millisecond)
	sup.Stop()
}

func TestSupervisorPollPersistsCheckpoint(t *testing.T) {
	store, _, sup := setupSupervisor(t)
	ctx := context.Background()

	_, err := store.db.ExecContext(ctx,
		`INSERT INTO tasks (id, workflow_id, type, status, agent_id, input)
		 VALUES ('t1','w','x','running','a1','{}')`)
	require.NoError(t, err)

	fa := &fakeAdapter{}
	sup.WithAdapterResolver(func(agentID string) adapter.Adapter { return fa })

	require.NoError(t, sup.Poll(ctx))
	assert.Equal(t, 1, fa.checkpointCalls)

	var cp string
	store.db.QueryRow(`SELECT checkpoint FROM tasks WHERE id='t1'`).Scan(&cp)
	assert.Contains(t, cp, `"step":3`)
}

func TestSupervisorResumeOnAgentPassesCheckpoint(t *testing.T) {
	store, _, sup := setupSupervisor(t)
	ctx := context.Background()

	_, err := store.db.ExecContext(ctx,
		`INSERT INTO tasks (id, workflow_id, type, status, agent_id, input, checkpoint)
		 VALUES ('t1','w','x','pending','', '{}', '{"step":3}')`)
	require.NoError(t, err)

	fa := &fakeAdapter{}
	require.NoError(t, sup.ResumeOnAgent(ctx, "t1", fa))
	assert.Equal(t, 1, fa.resumeCalls)
}

func TestSaveCheckpointStampsTimestamp(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })

	bus := event.NewBus(st.DB)
	store := NewStore(st.DB, bus)
	ctx := context.Background()

	_, err = store.db.ExecContext(ctx,
		`INSERT INTO tasks (id, workflow_id, type, status, input) VALUES ('t1','w1','x','running','{}')`)
	require.NoError(t, err)

	require.NoError(t, store.SaveCheckpoint(ctx, "t1", `{"step":1}`))

	var cp, cpAt string
	store.db.QueryRow(`SELECT checkpoint, COALESCE(checkpoint_at,'') FROM tasks WHERE id='t1'`).Scan(&cp, &cpAt)
	assert.Equal(t, `{"step":1}`, cp)
	assert.NotEmpty(t, cpAt, "checkpoint_at should be set by SaveCheckpoint")
}
