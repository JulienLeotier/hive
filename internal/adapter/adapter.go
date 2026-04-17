package adapter

import "context"

// AgentCapabilities describes what an agent can do.
type AgentCapabilities struct {
	Name       string   `json:"name"`
	TaskTypes  []string `json:"task_types"`
	CostPerRun float64  `json:"cost_per_run,omitempty"`
	// Version is advertised by the adapter so the hive can run multiple
	// versions of the same named agent side-by-side (canary / A-B). Empty
	// defaults to "1.0.0" at registration time.
	Version string `json:"version,omitempty"`
}

// Task represents work to be sent to an agent.
type Task struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Input any    `json:"input"`
}

// TaskResult is the response from an agent after processing a task.
type TaskResult struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"` // "completed" or "failed"
	Output any    `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

// HealthStatus reports agent health.
type HealthStatus struct {
	Status  string `json:"status"` // "healthy", "degraded", "unavailable"
	Message string `json:"message,omitempty"`
}

// Checkpoint is a serializable snapshot of agent state.
type Checkpoint struct {
	Data any `json:"data"`
}

// Adapter is the interface every agent adapter must implement.
// This is the Agent Adapter Protocol — the core interop layer.
type Adapter interface {
	// Declare returns the agent's capabilities.
	Declare(ctx context.Context) (AgentCapabilities, error)

	// Invoke sends a task to the agent and returns the result.
	Invoke(ctx context.Context, task Task) (TaskResult, error)

	// Health returns the agent's current health status.
	Health(ctx context.Context) (HealthStatus, error)

	// Checkpoint returns the agent's current state for persistence.
	Checkpoint(ctx context.Context) (Checkpoint, error)

	// Resume restores the agent from a previously saved checkpoint.
	Resume(ctx context.Context, cp Checkpoint) error
}
