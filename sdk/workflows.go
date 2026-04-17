package sdk

import (
	"context"
	"net/url"
	"time"
)

// Workflow is the public shape of a workflow record.
type Workflow struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// WorkflowsClient groups workflow-scoped operations.
type WorkflowsClient struct{ c *Client }

// List returns registered workflow runs.
func (w *WorkflowsClient) List(ctx context.Context) ([]Workflow, error) {
	return do[[]Workflow](ctx, w.c, "GET", "/api/v1/workflows", nil)
}

// FireResult is the response from a manual workflow fire.
type FireResult struct {
	Status   string `json:"status"`
	Workflow string `json:"workflow"`
}

// Fire triggers a registered workflow by name with an optional JSON payload
// forwarded as the first task's input. The API returns 202 immediately — the
// run completes asynchronously; poll List or subscribe to /ws for updates.
func (w *WorkflowsClient) Fire(ctx context.Context, name string, payload any) (*FireResult, error) {
	res, err := do[FireResult](ctx, w.c, "POST",
		"/api/v1/workflows/"+url.PathEscape(name)+"/runs", payload)
	if err != nil {
		return nil, err
	}
	return &res, nil
}
