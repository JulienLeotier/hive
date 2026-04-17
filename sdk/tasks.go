package sdk

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// Task is the public shape of a Hive task row.
type Task struct {
	ID              string    `json:"id"`
	WorkflowID      string    `json:"workflow_id,omitempty"`
	Type            string    `json:"type"`
	Status          string    `json:"status"`
	AgentID         string    `json:"agent_id,omitempty"`
	AgentName       string    `json:"agent_name,omitempty"`
	Input           string    `json:"input,omitempty"`
	Output          string    `json:"output,omitempty"`
	ResultSummary   string    `json:"result_summary,omitempty"`
	DurationSeconds float64   `json:"duration_seconds,omitempty"`
	CreatedAt       time.Time `json:"created_at,omitempty"`
	UpdatedAt       time.Time `json:"updated_at,omitempty"`
}

// TasksClient groups task-scoped operations.
type TasksClient struct{ c *Client }

// ListOpts narrows a task list query. Zero value returns all tasks visible
// to the caller's tenant, paginated server-side.
type ListOpts struct {
	Status     string // "pending", "running", "completed", "failed", etc.
	WorkflowID string
	Limit      int
	Offset     int
}

// List fetches tasks matching opts.
func (t *TasksClient) List(ctx context.Context, opts ListOpts) ([]Task, error) {
	q := url.Values{}
	if opts.Status != "" {
		q.Set("status", opts.Status)
	}
	if opts.WorkflowID != "" {
		q.Set("workflow_id", opts.WorkflowID)
	}
	if opts.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", opts.Limit))
	}
	if opts.Offset > 0 {
		q.Set("offset", fmt.Sprintf("%d", opts.Offset))
	}
	path := "/api/v1/tasks"
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	return do[[]Task](ctx, t.c, "GET", path, nil)
}

// RetryResult is the body returned by the retry endpoint.
type RetryResult struct {
	NewTaskID      string `json:"new_task_id"`
	OriginalTaskID string `json:"original_task_id"`
}

// Retry re-queues a failed or completed task by creating a fresh pending
// row with the same type/input/workflow_id. Returns 409 if the task is
// still in flight — callers should wait or cancel first.
func (t *TasksClient) Retry(ctx context.Context, id string) (*RetryResult, error) {
	res, err := do[RetryResult](ctx, t.c, "POST",
		"/api/v1/tasks/"+url.PathEscape(id)+"/retry", nil)
	if err != nil {
		return nil, err
	}
	return &res, nil
}
