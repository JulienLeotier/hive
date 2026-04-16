package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"
)

// rand2 returns a pseudo-random float in [0,1). A package-private helper keeps
// the retry-jitter code self-contained.
func rand2() float64 { return rand.Float64() }

// HTTPAdapter communicates with agents over HTTP/JSON.
type HTTPAdapter struct {
	BaseURL    string
	HTTPClient *http.Client

	// Retry wraps Invoke so transient 5xx / network errors get exponential
	// backoff + jitter. Story 5.5. nil = no retries.
	Retry *RetryPolicy
}

// RetryPolicy is the adapter-facing view of resilience.RetryPolicy. Kept
// inline so the adapter package doesn't import resilience (avoids a cycle
// if resilience ever needs an adapter).
type RetryPolicy struct {
	MaxAttempts int
	InitialWait time.Duration
	MaxWait     time.Duration
	Multiplier  float64
	Jitter      float64
	// OnAttempt is called before each retry so the engine can emit task.retry.
	OnAttempt func(attempt int, wait time.Duration, lastErr error)
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

// WithRetry installs a retry policy on Invoke.
func (a *HTTPAdapter) WithRetry(p *RetryPolicy) *HTTPAdapter {
	a.Retry = p
	return a
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
	call := func() error {
		result = TaskResult{}
		return a.post(ctx, "/invoke", task, &result)
	}

	if a.Retry == nil || a.Retry.MaxAttempts <= 1 {
		if err := call(); err != nil {
			return result, fmt.Errorf("invoke task %s: %w", task.ID, err)
		}
		return result, nil
	}

	// Story 5.5: retry with exponential backoff + jitter.
	wait := a.Retry.InitialWait
	var lastErr error
	for attempt := 1; attempt <= a.Retry.MaxAttempts; attempt++ {
		lastErr = call()
		if lastErr == nil {
			return result, nil
		}
		if attempt == a.Retry.MaxAttempts {
			break
		}
		if a.Retry.OnAttempt != nil {
			a.Retry.OnAttempt(attempt, wait, lastErr)
		}
		select {
		case <-time.After(a.Retry.jitterWait(wait)):
		case <-ctx.Done():
			return result, fmt.Errorf("invoke task %s aborted: %w", task.ID, ctx.Err())
		}
		next := time.Duration(float64(wait) * a.Retry.Multiplier)
		if a.Retry.MaxWait > 0 && next > a.Retry.MaxWait {
			next = a.Retry.MaxWait
		}
		wait = next
	}
	return result, fmt.Errorf("invoke task %s: after %d attempts: %w", task.ID, a.Retry.MaxAttempts, lastErr)
}

func (p *RetryPolicy) jitterWait(d time.Duration) time.Duration {
	if p.Jitter <= 0 {
		return d
	}
	j := p.Jitter
	if j > 1 {
		j = 1
	}
	// Use math/rand which is fine for jitter (not crypto).
	delta := (rand2() - 0.5) * 2 * j
	scaled := float64(d) * (1 + delta)
	if scaled < 0 {
		scaled = 0
	}
	return time.Duration(scaled)
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

	// Limit response body to 10MB to prevent OOM from malicious agents
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
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
