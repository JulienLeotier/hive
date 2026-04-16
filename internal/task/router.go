package task

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/JulienLeotier/hive/internal/adapter"
	"github.com/JulienLeotier/hive/internal/event"
)

// FederationResolver is consulted when no local agent can satisfy a task.
// Returning a non-empty hiveName means the task is routable to a federated
// peer; the router then emits task.federated for the caller to proxy.
type FederationResolver func(ctx context.Context, taskType string) (hiveName, hiveURL string, ok bool)

// Router matches tasks to capable agents based on declared capabilities.
type Router struct {
	db         *sql.DB
	bus        *event.Bus
	federation FederationResolver
}

// WithFederation installs a cross-hive fallback resolver (Story 19.3).
func (r *Router) WithFederation(fr FederationResolver) *Router {
	r.federation = fr
	return r
}

// NewRouter creates a task router.
func NewRouter(db *sql.DB) *Router {
	return &Router{db: db}
}

// WithBus attaches an event bus so reassignments can be announced.
func (r *Router) WithBus(bus *event.Bus) *Router {
	r.bus = bus
	return r
}

// FindCapableAgent returns the ID and name of a healthy agent capable of handling the given task type.
// Returns empty strings if no capable agent is available.
func (r *Router) FindCapableAgent(ctx context.Context, taskType string) (agentID, agentName string, err error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, capabilities FROM agents WHERE health_status = 'healthy' ORDER BY name`,
	)
	if err != nil {
		return "", "", fmt.Errorf("querying agents: %w", err)
	}
	defer rows.Close()

	considered := 0
	for rows.Next() {
		var id, name, capsJSON string
		if err := rows.Scan(&id, &name, &capsJSON); err != nil {
			continue
		}
		considered++

		var caps adapter.AgentCapabilities
		if err := json.Unmarshal([]byte(capsJSON), &caps); err != nil {
			continue
		}

		for _, tt := range caps.TaskTypes {
			if tt == taskType {
				slog.Debug("routed task to agent", "task_type", taskType, "agent", name)
				r.emitRouteDecision(ctx, taskType, name, "capability_match", considered)
				return id, name, nil
			}
		}
	}

	// Story 19.3: before giving up, ask the federation resolver for a peer.
	if r.federation != nil {
		if hiveName, hiveURL, ok := r.federation(ctx, taskType); ok {
			if r.bus != nil {
				_, _ = r.bus.Publish(ctx, event.TaskFederated, "router", map[string]string{
					"task_type": taskType,
					"hive_name": hiveName,
					"hive_url":  hiveURL,
				})
			}
			r.emitRouteDecision(ctx, taskType, "federation:"+hiveName, "federated", considered)
			return "federation:" + hiveName, hiveName, nil
		}
	}

	r.emitRouteDecision(ctx, taskType, "", "no_capable_agent", considered)
	if r.bus != nil {
		_, _ = r.bus.Publish(ctx, event.TaskUnroutable, "router", map[string]string{
			"task_type": taskType,
			"reason":    "no capable healthy agent",
		})
	}
	return "", "", nil // no capable agent found
}

func (r *Router) emitRouteDecision(ctx context.Context, taskType, chosen, reason string, considered int) {
	if r.bus == nil {
		return
	}
	_, _ = r.bus.Publish(ctx, event.DecisionTaskRouted, "router", event.Decision{
		Action:  "route_task",
		Subject: taskType,
		Reason:  reason,
		Context: map[string]any{
			"candidates_considered": considered,
			"chosen":                chosen,
		},
	})
}

// ClaimPendingForAgent atomically assigns one pending task matching the agent's
// declared capabilities. Returns empty taskID if nothing is claimable.
func (r *Router) ClaimPendingForAgent(ctx context.Context, agentName string) (string, error) {
	var agentID, capsJSON string
	err := r.db.QueryRowContext(ctx,
		`SELECT id, capabilities FROM agents WHERE name = ? AND health_status = 'healthy'`,
		agentName,
	).Scan(&agentID, &capsJSON)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("loading agent %s: %w", agentName, err)
	}

	var caps adapter.AgentCapabilities
	if err := json.Unmarshal([]byte(capsJSON), &caps); err != nil {
		return "", fmt.Errorf("parsing capabilities: %w", err)
	}
	if len(caps.TaskTypes) == 0 {
		return "", nil
	}

	// Build IN clause for capable task types
	placeholders := make([]byte, 0, len(caps.TaskTypes)*2)
	args := make([]any, 0, len(caps.TaskTypes)+1)
	for i, tt := range caps.TaskTypes {
		if i > 0 {
			placeholders = append(placeholders, ',')
		}
		placeholders = append(placeholders, '?')
		args = append(args, tt)
	}

	query := fmt.Sprintf(
		`SELECT id FROM tasks WHERE status = 'pending' AND type IN (%s)
		 ORDER BY created_at LIMIT 1`, string(placeholders))

	var taskID string
	err = r.db.QueryRowContext(ctx, query, args...).Scan(&taskID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("finding claimable task: %w", err)
	}

	// Atomic claim — only succeeds if task is still pending
	res, err := r.db.ExecContext(ctx,
		`UPDATE tasks SET status = 'assigned', agent_id = ?
		 WHERE id = ? AND status = 'pending'`, agentID, taskID,
	)
	if err != nil {
		return "", fmt.Errorf("claiming task: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return "", nil // lost the race
	}

	if r.bus != nil {
		_, _ = r.bus.Publish(ctx, event.TaskSelfAssigned, agentName, map[string]string{
			"task_id": taskID, "agent": agentName,
		})
		// Also emit task.assigned so generic subscribers don't need to track both.
		_, _ = r.bus.Publish(ctx, event.TaskAssigned, agentName, map[string]string{
			"task_id": taskID, "agent": agentName, "via": "self-claim",
		})
	}

	slog.Info("agent self-claimed task", "agent", agentName, "task_id", taskID)
	return taskID, nil
}

// Reassign detaches a task from its current agent and sets it back to pending.
// Emits task.failover so the watcher can announce the handoff.
func (r *Router) Reassign(ctx context.Context, taskID, reason string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE tasks SET status = 'pending', agent_id = ''
		 WHERE id = ? AND status IN ('assigned', 'running')`, taskID,
	)
	if err != nil {
		return fmt.Errorf("reassigning task %s: %w", taskID, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("task %s not in a reassignable state", taskID)
	}

	if r.bus != nil {
		_, _ = r.bus.Publish(ctx, event.TaskFailover, "system", map[string]string{
			"task_id": taskID, "reason": reason,
		})
	}
	slog.Warn("task reassigned", "task_id", taskID, "reason", reason)
	return nil
}

// ReassignAgentTasks reassigns every assigned/running task attached to an agent.
// Returns the number of tasks that were moved back to pending.
func (r *Router) ReassignAgentTasks(ctx context.Context, agentName, reason string) (int, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT t.id FROM tasks t JOIN agents a ON a.id = t.agent_id
		 WHERE a.name = ? AND t.status IN ('assigned', 'running')`, agentName,
	)
	if err != nil {
		return 0, fmt.Errorf("listing agent tasks: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}

	count := 0
	for _, id := range ids {
		if err := r.Reassign(ctx, id, reason); err == nil {
			count++
		}
	}
	return count, nil
}
