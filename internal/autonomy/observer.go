package autonomy

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/JulienLeotier/hive/internal/event"
)

// WakeUpEventType is emitted for every wake-up cycle decision.
const WakeUpEventType = "agent.wakeup"

// Wake-up cycle action values assigned to Decision.Action. Stringly-typed
// because they travel over the event bus as part of the decision payload,
// but centralising them here keeps typo drift in check.
const (
	ActionClaim    = "claim"
	ActionIdle     = "idle"
	ActionEscalate = "escalate"
	ActionNoop     = "noop"
)

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

// DetectBusywork reports agent names that appear to be generating tasks without
// an upstream trigger or backlog source (Story 4.5 AC). Heuristic: an agent
// created N > threshold tasks in the window where each task has no depends_on,
// no workflow_id matching a real workflow row, and the agent itself is the
// source.
//
// Returns the map of agent_name → offending task count; callers can emit
// agent.busywork events or page a human.
func (o *Observer) DetectBusywork(ctx context.Context, window time.Duration, threshold int) (map[string]int, error) {
	cutoff := time.Now().Add(-window).UTC().Format("2006-01-02 15:04:05")
	rows, err := o.db.QueryContext(ctx, `
		SELECT a.name, COUNT(*) AS task_count
		FROM tasks t
		JOIN agents a ON a.id = t.agent_id
		WHERE t.created_at >= ?
		  AND (t.depends_on IS NULL OR t.depends_on = '' OR t.depends_on = '[]')
		  AND t.workflow_id NOT IN (SELECT id FROM workflows)
		GROUP BY a.name
		HAVING task_count >= ?`, cutoff, threshold)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	offenders := map[string]int{}
	for rows.Next() {
		var name string
		var n int
		if err := rows.Scan(&name, &n); err == nil {
			offenders[name] = n
		}
	}
	return offenders, rows.Err()
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
	obs      *Observer
	claimer  Claimer
	tracker  *IdleTracker
	bus      *event.Bus
	planPath string // Story 4.1: when set, re-read PLAN.yaml on every wake-up.

	planMu   sync.Mutex
	planInfo os.FileInfo
	plan     *Plan
}

// NewDefaultHandler builds a wake-up handler that exercises the full cycle.
func NewDefaultHandler(obs *Observer, claimer Claimer, tracker *IdleTracker, bus *event.Bus) *DefaultHandler {
	return &DefaultHandler{obs: obs, claimer: claimer, tracker: tracker, bus: bus}
}

// WithPlan enables plan hot-reload (Story 4.1 AC: "changes take effect at next
// wake-up"). The YAML file is re-stat'd on every cycle and re-parsed when the
// mtime changes; parse errors are logged but don't block the rest of the cycle.
func (h *DefaultHandler) WithPlan(path string) *DefaultHandler {
	h.planPath = path
	return h
}

// CurrentPlan returns the most recently successfully-loaded plan (nil until
// WithPlan has been called and at least one Handle() has succeeded).
func (h *DefaultHandler) CurrentPlan() *Plan {
	h.planMu.Lock()
	defer h.planMu.Unlock()
	return h.plan
}

// reloadPlanIfChanged compares the file mtime with what we last saw; when it
// differs we re-parse. Called inside Handle before the observation.
func (h *DefaultHandler) reloadPlanIfChanged() {
	if h.planPath == "" {
		return
	}
	info, err := os.Stat(h.planPath)
	if err != nil {
		slog.Warn("plan hot-reload stat failed", "path", h.planPath, "error", err)
		return
	}
	h.planMu.Lock()
	defer h.planMu.Unlock()
	if h.planInfo != nil && info.ModTime().Equal(h.planInfo.ModTime()) && info.Size() == h.planInfo.Size() {
		return
	}
	plan, err := ParsePlan(h.planPath)
	if err != nil {
		slog.Warn("plan hot-reload parse failed", "path", h.planPath, "error", err)
		return
	}
	h.plan = plan
	h.planInfo = info
	slog.Info("plan reloaded", "path", h.planPath, "initial_state", plan.InitialState)
}

// Handle runs one wake-up cycle.
func (h *DefaultHandler) Handle(ctx context.Context, agentName string) error {
	h.reloadPlanIfChanged()

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
		d.Action = ActionNoop
		d.Reason = "agent already has work in flight"
		h.tracker.RecordAction(agentName)
	case snap.PendingTasks > 0 && h.claimer != nil:
		taskID, err := h.claimer.ClaimPendingForAgent(ctx, agentName)
		if err != nil {
			d.Action = ActionEscalate
			d.Reason = err.Error()
		} else if taskID != "" {
			d.Action = ActionClaim
			d.TaskID = taskID
			d.Reason = "claimed pending task"
			h.tracker.RecordAction(agentName)
		} else {
			d.Action = ActionIdle
			d.Reason = "no capable task matches this agent"
		}
	default:
		d.Action = ActionIdle
		d.Reason = "no pending work"
	}

	// Idle busywork prevention: stop emitting once tracker says we've logged enough.
	if d.Action == ActionIdle && !h.tracker.RecordIdle(agentName) {
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
		if d.Action == ActionIdle {
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
		return ActionIdle
	}
}
