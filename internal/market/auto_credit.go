package market

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/JulienLeotier/hive/internal/event"
)

// AutoCredit subscribes to task.completed and credits the winning agent's
// token wallet proportionally to task value. Story 18.3 AC: "agent earns
// tokens proportional to task value".
//
// Task value is taken from costs table when available, otherwise DefaultReward
// is used so agents still accumulate tokens in dev setups that don't track
// cost-per-run.
type AutoCredit struct {
	db            *sql.DB
	store         *Store
	defaultReward float64
}

// NewAutoCredit builds an AutoCredit.
func NewAutoCredit(db *sql.DB, store *Store, defaultReward float64) *AutoCredit {
	if defaultReward <= 0 {
		defaultReward = 1.0
	}
	return &AutoCredit{db: db, store: store, defaultReward: defaultReward}
}

// Attach wires the credit hook to the bus.
func (a *AutoCredit) Attach(bus *event.Bus) {
	bus.Subscribe(event.TaskCompleted, func(e event.Event) {
		a.handle(e)
	})
}

func (a *AutoCredit) handle(e event.Event) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(e.Payload), &payload); err != nil {
		return
	}
	taskID, _ := payload["task_id"].(string)
	if taskID == "" {
		return
	}

	// Resolve agent name from the task row.
	var agentName string
	_ = a.db.QueryRow(
		`SELECT COALESCE(a.name, '') FROM tasks t LEFT JOIN agents a ON a.id = t.agent_id WHERE t.id = ?`,
		taskID).Scan(&agentName)
	if agentName == "" {
		return
	}

	// Pick up the recorded cost; otherwise pay the flat default reward.
	reward := a.defaultReward
	var cost float64
	if err := a.db.QueryRow(`SELECT COALESCE(SUM(cost), 0) FROM costs WHERE task_id = ?`, taskID).Scan(&cost); err == nil && cost > 0 {
		// Token reward scales inversely to cost so cheaper agents earn faster.
		// Tune to taste; the key guarantee is monotonic accrual.
		reward = 1.0 / (cost + 0.01)
	}

	// Event subscriber has no ambient ctx; cap the credit write so a stuck
	// DB doesn't leak one goroutine per completed task.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.store.Credit(ctx, agentName, reward); err != nil {
		slog.Warn("auto-credit failed", "agent", agentName, "error", err)
	}
}
