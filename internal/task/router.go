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

// CapacityLimit caps the number of simultaneously assigned/running tasks per
// agent before routing considers it saturated (Story 2.3 AC: "healthy, not at
// capacity"). Default matches the common desktop-concurrency ceiling.
var CapacityLimit = 10

// LocalNodeID identifies the local node. When set, FindCapableAgent prefers
// agents whose agents.node_id matches. Story 22.3.
var LocalNodeID = ""

// RoutingMode controls node affinity: "local-first" or "best-fit". Story 22.3.
var RoutingMode = "local-first"

// FindCapableAgent returns the ID and name of a healthy agent capable of handling the given task type.
// Returns empty strings if no capable agent is available.
func (r *Router) FindCapableAgent(ctx context.Context, taskType string) (agentID, agentName string, err error) {
	// Story 22.3: when local-first routing is configured and we know the local
	// node id, order agents so local candidates come first. Empty node_id
	// agents (single-node deployments) are treated as local.
	orderBy := "ORDER BY name"
	if RoutingMode == "local-first" && LocalNodeID != "" {
		orderBy = fmt.Sprintf("ORDER BY CASE WHEN node_id = %q OR node_id = '' THEN 0 ELSE 1 END, name", LocalNodeID)
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, capabilities FROM agents WHERE health_status = 'healthy' `+orderBy,
	)
	if err != nil {
		return "", "", fmt.Errorf("querying agents: %w", err)
	}
	defer rows.Close()

	considered := 0
	type candidate struct{ id, name, capsJSON string }
	var candidates []candidate
	for rows.Next() {
		var id, name, capsJSON string
		if err := rows.Scan(&id, &name, &capsJSON); err != nil {
			continue
		}
		candidates = append(candidates, candidate{id, name, capsJSON})
	}

	for _, c := range candidates {
		considered++

		var caps adapter.AgentCapabilities
		if err := json.Unmarshal([]byte(c.capsJSON), &caps); err != nil {
			continue
		}

		for _, tt := range caps.TaskTypes {
			if tt == taskType {
				// Capacity check: how many in-flight tasks does this agent hold?
				var inFlight int
				_ = r.db.QueryRowContext(ctx,
					`SELECT COUNT(*) FROM tasks WHERE agent_id = ? AND status IN ('assigned','running')`,
					c.id).Scan(&inFlight)
				if inFlight >= CapacityLimit {
					slog.Debug("agent at capacity; skipping", "agent", c.name, "in_flight", inFlight)
					continue
				}
				slog.Debug("routed task to agent", "task_type", taskType, "agent", c.name, "in_flight", inFlight)
				r.emitRouteDecision(ctx, taskType, c.name, "capability_match", considered)
				return c.id, c.name, nil
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

	// Story 4.4 AC: "if the task was already claimed by another agent, the
	// agent tries the next one". Fetch up to 16 candidates so a lost race
	// doesn't starve the wake-up cycle.
	query := fmt.Sprintf(
		`SELECT id FROM tasks WHERE status = 'pending' AND type IN (%s)
		 ORDER BY created_at LIMIT 16`, string(placeholders))

	candidateRows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return "", fmt.Errorf("finding claimable task: %w", err)
	}
	var candidateIDs []string
	for candidateRows.Next() {
		var id string
		if err := candidateRows.Scan(&id); err == nil {
			candidateIDs = append(candidateIDs, id)
		}
	}
	candidateRows.Close()

	if len(candidateIDs) == 0 {
		return "", nil
	}

	var taskID string
	for _, id := range candidateIDs {
		res, err := r.db.ExecContext(ctx,
			`UPDATE tasks SET status = 'assigned', agent_id = ?
			 WHERE id = ? AND status = 'pending'`, agentID, id,
		)
		if err != nil {
			return "", fmt.Errorf("claiming task: %w", err)
		}
		n, _ := res.RowsAffected()
		if n > 0 {
			taskID = id
			break
		}
		// Lost the race on this one — try the next.
	}
	if taskID == "" {
		return "", nil
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
