package autonomy

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/JulienLeotier/hive/internal/event"
)

// WakeUpEventType is emitted for every wake-up cycle decision.
const WakeUpEventType = "agent.wakeup"

// Observation is the snapshot collected at the start of a wake-up cycle.
type Observation struct {
	PendingTasks    int
	AssignedToAgent int
	RunningByAgent  int
	LastActionAt    time.Time
}

// Decision is what the cycle chose to do.
type Decision struct {
	AgentName string    `json:"agent"`
	State     string    `json:"state"`
	Action    string    `json:"action"`           // claim, idle, escalate, noop
	TaskID    string    `json:"task_id,omitempty"`
	Reason    string    `json:"reason,omitempty"`
	Timestamp time.Time `json:"ts"`
}

// Observer collects state from the database so a WakeUpHandler can reason about it.
type Observer struct {
	db *sql.DB
}

// NewObserver builds an observer backed by the given database.
func NewObserver(db *sql.DB) *Observer {
	return &Observer{db: db}
}

// Snapshot returns the current Observation for an agent.
func (o *Observer) Snapshot(ctx context.Context, agentName string) (Observation, error) {
	var obs Observation

	if err := o.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks WHERE status = 'pending'`).Scan(&obs.PendingTasks); err != nil {
		return obs, fmt.Errorf("pending count: %w", err)
	}

	if err := o.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks t JOIN agents a ON a.id = t.agent_id
		 WHERE a.name = ? AND t.status = 'assigned'`, agentName).Scan(&obs.AssignedToAgent); err != nil {
		return obs, fmt.Errorf("assigned count: %w", err)
	}

	if err := o.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks t JOIN agents a ON a.id = t.agent_id
		 WHERE a.name = ? AND t.status = 'running'`, agentName).Scan(&obs.RunningByAgent); err != nil {
		return obs, fmt.Errorf("running count: %w", err)
	}

	return obs, nil
}

// IdleTracker prevents busywork by remembering when an agent last took a productive action.
type IdleTracker struct {
	mu           sync.Mutex
	lastProduct  map[string]time.Time
	idleCycles   map[string]int
	maxIdleLogs  int
}

// NewIdleTracker builds an idle tracker. After maxIdleLogs consecutive idle cycles,
// the tracker stops re-emitting idle decisions for the same agent.
func NewIdleTracker(maxIdleLogs int) *IdleTracker {
	return &IdleTracker{
		lastProduct: make(map[string]time.Time),
		idleCycles:  make(map[string]int),
		maxIdleLogs: maxIdleLogs,
	}
}

// RecordAction marks an action as productive (resets idle counter).
func (t *IdleTracker) RecordAction(agent string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastProduct[agent] = time.Now()
	t.idleCycles[agent] = 0
}

// RecordIdle increments the idle counter, returning whether this cycle should be logged.
func (t *IdleTracker) RecordIdle(agent string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.idleCycles[agent]++
	return t.idleCycles[agent] <= t.maxIdleLogs
}

// LastAction returns the last productive action time for an agent.
func (t *IdleTracker) LastAction(agent string) time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastProduct[agent]
}

// Claimer is the subset of task routing the default handler needs.
type Claimer interface {
	ClaimPendingForAgent(ctx context.Context, agentName string) (taskID string, err error)
}

// DefaultHandler composes an Observer, Claimer, IdleTracker, and event bus into a WakeUpHandler.
// Story 4.3: observe state. Story 4.4: claim pending tasks. Story 4.5: suppress busywork.
// Story 4.6: emit structured decision events.
type DefaultHandler struct {
	obs     *Observer
	claimer Claimer
	tracker *IdleTracker
	bus     *event.Bus
}

// NewDefaultHandler builds a wake-up handler that exercises the full cycle.
func NewDefaultHandler(obs *Observer, claimer Claimer, tracker *IdleTracker, bus *event.Bus) *DefaultHandler {
	return &DefaultHandler{obs: obs, claimer: claimer, tracker: tracker, bus: bus}
}

// Handle runs one wake-up cycle.
func (h *DefaultHandler) Handle(ctx context.Context, agentName string) error {
	snap, err := h.obs.Snapshot(ctx, agentName)
	if err != nil {
		return fmt.Errorf("observe %s: %w", agentName, err)
	}

	d := Decision{
		AgentName: agentName,
		State:     stateFromSnapshot(snap),
		Timestamp: time.Now(),
	}

	switch {
	case snap.RunningByAgent > 0 || snap.AssignedToAgent > 0:
		d.Action = "noop"
		d.Reason = "agent already has work in flight"
		h.tracker.RecordAction(agentName)
	case snap.PendingTasks > 0 && h.claimer != nil:
		taskID, err := h.claimer.ClaimPendingForAgent(ctx, agentName)
		if err != nil {
			d.Action = "escalate"
			d.Reason = err.Error()
		} else if taskID != "" {
			d.Action = "claim"
			d.TaskID = taskID
			d.Reason = "claimed pending task"
			h.tracker.RecordAction(agentName)
		} else {
			d.Action = "idle"
			d.Reason = "no capable task matches this agent"
		}
	default:
		d.Action = "idle"
		d.Reason = "no pending work"
	}

	// Idle busywork prevention: stop emitting once tracker says we've logged enough.
	if d.Action == "idle" && !h.tracker.RecordIdle(agentName) {
		slog.Debug("wake-up idle suppressed", "agent", agentName)
		return nil
	}

	slog.Info("wake-up cycle",
		"agent", agentName,
		"state", d.State,
		"action", d.Action,
		"task", d.TaskID,
		"pending", snap.PendingTasks,
	)

	if h.bus != nil {
		_, _ = h.bus.Publish(ctx, WakeUpEventType, agentName, d)
		if d.Action == "idle" {
			_, _ = h.bus.Publish(ctx, event.AgentIdle, agentName, map[string]string{
				"reason": d.Reason,
			})
		}
	}
	return nil
}

func stateFromSnapshot(s Observation) string {
	switch {
	case s.RunningByAgent > 0:
		return "executing"
	case s.AssignedToAgent > 0:
		return "starting"
	case s.PendingTasks > 0:
		return "available"
	default:
		return "idle"
	}
}
