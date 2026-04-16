package cost

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

// Entry represents a cost record.
type Entry struct {
	AgentID    string  `json:"agent_id"`
	AgentName  string  `json:"agent_name"`
	WorkflowID string  `json:"workflow_id"`
	TaskID     string  `json:"task_id"`
	Cost       float64 `json:"cost"`
	CreatedAt  time.Time `json:"created_at"`
}

// Summary holds aggregated cost data.
type Summary struct {
	AgentName  string  `json:"agent_name"`
	TotalCost  float64 `json:"total_cost"`
	TaskCount  int     `json:"task_count"`
}

// PublishFunc is the minimal surface we need from event.Bus; keeps this package
// from importing event (and making cycles). *event.Bus.Publish satisfies it
// modulo the return type — wrap with event.PublishFunc(bus.Publish).
type PublishFunc func(ctx context.Context, eventType, source string, payload any) error

// Tracker manages cost tracking for agents and workflows.
type Tracker struct {
	db  *sql.DB
	bus PublishFunc
}

// WithBus installs a publisher so budget breaches emit cost.alert events.
func (t *Tracker) WithBus(publish PublishFunc) *Tracker {
	t.bus = publish
	return t
}

// NewTracker creates a cost tracker.
func NewTracker(db *sql.DB) *Tracker {
	return &Tracker{db: db}
}

// Record stores a cost entry for a completed task.
func (t *Tracker) Record(ctx context.Context, agentID, agentName, workflowID, taskID string, cost float64) error {
	_, err := t.db.ExecContext(ctx,
		`INSERT INTO costs (agent_id, agent_name, workflow_id, task_id, cost) VALUES (?, ?, ?, ?, ?)`,
		agentID, agentName, workflowID, taskID, cost,
	)
	if err != nil {
		return fmt.Errorf("recording cost: %w", err)
	}
	slog.Debug("cost recorded", "agent", agentName, "cost", cost)
	return nil
}

// ByAgent returns cost summaries grouped by agent.
func (t *Tracker) ByAgent(ctx context.Context) ([]Summary, error) {
	rows, err := t.db.QueryContext(ctx,
		`SELECT agent_name, SUM(cost), COUNT(*) FROM costs GROUP BY agent_name ORDER BY SUM(cost) DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []Summary
	for rows.Next() {
		var s Summary
		if err := rows.Scan(&s.AgentName, &s.TotalCost, &s.TaskCount); err != nil {
			continue
		}
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}

// DailyCostForAgent returns today's total cost for an agent.
func (t *Tracker) DailyCostForAgent(ctx context.Context, agentName string) (float64, error) {
	var total float64
	err := t.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(cost), 0) FROM costs WHERE agent_name = ? AND date(created_at) = date('now')`,
		agentName,
	).Scan(&total)
	return total, err
}
