package optimizer

import (
	"context"
	"testing"
	"time"

	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAnalyzer(t *testing.T) *Analyzer {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	return NewAnalyzer(st.DB)
}

func TestTrendReportsCounts(t *testing.T) {
	a := setupAnalyzer(t)
	ctx := context.Background()

	// 2 completed + 1 failed in the current 7-day window.
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := a.db.Exec(
		`INSERT INTO tasks (id, workflow_id, type, status, input, created_at, started_at, completed_at)
		 VALUES ('t1','w','x','completed','{}', ?, ?, ?)`, now, now, now)
	require.NoError(t, err)
	_, err = a.db.Exec(
		`INSERT INTO tasks (id, workflow_id, type, status, input, created_at)
		 VALUES ('t2','w','x','failed','{}', ?)`, now)
	require.NoError(t, err)
	_, err = a.db.Exec(
		`INSERT INTO tasks (id, workflow_id, type, status, input, created_at, started_at, completed_at)
		 VALUES ('t3','w','x','completed','{}', ?, ?, ?)`, now, now, now)
	require.NoError(t, err)

	cur, _, err := a.Trend(ctx, 7)
	require.NoError(t, err)
	assert.Equal(t, 3, cur.TasksRun)
	assert.Equal(t, 1, cur.TasksFailed)
	assert.InDelta(t, 1.0/3.0, cur.FailureRate, 0.01)
}

func TestAnalyzeRunsWithoutError(t *testing.T) {
	a := setupAnalyzer(t)
	// Seed enough history that each heuristic has something to chew on.
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	for i := 0; i < 6; i++ {
		_, _ = a.db.Exec(
			`INSERT INTO tasks (id, workflow_id, type, status, agent_id, input, started_at, completed_at, created_at)
			 VALUES (?, 'w', 'x', 'completed', 'a1', '{}', ?, ?, ?)`,
			"slow-"+string(rune('a'+i)), now, now, now)
	}
	_, _ = a.db.Exec(
		`INSERT INTO agents (id, name, type, config, capabilities, health_status)
		 VALUES ('a1','slow','http','{}','{}','healthy')`)

	recs, err := a.Analyze(context.Background())
	require.NoError(t, err)
	// Can't assert specific recommendations — heuristics depend on duration
	// distribution — but the call must succeed.
	_ = recs
}

func TestComparativeSlowdownDetectsFasterPeer(t *testing.T) {
	a := setupAnalyzer(t)
	now := time.Now().UTC()

	// Two agents on the same task type; slow averages ~3× faster.
	_, _ = a.db.Exec(`INSERT INTO agents (id, name, type, config, capabilities, health_status) VALUES ('a1','slow','http','{}','{}','healthy'), ('a2','fast','http','{}','{}','healthy')`)
	fmt := func(t time.Time) string { return t.Format("2006-01-02 15:04:05") }

	// 6 slow runs at ~30s
	for i := 0; i < 6; i++ {
		start := now.Add(-time.Duration(i) * time.Hour)
		_, _ = a.db.Exec(
			`INSERT INTO tasks (id, workflow_id, type, status, agent_id, input, started_at, completed_at, created_at)
			 VALUES (?, 'w', 'code-review', 'completed', 'a1', '{}', ?, ?, ?)`,
			"s-"+string(rune('a'+i)), fmt(start), fmt(start.Add(30*time.Second)), fmt(start))
	}
	// 6 fast runs at ~5s
	for i := 0; i < 6; i++ {
		start := now.Add(-time.Duration(i) * time.Hour)
		_, _ = a.db.Exec(
			`INSERT INTO tasks (id, workflow_id, type, status, agent_id, input, started_at, completed_at, created_at)
			 VALUES (?, 'w', 'code-review', 'completed', 'a2', '{}', ?, ?, ?)`,
			"f-"+string(rune('a'+i)), fmt(start), fmt(start.Add(5*time.Second)), fmt(start))
	}

	recs, err := a.findComparativeSlowdowns(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, recs, "slow agent must be flagged against fast peer")
	assert.Equal(t, "comparative-slowdown", recs[0].Type)
}

func TestSnapshotAndCompareBaseline(t *testing.T) {
	a := setupAnalyzer(t)
	ctx := context.Background()

	baseline, err := a.SnapshotBaseline(ctx, 7, "initial")
	require.NoError(t, err)
	assert.Equal(t, "initial", baseline.Description)

	delta, err := a.CompareToBaseline(ctx, baseline, 7)
	require.NoError(t, err)
	assert.True(t, delta.Improved, "fresh snapshot vs itself should be a tie / improved")
}

func TestAutoTuneReturnsSuggestionWhenFailureSpikes(t *testing.T) {
	a := setupAnalyzer(t)
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	past := time.Now().AddDate(0, 0, -10).UTC().Format("2006-01-02 15:04:05")

	// Previous window: 10 completed, 0 failed.
	for i := 0; i < 10; i++ {
		_, _ = a.db.Exec(
			`INSERT INTO tasks (id, workflow_id, type, status, input, created_at)
			 VALUES (?, 'w', 'x', 'completed', '{}', ?)`, "prev-"+string(rune(i+65)), past)
	}
	// Current window: 5 failed, 5 completed → 50% failure rate.
	for i := 0; i < 5; i++ {
		_, _ = a.db.Exec(
			`INSERT INTO tasks (id, workflow_id, type, status, input, created_at)
			 VALUES (?, 'w', 'x', 'failed', '{}', ?)`, "cur-f-"+string(rune(i+65)), now)
		_, _ = a.db.Exec(
			`INSERT INTO tasks (id, workflow_id, type, status, input, created_at)
			 VALUES (?, 'w', 'x', 'completed', '{}', ?)`, "cur-c-"+string(rune(i+65)), now)
	}

	tunings, err := a.AutoTune(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, tunings)
	assert.Contains(t, tunings[0].Setting, "breaker")
}
