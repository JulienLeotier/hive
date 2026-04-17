package sdk

import (
	"context"
	"net/url"
)

// Webhook is the public shape of a webhook configuration.
type Webhook struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Type        string `json:"type"`
	EventFilter string `json:"event_filter,omitempty"`
	Enabled     bool   `json:"enabled"`
}

// WebhooksClient groups webhook-scoped operations.
type WebhooksClient struct{ c *Client }

// List returns every configured outbound webhook.
func (w *WebhooksClient) List(ctx context.Context) ([]Webhook, error) {
	return do[[]Webhook](ctx, w.c, "GET", "/api/v1/webhooks", nil)
}

// AddOpts is the body shape for POST /webhooks. Type is one of "generic",
// "slack", or "github". EventFilter is a JSON array or comma-separated list
// of event types to deliver (empty = all).
type AddOpts struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Type        string `json:"type"`
	EventFilter string `json:"event_filter,omitempty"`
}

// Add registers a new webhook. The server rejects URLs pointing at
// private/loopback addresses (SSRF guard).
func (w *WebhooksClient) Add(ctx context.Context, opts AddOpts) (*Webhook, error) {
	wh, err := do[Webhook](ctx, w.c, "POST", "/api/v1/webhooks", opts)
	if err != nil {
		return nil, err
	}
	return &wh, nil
}

// Delete removes a webhook by name.
func (w *WebhooksClient) Delete(ctx context.Context, name string) error {
	_, err := do[map[string]string](ctx, w.c, "DELETE",
		"/api/v1/webhooks/"+url.PathEscape(name), nil)
	return err
}
