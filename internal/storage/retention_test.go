package storage

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetention_SweepEventsAndTasks(t *testing.T) {
	store, err := Open(t.TempDir())
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// 100-day-old event → should be deleted at 90d window.
	oldEvt := time.Now().UTC().Add(-100 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	// 10-day-old event → retained.
	newEvt := time.Now().UTC().Add(-10 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	_, err = store.DB.ExecContext(ctx,
		`INSERT INTO events (type, source, payload, created_at) VALUES
		 ('t1', 's', '{}', ?), ('t2', 's', '{}', ?)`, oldEvt, newEvt)
	require.NoError(t, err)

	// 60-day-old completed task → deleted at 30d window.
	oldTaskDone := time.Now().UTC().Add(-60 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	// 60-day-old still running → retained (we never purge unresolved tasks).
	_, err = store.DB.ExecContext(ctx,
		`INSERT INTO tasks (id, workflow_id, type, status, completed_at, created_at) VALUES
		 ('old-done', 'wf', 't', 'completed', ?, ?),
		 ('old-running', 'wf', 't', 'running', NULL, ?)`,
		oldTaskDone, oldTaskDone, oldTaskDone)
	require.NoError(t, err)

	sweepRetention(ctx, store.DB, RetentionConfig{}) // defaults

	var evtCount, taskRunning, taskDone int
	require.NoError(t, store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM events`).Scan(&evtCount))
	require.NoError(t, store.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks WHERE status='running'`).Scan(&taskRunning))
	require.NoError(t, store.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks WHERE status='completed'`).Scan(&taskDone))

	assert.Equal(t, 1, evtCount, "100d-old event should be purged, 10d-old kept")
	assert.Equal(t, 1, taskRunning, "running task must never be purged by retention")
	assert.Equal(t, 0, taskDone, "60d-old completed task should be purged at 30d window")
}

func TestRetention_NegativeDisables(t *testing.T) {
	store, err := Open(t.TempDir())
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	oldEvt := time.Now().UTC().Add(-1000 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	_, err = store.DB.ExecContext(ctx,
		`INSERT INTO events (type, source, payload, created_at) VALUES ('old', 's', '{}', ?)`, oldEvt)
	require.NoError(t, err)

	// Negative = disabled; ancient event must be retained.
	sweepRetention(ctx, store.DB, RetentionConfig{EventsDays: -1})

	var n int
	require.NoError(t, store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM events`).Scan(&n))
	assert.Equal(t, 1, n, "retention disabled → no purge")
}
