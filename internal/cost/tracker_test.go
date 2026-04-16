package cost

import (
	"context"
	"testing"

	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTracker(t *testing.T) *Tracker {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })

	st.DB.Exec(`CREATE TABLE IF NOT EXISTS costs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		agent_id TEXT NOT NULL,
		agent_name TEXT NOT NULL,
		workflow_id TEXT NOT NULL,
		task_id TEXT NOT NULL,
		cost REAL NOT NULL,
		created_at TEXT DEFAULT (datetime('now'))
	)`)

	return NewTracker(st.DB)
}

func TestRecordAndByAgent(t *testing.T) {
	tr := setupTracker(t)

	tr.Record(context.Background(), "a1", "reviewer", "wf1", "t1", 0.05)
	tr.Record(context.Background(), "a1", "reviewer", "wf1", "t2", 0.03)
	tr.Record(context.Background(), "a2", "writer", "wf1", "t3", 0.10)

	summaries, err := tr.ByAgent(context.Background())
	require.NoError(t, err)
	assert.Len(t, summaries, 2)
	assert.Equal(t, "writer", summaries[0].AgentName) // highest cost first
	assert.InDelta(t, 0.10, summaries[0].TotalCost, 0.001)
	assert.Equal(t, "reviewer", summaries[1].AgentName)
	assert.InDelta(t, 0.08, summaries[1].TotalCost, 0.001)
}

func TestDailyCost(t *testing.T) {
	tr := setupTracker(t)

	tr.Record(context.Background(), "a1", "reviewer", "wf1", "t1", 0.05)
	tr.Record(context.Background(), "a1", "reviewer", "wf1", "t2", 0.03)

	daily, err := tr.DailyCostForAgent(context.Background(), "reviewer")
	require.NoError(t, err)
	assert.InDelta(t, 0.08, daily, 0.001)
}
