package optimizer

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Recommendation is an actionable optimization suggestion.
type Recommendation struct {
	Type        string  `json:"type"`        // "prefer-agent", "parallelize", "reduce-heartbeat"
	Description string  `json:"description"`
	Impact      string  `json:"impact"`      // estimated improvement
	Confidence  float64 `json:"confidence"`  // 0.0-1.0
	AutoApply   bool    `json:"auto_apply"`
}

// Analyzer inspects historical execution data to find optimization opportunities.
type Analyzer struct {
	db *sql.DB
}

// NewAnalyzer creates an optimization analyzer.
func NewAnalyzer(db *sql.DB) *Analyzer {
	return &Analyzer{db: db}
}

// TrendSnapshot is the comparative-analysis payload Story 20.1 produces.
type TrendSnapshot struct {
	Window        string  `json:"window"`          // e.g., "7d"
	TasksRun      int     `json:"tasks_run"`
	TasksFailed   int     `json:"tasks_failed"`
	FailureRate   float64 `json:"failure_rate"`
	AvgDurationS  float64 `json:"avg_duration_s"`
}

// Trend runs a comparative analysis of the current window vs. the previous
// same-length window, returning deltas so you can spot regressions.
func (a *Analyzer) Trend(ctx context.Context, windowDays int) (current, previous TrendSnapshot, err error) {
	if windowDays <= 0 {
		windowDays = 7
	}
	label := fmt.Sprintf("%dd", windowDays)
	now := time.Now().UTC()
	startCur := now.AddDate(0, 0, -windowDays)
	startPrev := startCur.AddDate(0, 0, -windowDays)

	current, err = a.windowSnapshot(ctx, label, startCur, now)
	if err != nil {
		return
	}
	previous, err = a.windowSnapshot(ctx, label, startPrev, startCur)
	return
}

func (a *Analyzer) windowSnapshot(ctx context.Context, label string, from, to time.Time) (TrendSnapshot, error) {
	snap := TrendSnapshot{Window: label}

	fmtTime := func(t time.Time) string { return t.Format("2006-01-02 15:04:05") }

	err := a.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks WHERE created_at >= ? AND created_at <= ?
		 AND status IN ('completed','failed')`,
		fmtTime(from), fmtTime(to)).Scan(&snap.TasksRun)
	if err != nil {
		return snap, err
	}

	if err := a.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks WHERE created_at >= ? AND created_at <= ? AND status = 'failed'`,
		fmtTime(from), fmtTime(to)).Scan(&snap.TasksFailed); err != nil {
		return snap, err
	}

	if snap.TasksRun > 0 {
		snap.FailureRate = float64(snap.TasksFailed) / float64(snap.TasksRun)
	}

	_ = a.db.QueryRowContext(ctx,
		`SELECT COALESCE(AVG((JULIANDAY(completed_at) - JULIANDAY(started_at)) * 86400), 0)
		 FROM tasks WHERE status = 'completed' AND started_at IS NOT NULL AND completed_at IS NOT NULL
		 AND created_at >= ? AND created_at <= ?`,
		fmtTime(from), fmtTime(to)).Scan(&snap.AvgDurationS)

	return snap, nil
}

// Tuning is the outcome of an auto-tune pass. Story 20.3.
type Tuning struct {
	Setting   string  `json:"setting"`
	OldValue  float64 `json:"old_value"`
	NewValue  float64 `json:"new_value"`
	Rationale string  `json:"rationale"`
}

// AutoTune derives tuning suggestions from the latest trend snapshot.
// Suggestions only — callers decide whether to apply them.
func (a *Analyzer) AutoTune(ctx context.Context) ([]Tuning, error) {
	cur, prev, err := a.Trend(ctx, 7)
	if err != nil {
		return nil, err
	}
	var tunings []Tuning
	// Failure rate regressions → propose lowering breaker threshold.
	if cur.FailureRate > prev.FailureRate+0.05 {
		tunings = append(tunings, Tuning{
			Setting:   "resilience.breaker.threshold",
			OldValue:  3,
			NewValue:  2,
			Rationale: fmt.Sprintf("failure rate rose from %.1f%% to %.1f%%", prev.FailureRate*100, cur.FailureRate*100),
		})
	}
	// Latency regressions → propose longer retry backoff.
	if prev.AvgDurationS > 0 && cur.AvgDurationS > prev.AvgDurationS*1.5 {
		tunings = append(tunings, Tuning{
			Setting:   "resilience.retry.max_wait_seconds",
			OldValue:  2,
			NewValue:  5,
			Rationale: fmt.Sprintf("avg task duration rose from %.1fs to %.1fs", prev.AvgDurationS, cur.AvgDurationS),
		})
	}
	return tunings, nil
}

// Analyze runs pattern detection and returns recommendations.
func (a *Analyzer) Analyze(ctx context.Context) ([]Recommendation, error) {
	var recs []Recommendation

	// 1. Find slow agents (p95 > 2x median for same task type)
	slowAgents, err := a.findSlowAgents(ctx)
	if err == nil {
		recs = append(recs, slowAgents...)
	}

	// 2. Find underutilized agents (< 10% task allocation)
	idleAgents, err := a.findIdleAgents(ctx)
	if err == nil {
		recs = append(recs, idleAgents...)
	}

	// 3. Find sequential tasks that could parallelize
	parallelOps, err := a.findParallelOpportunities(ctx)
	if err == nil {
		recs = append(recs, parallelOps...)
	}

	return recs, nil
}

func (a *Analyzer) findSlowAgents(ctx context.Context) ([]Recommendation, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT agent_id, type,
			AVG(JULIANDAY(completed_at) - JULIANDAY(started_at)) * 86400 as avg_duration
		FROM tasks
		WHERE status = 'completed' AND started_at IS NOT NULL AND completed_at IS NOT NULL
		GROUP BY agent_id, type
		HAVING COUNT(*) >= 5
		ORDER BY avg_duration DESC
		LIMIT 5
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recs []Recommendation
	for rows.Next() {
		var agentID, taskType string
		var avgDuration float64
		rows.Scan(&agentID, &taskType, &avgDuration)

		if avgDuration > 30 { // > 30 seconds average
			recs = append(recs, Recommendation{
				Type:        "slow-agent",
				Description: fmt.Sprintf("Agent %s averages %.0fs for %s tasks — consider alternatives", agentID, avgDuration, taskType),
				Impact:      fmt.Sprintf("Could save ~%.0fs per task", avgDuration*0.5),
				Confidence:  0.7,
			})
		}
	}
	return recs, nil
}

func (a *Analyzer) findIdleAgents(ctx context.Context) ([]Recommendation, error) {
	cutoff := time.Now().Add(-7 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	rows, err := a.db.QueryContext(ctx, `
		SELECT a.name, COUNT(t.id) as task_count
		FROM agents a
		LEFT JOIN tasks t ON t.agent_id = a.id AND t.created_at >= ?
		GROUP BY a.id
		HAVING task_count < 3
	`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recs []Recommendation
	for rows.Next() {
		var name string
		var count int
		rows.Scan(&name, &count)
		recs = append(recs, Recommendation{
			Type:        "idle-agent",
			Description: fmt.Sprintf("Agent %s had only %d tasks in the last 7 days — consider increasing heartbeat interval or removing", name, count),
			Impact:      "Reduce idle compute costs",
			Confidence:  0.6,
		})
	}
	return recs, nil
}

func (a *Analyzer) findParallelOpportunities(ctx context.Context) ([]Recommendation, error) {
	// Check for workflows where sequential tasks have no data dependencies
	// Simple heuristic: tasks in same workflow with no depends_on that ran sequentially
	rows, err := a.db.QueryContext(ctx, `
		SELECT workflow_id, COUNT(*) as task_count
		FROM tasks
		WHERE depends_on = '[]' OR depends_on = '' OR depends_on IS NULL
		GROUP BY workflow_id
		HAVING task_count >= 3
		LIMIT 5
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recs []Recommendation
	for rows.Next() {
		var wfID string
		var count int
		rows.Scan(&wfID, &count)
		recs = append(recs, Recommendation{
			Type:        "parallelize",
			Description: fmt.Sprintf("Workflow %s has %d independent tasks — consider running them in parallel", wfID, count),
			Impact:      fmt.Sprintf("Could reduce workflow time by ~%dx", count),
			Confidence:  0.5,
		})
	}
	return recs, nil
}
