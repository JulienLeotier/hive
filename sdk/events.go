package sdk

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// Event is the public shape of a Hive event row.
type Event struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	Source    string    `json:"source"`
	Payload   string    `json:"payload,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// EventsClient groups event-scoped operations.
type EventsClient struct{ c *Client }

// QueryOpts narrows an event query.
type QueryOpts struct {
	Type   string // event type filter (e.g. "task.failed"), partial match
	Source string // source filter
	Since  time.Time
	Limit  int
}

// List returns events matching opts, newest first.
func (e *EventsClient) List(ctx context.Context, opts QueryOpts) ([]Event, error) {
	q := url.Values{}
	if opts.Type != "" {
		q.Set("type", opts.Type)
	}
	if opts.Source != "" {
		q.Set("source", opts.Source)
	}
	if !opts.Since.IsZero() {
		q.Set("since", opts.Since.Format(time.RFC3339))
	}
	if opts.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", opts.Limit))
	}
	path := "/api/v1/events"
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	return do[[]Event](ctx, e.c, "GET", path, nil)
}

// EmitOpts is the body shape for POST /events.
type EmitOpts struct {
	Type    string `json:"type"`
	Source  string `json:"source,omitempty"` // defaults to the caller's identity
	Payload any    `json:"payload,omitempty"`
}

// Emit publishes a custom event to the bus. Useful for adapters pushing
// progress updates back to Hive without going through the task protocol.
func (e *EventsClient) Emit(ctx context.Context, opts EmitOpts) error {
	_, err := do[map[string]any](ctx, e.c, "POST", "/api/v1/events", opts)
	return err
}
