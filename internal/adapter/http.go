package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPAdapter communicates with agents over HTTP/JSON.
type HTTPAdapter struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewHTTPAdapter creates an adapter for an HTTP-based agent.
func NewHTTPAdapter(baseURL string) *HTTPAdapter {
	return &HTTPAdapter{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (a *HTTPAdapter) Declare(ctx context.Context) (AgentCapabilities, error) {
	var caps AgentCapabilities
	if err := a.get(ctx, "/declare", &caps); err != nil {
		return caps, fmt.Errorf("declare: %w", err)
	}
	return caps, nil
}

func (a *HTTPAdapter) Invoke(ctx context.Context, task Task) (TaskResult, error) {
	var result TaskResult
	if err := a.post(ctx, "/invoke", task, &result); err != nil {
		return result, fmt.Errorf("invoke task %s: %w", task.ID, err)
	}
	return result, nil
}

func (a *HTTPAdapter) Health(ctx context.Context) (HealthStatus, error) {
	var status HealthStatus
	if err := a.get(ctx, "/health", &status); err != nil {
		return HealthStatus{Status: "unavailable", Message: err.Error()}, nil
	}
	return status, nil
}

func (a *HTTPAdapter) Checkpoint(ctx context.Context) (Checkpoint, error) {
	var cp Checkpoint
	if err := a.get(ctx, "/checkpoint", &cp); err != nil {
		return cp, fmt.Errorf("checkpoint: %w", err)
	}
	return cp, nil
}

func (a *HTTPAdapter) Resume(ctx context.Context, cp Checkpoint) error {
	if err := a.post(ctx, "/resume", cp, nil); err != nil {
		return fmt.Errorf("resume: %w", err)
	}
	return nil
}

func (a *HTTPAdapter) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.BaseURL+path, nil)
	if err != nil {
		return err
	}
	return a.do(req, out)
}

func (a *HTTPAdapter) post(ctx context.Context, path string, body any, out any) error {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.BaseURL+path, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return a.do(req, out)
}

func (a *HTTPAdapter) do(req *http.Request, out any) error {
	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}
