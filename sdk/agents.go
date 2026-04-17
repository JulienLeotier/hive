package sdk

import (
	"context"
	"net/url"
	"time"
)

// Agent is the public shape of a registered Hive agent. Matches the JSON
// returned by /api/v1/agents — SDK-side types are deliberately distinct
// from internal/agent.Agent so this package stays importable from any
// external project without pulling in the hive server.
type Agent struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	Version      string    `json:"version,omitempty"`
	Config       string    `json:"config,omitempty"`
	Capabilities string    `json:"capabilities,omitempty"`
	HealthStatus string    `json:"health_status"`
	TrustLevel   string    `json:"trust_level,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	UpdatedAt    time.Time `json:"updated_at,omitempty"`
}

// TaskResult mirrors the adapter protocol's response shape, surfaced by
// /api/v1/agents/{name}/invoke for the playground flow.
type TaskResult struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
	Output any    `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

// AgentsClient groups agent-scoped operations.
type AgentsClient struct{ c *Client }

// List returns every agent visible to the caller's tenant.
func (a *AgentsClient) List(ctx context.Context) ([]Agent, error) {
	return do[[]Agent](ctx, a.c, "GET", "/api/v1/agents", nil)
}

// RegisterOpts is the body shape for POST /agents. url is the adapter's
// HTTP endpoint; Hive health-checks it and calls /declare before persisting.
type RegisterOpts struct {
	Name string `json:"name"`
	Type string `json:"type"`
	URL  string `json:"url"`
}

// Register creates a new agent. The API health-checks the adapter before
// persisting, so a 502 here means the adapter itself was unreachable.
func (a *AgentsClient) Register(ctx context.Context, opts RegisterOpts) (*Agent, error) {
	agent, err := do[Agent](ctx, a.c, "POST", "/api/v1/agents", opts)
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

// Delete removes an agent by name. In-flight tasks are requeued server-side.
func (a *AgentsClient) Delete(ctx context.Context, name string) error {
	_, err := do[map[string]string](ctx, a.c, "DELETE",
		"/api/v1/agents/"+url.PathEscape(name), nil)
	return err
}

// InvokeOpts carries the ad-hoc task body for the playground endpoint.
type InvokeOpts struct {
	Type  string `json:"type"`
	Input any    `json:"input"`
}

// Invoke sends an ad-hoc task to the named agent through its adapter. Used
// for connectivity probes and one-off calls that don't warrant a workflow.
func (a *AgentsClient) Invoke(ctx context.Context, name string, opts InvokeOpts) (*TaskResult, error) {
	res, err := do[TaskResult](ctx, a.c, "POST",
		"/api/v1/agents/"+url.PathEscape(name)+"/invoke", opts)
	if err != nil {
		return nil, err
	}
	return &res, nil
}
