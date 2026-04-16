package cost

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
)

// Budget describes a daily spending limit for an agent (or "*" for global).
type Budget struct {
	ID         string  `json:"id"`
	AgentName  string  `json:"agent_name"`
	DailyLimit float64 `json:"daily_limit"`
	Enabled    bool    `json:"enabled"`
}

// Alert flags a budget breach. Computed on demand from budgets + today's spend.
type Alert struct {
	AgentName  string  `json:"agent_name"`
	DailyLimit float64 `json:"daily_limit"`
	Spend      float64 `json:"spend"`
	Breached   bool    `json:"breached"`
}

// SetBudget upserts a daily budget for an agent.
func (t *Tracker) SetBudget(ctx context.Context, agentName string, dailyLimit float64) error {
	if dailyLimit < 0 {
		return fmt.Errorf("daily limit must be non-negative")
	}

	// Upsert: replace any existing row for this agent so the CLI behaves as "set".
	if _, err := t.db.ExecContext(ctx,
		`DELETE FROM budget_alerts WHERE agent_name = ?`, agentName); err != nil {
		return fmt.Errorf("clearing existing budget: %w", err)
	}

	id := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
	_, err := t.db.ExecContext(ctx,
		`INSERT INTO budget_alerts (id, agent_name, daily_limit, enabled) VALUES (?, ?, ?, 1)`,
		id, agentName, dailyLimit)
	return err
}

// DeleteBudget removes the budget for an agent.
func (t *Tracker) DeleteBudget(ctx context.Context, agentName string) error {
	_, err := t.db.ExecContext(ctx,
		`DELETE FROM budget_alerts WHERE agent_name = ?`, agentName)
	return err
}

// ListBudgets returns all configured budgets.
func (t *Tracker) ListBudgets(ctx context.Context) ([]Budget, error) {
	rows, err := t.db.QueryContext(ctx,
		`SELECT id, agent_name, daily_limit, enabled FROM budget_alerts ORDER BY agent_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Budget
	for rows.Next() {
		var b Budget
		var enabled int
		if err := rows.Scan(&b.ID, &b.AgentName, &b.DailyLimit, &enabled); err != nil {
			return nil, err
		}
		b.Enabled = enabled == 1
		out = append(out, b)
	}
	return out, rows.Err()
}

// EvaluateAlerts returns breach status for every configured budget.
// When an event bus is attached via WithBus, breaches emit cost.alert events.
func (t *Tracker) EvaluateAlerts(ctx context.Context) ([]Alert, error) {
	budgets, err := t.ListBudgets(ctx)
	if err != nil {
		return nil, err
	}

	alerts := make([]Alert, 0, len(budgets))
	for _, b := range budgets {
		if !b.Enabled {
			continue
		}
		spend, err := t.DailyCostForAgent(ctx, b.AgentName)
		if err != nil {
			return nil, err
		}
		a := Alert{
			AgentName:  b.AgentName,
			DailyLimit: b.DailyLimit,
			Spend:      spend,
			Breached:   spend >= b.DailyLimit,
		}
		alerts = append(alerts, a)
		if a.Breached && t.bus != nil {
			_ = t.bus(ctx, "cost.alert", "budget_tracker", map[string]any{
				"agent":       a.AgentName,
				"daily_limit": a.DailyLimit,
				"spend":       a.Spend,
			})
		}
	}
	return alerts, nil
}

// ensureBudgetsTable is used by tests when a fresh DB is created without the v0.3 migration.
// Normal callers rely on storage.Open running the migration.
func ensureBudgetsTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS budget_alerts (
		id TEXT PRIMARY KEY,
		agent_name TEXT NOT NULL,
		daily_limit REAL NOT NULL,
		enabled INTEGER DEFAULT 1,
		last_alerted_date TEXT,
		created_at TEXT DEFAULT (datetime('now'))
	)`)
	return err
}
