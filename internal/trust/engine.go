package trust

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"crypto/rand"

	"github.com/JulienLeotier/hive/internal/event"
	"github.com/oklog/ulid/v2"
)

// Trust levels in ascending order of autonomy.
const (
	LevelSupervised = "supervised"
	LevelGuided     = "guided"
	LevelAutonomous = "autonomous"
	LevelTrusted    = "trusted"
)

// Thresholds configures when an agent is promoted.
type Thresholds struct {
	GuidedAfterTasks     int     `yaml:"guided_after_tasks"`
	GuidedMaxErrorRate   float64 `yaml:"guided_max_error_rate"`
	AutonomousAfterTasks int     `yaml:"autonomous_after_tasks"`
	AutonomousMaxError   float64 `yaml:"autonomous_max_error_rate"`
	TrustedAfterTasks    int     `yaml:"trusted_after_tasks"`
	TrustedMaxError      float64 `yaml:"trusted_max_error_rate"`
}

// DefaultThresholds returns sensible defaults.
func DefaultThresholds() Thresholds {
	return Thresholds{
		GuidedAfterTasks:     50,
		GuidedMaxErrorRate:   0.10,
		AutonomousAfterTasks: 200,
		AutonomousMaxError:   0.05,
		TrustedAfterTasks:    500,
		TrustedMaxError:      0.02,
	}
}

// Engine manages trust levels for agents.
type Engine struct {
	db         *sql.DB
	thresholds Thresholds
	bus        *event.Bus
}

// NewEngine creates a trust engine.
func NewEngine(db *sql.DB, thresholds Thresholds) *Engine {
	return &Engine{db: db, thresholds: thresholds}
}

// WithBus attaches an event bus so promotions emit decision.* events.
func (e *Engine) WithBus(bus *event.Bus) *Engine {
	e.bus = bus
	return e
}

// AgentStats holds performance metrics for trust evaluation.
// Story 9.1 AC: tracks total tasks, success rate, error rate, consecutive successes.
type AgentStats struct {
	TotalTasks           int
	Successes            int
	Failures             int
	ErrorRate            float64
	ConsecutiveSuccesses int
	CurrentLevel         string
}

// GetStats returns performance stats for an agent.
func (e *Engine) GetStats(ctx context.Context, agentID string) (AgentStats, error) {
	var stats AgentStats

	err := e.db.QueryRowContext(ctx,
		`SELECT trust_level FROM agents WHERE id = ?`, agentID,
	).Scan(&stats.CurrentLevel)
	if err != nil {
		return stats, fmt.Errorf("getting agent trust level: %w", err)
	}

	err = e.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks WHERE agent_id = ? AND status IN ('completed', 'failed')`, agentID,
	).Scan(&stats.TotalTasks)
	if err != nil {
		return stats, err
	}

	if err := e.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks WHERE agent_id = ? AND status = 'completed'`, agentID,
	).Scan(&stats.Successes); err != nil {
		return stats, fmt.Errorf("counting successes: %w", err)
	}

	if err := e.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks WHERE agent_id = ? AND status = 'failed'`, agentID,
	).Scan(&stats.Failures); err != nil {
		return stats, fmt.Errorf("counting failures: %w", err)
	}

	if stats.TotalTasks > 0 {
		stats.ErrorRate = float64(stats.Failures) / float64(stats.TotalTasks)
	}

	// Story 9.1: consecutive successes = count of most-recent 'completed' tasks
	// in a row, broken by any 'failed' task.
	rows, err := e.db.QueryContext(ctx,
		`SELECT status FROM tasks
		 WHERE agent_id = ? AND status IN ('completed','failed')
		 ORDER BY created_at DESC LIMIT 200`, agentID)
	if err == nil {
		defer rows.Close()
		streak := 0
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				continue
			}
			if s == "completed" {
				streak++
			} else {
				break
			}
		}
		stats.ConsecutiveSuccesses = streak
	}

	return stats, nil
}

// Evaluate checks if an agent should be promoted and applies the promotion.
func (e *Engine) Evaluate(ctx context.Context, agentID string) (promoted bool, newLevel string, err error) {
	stats, err := e.GetStats(ctx, agentID)
	if err != nil {
		return false, "", err
	}

	targetLevel := e.calculateTargetLevel(stats)
	if targetLevel == stats.CurrentLevel {
		return false, stats.CurrentLevel, nil
	}

	// Only promote, never auto-demote
	if levelRank(targetLevel) <= levelRank(stats.CurrentLevel) {
		return false, stats.CurrentLevel, nil
	}

	if err := e.setLevel(ctx, agentID, stats.CurrentLevel, targetLevel, "auto_promotion",
		fmt.Sprintf("tasks=%d, error_rate=%.2f%%", stats.TotalTasks, stats.ErrorRate*100)); err != nil {
		return false, "", err
	}

	slog.Info("agent trust promoted",
		"agent_id", agentID,
		"from", stats.CurrentLevel,
		"to", targetLevel,
		"tasks", stats.TotalTasks,
		"error_rate", fmt.Sprintf("%.2f%%", stats.ErrorRate*100),
	)

	if e.bus != nil {
		_, _ = e.bus.Publish(ctx, event.DecisionTrustPromoted, "trust_engine", event.Decision{
			Action:  "promote",
			Subject: agentID,
			Reason:  fmt.Sprintf("%d tasks with %.2f%% error rate", stats.TotalTasks, stats.ErrorRate*100),
			Context: map[string]any{
				"from": stats.CurrentLevel,
				"to":   targetLevel,
			},
		})
	}

	return true, targetLevel, nil
}

// SetOverride records a trust override scoped to a specific task type.
// Returns an error if the level is invalid.
func (e *Engine) SetOverride(ctx context.Context, agentID, taskType, level, reason string) error {
	if !IsValidLevel(level) {
		return fmt.Errorf("invalid trust level: %s", level)
	}
	_, err := e.db.ExecContext(ctx,
		`INSERT INTO agent_trust_overrides (agent_id, task_type, level, reason)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(agent_id, task_type) DO UPDATE SET level = excluded.level, reason = excluded.reason`,
		agentID, taskType, level, reason)
	return err
}

// RemoveOverride clears a per-task-type override.
func (e *Engine) RemoveOverride(ctx context.Context, agentID, taskType string) error {
	_, err := e.db.ExecContext(ctx,
		`DELETE FROM agent_trust_overrides WHERE agent_id = ? AND task_type = ?`,
		agentID, taskType)
	return err
}

// EffectiveLevel returns the trust level for an agent *for a given task type*.
// If an override exists it wins, otherwise the agent's base level is returned.
func (e *Engine) EffectiveLevel(ctx context.Context, agentID, taskType string) (string, error) {
	var override string
	err := e.db.QueryRowContext(ctx,
		`SELECT level FROM agent_trust_overrides WHERE agent_id = ? AND task_type = ?`,
		agentID, taskType,
	).Scan(&override)
	if err == nil {
		return override, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}

	var base string
	if err := e.db.QueryRowContext(ctx,
		`SELECT trust_level FROM agents WHERE id = ?`, agentID,
	).Scan(&base); err != nil {
		return "", fmt.Errorf("agent %s not found", agentID)
	}
	return base, nil
}

// ListOverrides returns all per-task-type overrides for an agent.
func (e *Engine) ListOverrides(ctx context.Context, agentID string) (map[string]string, error) {
	rows, err := e.db.QueryContext(ctx,
		`SELECT task_type, level FROM agent_trust_overrides WHERE agent_id = ?`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var tt, lvl string
		if err := rows.Scan(&tt, &lvl); err != nil {
			return nil, err
		}
		out[tt] = lvl
	}
	return out, rows.Err()
}

// IsValidLevel reports whether the string is one of the four known trust levels.
func IsValidLevel(level string) bool {
	switch level {
	case LevelSupervised, LevelGuided, LevelAutonomous, LevelTrusted:
		return true
	}
	return false
}

// SetManual manually sets an agent's trust level.
func (e *Engine) SetManual(ctx context.Context, agentID, newLevel string) error {
	var currentLevel string
	err := e.db.QueryRowContext(ctx, `SELECT trust_level FROM agents WHERE id = ?`, agentID).Scan(&currentLevel)
	if err != nil {
		return fmt.Errorf("agent %s not found", agentID)
	}

	return e.setLevel(ctx, agentID, currentLevel, newLevel, "manual_override", "set by user")
}

func (e *Engine) setLevel(ctx context.Context, agentID, oldLevel, newLevel, reason, criteria string) error {
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE agents SET trust_level = ?, updated_at = datetime('now') WHERE id = ?`,
		newLevel, agentID,
	); err != nil {
		tx.Rollback()
		return err
	}

	id := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO trust_history (id, agent_id, old_level, new_level, reason, criteria) VALUES (?, ?, ?, ?, ?, ?)`,
		id.String(), agentID, oldLevel, newLevel, reason, criteria,
	); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (e *Engine) calculateTargetLevel(stats AgentStats) string {
	t := e.thresholds

	if stats.TotalTasks >= t.TrustedAfterTasks && stats.ErrorRate <= t.TrustedMaxError {
		return LevelTrusted
	}
	if stats.TotalTasks >= t.AutonomousAfterTasks && stats.ErrorRate <= t.AutonomousMaxError {
		return LevelAutonomous
	}
	if stats.TotalTasks >= t.GuidedAfterTasks && stats.ErrorRate <= t.GuidedMaxErrorRate {
		return LevelGuided
	}
	return LevelSupervised
}

func levelRank(level string) int {
	switch level {
	case LevelSupervised:
		return 0
	case LevelGuided:
		return 1
	case LevelAutonomous:
		return 2
	case LevelTrusted:
		return 3
	}
	return 0
}
