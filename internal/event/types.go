package event

import "time"

// Event represents a system event persisted to the event log.
type Event struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	Source    string    `json:"source"`
	Payload   string    `json:"payload"`
	CreatedAt time.Time `json:"created_at"`
}

// Common event types using dot notation.
const (
	AgentRegistered = "agent.registered"
	AgentRemoved    = "agent.removed"
	AgentHealthUp   = "agent.health.up"
	AgentHealthDown = "agent.health.down"
	AgentIsolated   = "agent.isolated"
	AgentCircuitOpen = "agent.circuit_open"

	TaskCreated    = "task.created"
	TaskAssigned   = "task.assigned"
	TaskStarted    = "task.started"
	TaskCompleted  = "task.completed"
	TaskFailed     = "task.failed"
	TaskRetry      = "task.retry"
	TaskFailover   = "task.failover"

	WorkflowStarted   = "workflow.started"
	WorkflowCompleted = "workflow.completed"
	WorkflowFailed    = "workflow.failed"
)

// Subscriber is a callback invoked when a matching event is published.
type Subscriber func(Event)
