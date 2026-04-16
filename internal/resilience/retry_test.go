package resilience

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetryPolicySucceedsFirstTry(t *testing.T) {
	p := DefaultRetryPolicy()
	calls := 0
	err := p.Do(context.Background(), func(ctx context.Context, attempt int) error {
		calls++
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
}

func TestRetryPolicySucceedsAfterFailures(t *testing.T) {
	p := RetryPolicy{MaxAttempts: 4, InitialWait: time.Millisecond, Multiplier: 2, MaxWait: 5 * time.Millisecond}
	calls := 0
	err := p.Do(context.Background(), func(ctx context.Context, attempt int) error {
		calls++
		if attempt < 3 {
			return errors.New("transient")
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 3, calls)
}

func TestRetryPolicyExhausts(t *testing.T) {
	p := RetryPolicy{MaxAttempts: 3, InitialWait: time.Millisecond, Multiplier: 2}
	calls := 0
	err := p.Do(context.Background(), func(ctx context.Context, attempt int) error {
		calls++
		return errors.New("down")
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "after 3 attempts")
	assert.Equal(t, 3, calls)
}

func TestRetryPolicyNonRetryable(t *testing.T) {
	p := RetryPolicy{MaxAttempts: 5, InitialWait: time.Millisecond}
	calls := 0
	err := p.Do(context.Background(), func(ctx context.Context, attempt int) error {
		calls++
		return NonRetryable(errors.New("fatal"))
	})
	require.Error(t, err)
	assert.Equal(t, "fatal", err.Error())
	assert.Equal(t, 1, calls)
}

func TestRetryPolicyRespectsCancel(t *testing.T) {
	p := RetryPolicy{MaxAttempts: 5, InitialWait: 100 * time.Millisecond, Multiplier: 2}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel
	err := p.Do(ctx, func(ctx context.Context, attempt int) error {
		return errors.New("transient")
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "retry aborted")
}
