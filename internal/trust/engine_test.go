package trust

import (
	"context"
	"fmt"
	"testing"

	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupEngine(t *testing.T) *Engine {
	store, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })

	// Create trust_history table (v0.2 migration)
	store.DB.Exec(`CREATE TABLE IF NOT EXISTS trust_history (
		id TEXT PRIMARY KEY,
		agent_id TEXT NOT NULL,
		old_level TEXT NOT NULL,
		new_level TEXT NOT NULL,
		reason TEXT NOT NULL,
		criteria TEXT,
		created_at TEXT DEFAULT (datetime('now'))
	)`)

	// Insert test agent
	store.DB.Exec(`INSERT INTO agents (id, name, type, config, capabilities, health_status, trust_level)
		VALUES ('a1', 'test-agent', 'http', '{}', '{}', 'healthy', 'scripted')`)

	return NewEngine(store.DB, DefaultThresholds())
}

func insertTasks(t *testing.T, e *Engine, agentID string, completed, failed int) {
	for i := 0; i < completed; i++ {
		e.db.Exec(`INSERT INTO tasks (id, workflow_id, type, status, agent_id, input) VALUES (?, 'wf', 'test', 'completed', ?, '{}')`,
			fmt.Sprintf("t-c-%d", i), agentID)
	}
	for i := 0; i < failed; i++ {
		e.db.Exec(`INSERT INTO tasks (id, workflow_id, type, status, agent_id, input) VALUES (?, 'wf', 'test', 'failed', ?, '{}')`,
			fmt.Sprintf("t-f-%d", i), agentID)
	}
}

func TestGetStatsEmpty(t *testing.T) {
	e := setupEngine(t)
	stats, err := e.GetStats(context.Background(), "a1")
	require.NoError(t, err)
	assert.Equal(t, 0, stats.TotalTasks)
	assert.Equal(t, 0.0, stats.ErrorRate)
}

func TestGetStatsWithTasks(t *testing.T) {
	e := setupEngine(t)
	insertTasks(t, e, "a1", 45, 5)

	stats, err := e.GetStats(context.Background(), "a1")
	require.NoError(t, err)
	assert.Equal(t, 50, stats.TotalTasks)
	assert.Equal(t, 45, stats.Successes)
	assert.Equal(t, 5, stats.Failures)
	assert.InDelta(t, 0.10, stats.ErrorRate, 0.01)
}

func TestEvaluateNoPromotion(t *testing.T) {
	e := setupEngine(t)
	insertTasks(t, e, "a1", 10, 0)

	promoted, level, err := e.Evaluate(context.Background(), "a1")
	require.NoError(t, err)
	assert.False(t, promoted)
	assert.Equal(t, "scripted", level)
}

func TestEvaluatePromoteToGuided(t *testing.T) {
	e := setupEngine(t)

	// Set to supervised first
	e.db.Exec(`UPDATE agents SET trust_level = 'supervised' WHERE id = 'a1'`)
	insertTasks(t, e, "a1", 48, 2) // 50 tasks, 4% error → qualifies for guided

	promoted, level, err := e.Evaluate(context.Background(), "a1")
	require.NoError(t, err)
	assert.True(t, promoted)
	assert.Equal(t, LevelGuided, level)
}

func TestSetManual(t *testing.T) {
	e := setupEngine(t)

	err := e.SetManual(context.Background(), "a1", LevelTrusted)
	require.NoError(t, err)

	var level string
	e.db.QueryRow(`SELECT trust_level FROM agents WHERE id = 'a1'`).Scan(&level)
	assert.Equal(t, LevelTrusted, level)

	// Check history
	var reason string
	e.db.QueryRow(`SELECT reason FROM trust_history WHERE agent_id = 'a1'`).Scan(&reason)
	assert.Equal(t, "manual_override", reason)
}

func TestNeverAutoDemote(t *testing.T) {
	e := setupEngine(t)
	e.db.Exec(`UPDATE agents SET trust_level = 'trusted' WHERE id = 'a1'`)
	insertTasks(t, e, "a1", 5, 5) // 50% error rate

	promoted, level, err := e.Evaluate(context.Background(), "a1")
	require.NoError(t, err)
	assert.False(t, promoted) // Should NOT demote
	assert.Equal(t, "trusted", level)
}
