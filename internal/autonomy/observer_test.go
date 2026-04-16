package autonomy

import (
	"context"
	"testing"
	"time"

	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeClaimer struct {
	result string
	err    error
	calls  int
}

func (f *fakeClaimer) ClaimPendingForAgent(ctx context.Context, agentName string) (string, error) {
	f.calls++
	return f.result, f.err
}

func setupObs(t *testing.T) *Observer {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })

	_, err = st.DB.Exec(`INSERT INTO agents (id, name, type, config, capabilities, health_status) VALUES ('a1', 'worker', 'http', '{}', '{"task_types":["x"]}', 'healthy')`)
	require.NoError(t, err)
	return NewObserver(st.DB)
}

func TestObserverSnapshotCountsPending(t *testing.T) {
	obs := setupObs(t)
	_, err := obs.db.Exec(`INSERT INTO tasks (id, workflow_id, type, status, input) VALUES ('t1','w1','x','pending','{}')`)
	require.NoError(t, err)

	snap, err := obs.Snapshot(context.Background(), "worker")
	require.NoError(t, err)
	assert.Equal(t, 1, snap.PendingTasks)
	assert.Equal(t, 0, snap.AssignedToAgent)
	assert.Equal(t, 0, snap.RunningByAgent)
}

func TestObserverSnapshotCountsAgentWork(t *testing.T) {
	obs := setupObs(t)
	_, err := obs.db.Exec(`INSERT INTO tasks (id, workflow_id, type, status, agent_id, input) VALUES ('t1','w1','x','running','a1','{}')`)
	require.NoError(t, err)

	snap, err := obs.Snapshot(context.Background(), "worker")
	require.NoError(t, err)
	assert.Equal(t, 1, snap.RunningByAgent)
}

func TestHandlerClaimsPending(t *testing.T) {
	obs := setupObs(t)
	_, err := obs.db.Exec(`INSERT INTO tasks (id, workflow_id, type, status, input) VALUES ('t1','w1','x','pending','{}')`)
	require.NoError(t, err)

	claimer := &fakeClaimer{result: "t1"}
	bus := event.NewBus(obs.db)
	tracker := NewIdleTracker(3)
	h := NewDefaultHandler(obs, claimer, tracker, bus)

	require.NoError(t, h.Handle(context.Background(), "worker"))
	assert.Equal(t, 1, claimer.calls)

	// Wake-up event persisted
	events, err := bus.Query(context.Background(), event.QueryOpts{Type: WakeUpEventType})
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Contains(t, events[0].Payload, "\"action\":\"claim\"")
}

func TestHandlerIdleSuppression(t *testing.T) {
	obs := setupObs(t)
	claimer := &fakeClaimer{} // nothing to claim
	bus := event.NewBus(obs.db)
	tracker := NewIdleTracker(2)
	h := NewDefaultHandler(obs, claimer, tracker, bus)

	for i := 0; i < 5; i++ {
		require.NoError(t, h.Handle(context.Background(), "worker"))
	}

	events, err := bus.Query(context.Background(), event.QueryOpts{Type: WakeUpEventType})
	require.NoError(t, err)
	assert.Len(t, events, 2, "only the first two idle cycles should emit events")
}

func TestHandlerNoopWhenAgentBusy(t *testing.T) {
	obs := setupObs(t)
	_, err := obs.db.Exec(`INSERT INTO tasks (id, workflow_id, type, status, agent_id, input) VALUES ('t1','w1','x','running','a1','{}')`)
	require.NoError(t, err)

	claimer := &fakeClaimer{}
	bus := event.NewBus(obs.db)
	h := NewDefaultHandler(obs, claimer, NewIdleTracker(3), bus)

	require.NoError(t, h.Handle(context.Background(), "worker"))
	assert.Equal(t, 0, claimer.calls, "should not claim while agent is running a task")
}

func TestIdleTrackerRecordResetsOnAction(t *testing.T) {
	tr := NewIdleTracker(2)
	tr.RecordIdle("a")
	tr.RecordIdle("a")
	tr.RecordAction("a")
	assert.True(t, tr.RecordIdle("a"), "after a productive action, idle counter resets")
	assert.WithinDuration(t, time.Now(), tr.LastAction("a"), time.Second)
}
