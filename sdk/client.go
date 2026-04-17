package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is the entry point to the SDK. One instance per (base URL, API key)
// pair — it's safe for concurrent use.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// NewClient creates a client. baseURL is the hive root (e.g.
// "https://hive.example.com" — no trailing slash), apiKey is a key previously
// minted via `hive api-key create` or the setup wizard. Pass "" for apiKey
// when hitting a dev hive that has no keys configured.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// WithHTTPClient lets callers override the underlying HTTP client — useful
// for plugging in OpenTelemetry instrumentation or a custom transport.
func (c *Client) WithHTTPClient(h *http.Client) *Client {
	c.http = h
	return c
}

// Agents returns the agent resource.
func (c *Client) Agents() *AgentsClient { return &AgentsClient{c: c} }

// Tasks returns the task resource.
func (c *Client) Tasks() *TasksClient { return &TasksClient{c: c} }

// Workflows returns the workflow resource.
func (c *Client) Workflows() *WorkflowsClient { return &WorkflowsClient{c: c} }

// Webhooks returns the webhook resource.
func (c *Client) Webhooks() *WebhooksClient { return &WebhooksClient{c: c} }

// Events returns the event resource.
func (c *Client) Events() *EventsClient { return &EventsClient{c: c} }

// APIError is the typed error returned when the API responds with an error
// envelope ({"error": {...}}) or a non-2xx status. Use errors.As to unwrap it.
type APIError struct {
	HTTPStatus int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

// Error implements error. "CODE: message (http N)" so grep in logs stays easy.
func (e *APIError) Error() string {
	return fmt.Sprintf("%s: %s (http %d)", e.Code, e.Message, e.HTTPStatus)
}

// envelope mirrors the Response struct the server writes — keeping it private
// here so SDK callers don't have to care about the wire shape.
type envelope[T any] struct {
	Data  T         `json:"data"`
	Error *APIError `json:"error"`
}

// do is the single HTTP choke-point. Every resource call ends up here so
// auth, error parsing, and body handling stay in one place.
func do[T any](ctx context.Context, c *Client, method, path string, body any) (T, error) {
	var zero T
	url := c.baseURL + path

	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return zero, fmt.Errorf("sdk: marshal request body: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return zero, fmt.Errorf("sdk: build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return zero, fmt.Errorf("sdk: transport: %w", err)
	}
	defer resp.Body.Close()

	// Read the whole body (capped) so JSON parsing has a single path, and
	// non-JSON error pages (e.g. 502 from an ingress) still surface as an
	// APIError with the raw text.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return zero, fmt.Errorf("sdk: read body: %w", err)
	}

	var env envelope[T]
	if len(raw) > 0 {
		if jsonErr := json.Unmarshal(raw, &env); jsonErr != nil {
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return zero, fmt.Errorf("sdk: decode response: %w (body: %s)", jsonErr, truncate(raw, 200))
			}
			// Non-2xx with non-JSON body — surface as APIError with the text.
			return zero, &APIError{
				HTTPStatus: resp.StatusCode,
				Code:       "HTTP_" + http.StatusText(resp.StatusCode),
				Message:    truncate(raw, 200),
			}
		}
	}

	if env.Error != nil {
		env.Error.HTTPStatus = resp.StatusCode
		return zero, env.Error
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return zero, &APIError{
			HTTPStatus: resp.StatusCode,
			Code:       "UNEXPECTED_STATUS",
			Message:    fmt.Sprintf("server returned %d with no error envelope", resp.StatusCode),
		}
	}
	return env.Data, nil
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n]) + "…"
	}
	return string(b)
}
