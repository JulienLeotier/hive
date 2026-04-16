package resilience

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"time"
)

// RetryPolicy describes an exponential backoff retry strategy.
type RetryPolicy struct {
	MaxAttempts int           // including the first try
	InitialWait time.Duration
	MaxWait     time.Duration
	Multiplier  float64 // backoff multiplier, typically 2.0
	Jitter      float64 // 0.0 to 1.0 — fraction of wait randomized
}

// DefaultRetryPolicy returns a safe baseline: 3 attempts, 200ms → 2s, 2x multiplier, 20% jitter.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: 3,
		InitialWait: 200 * time.Millisecond,
		MaxWait:     2 * time.Second,
		Multiplier:  2.0,
		Jitter:      0.2,
	}
}

// ErrNonRetryable wraps an error to opt out of retry at the call site.
type ErrNonRetryable struct{ Err error }

func (e ErrNonRetryable) Error() string { return e.Err.Error() }
func (e ErrNonRetryable) Unwrap() error { return e.Err }

// NonRetryable marks an error so Do will return immediately without retrying.
func NonRetryable(err error) error { return ErrNonRetryable{Err: err} }

// Do runs fn with exponential backoff retries. ctx cancellation aborts.
// The last error is returned wrapped with attempt count.
func (p RetryPolicy) Do(ctx context.Context, fn func(ctx context.Context, attempt int) error) error {
	attempts := p.MaxAttempts
	if attempts < 1 {
		attempts = 1
	}

	var lastErr error
	wait := p.InitialWait
	for i := 1; i <= attempts; i++ {
		err := fn(ctx, i)
		if err == nil {
			return nil
		}
		var nonRetry ErrNonRetryable
		if errors.As(err, &nonRetry) {
			return nonRetry.Err
		}
		lastErr = err

		if i == attempts {
			break
		}
		sleep := p.withJitter(wait)
		slog.Debug("retrying", "attempt", i, "wait", sleep, "error", err)
		select {
		case <-time.After(sleep):
		case <-ctx.Done():
			return fmt.Errorf("retry aborted after %d attempts: %w", i, ctx.Err())
		}

		// next wait
		next := time.Duration(float64(wait) * p.Multiplier)
		if p.MaxWait > 0 && next > p.MaxWait {
			next = p.MaxWait
		}
		wait = next
	}

	return fmt.Errorf("after %d attempts: %w", attempts, lastErr)
}

func (p RetryPolicy) withJitter(d time.Duration) time.Duration {
	if p.Jitter <= 0 {
		return d
	}
	j := p.Jitter
	if j > 1 {
		j = 1
	}
	// ±j fraction
	delta := (rand.Float64()*2 - 1) * j
	scaled := float64(d) * (1 + delta)
	if scaled < 0 {
		scaled = 0
	}
	return time.Duration(math.Round(scaled))
}
