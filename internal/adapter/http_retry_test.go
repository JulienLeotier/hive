package adapter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHTTPAdapterRetriesOnFailure verifies Story 5.5 AC: "the system retries
// with configured backoff" and "each retry emits a task.retry event".
func TestHTTPAdapterRetriesOnFailure(t *testing.T) {
	calls := int32(0)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			http.Error(w, "transient", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"task_id":"t","status":"completed"}`))
	}))
	defer srv.Close()

	attempts := int32(0)
	a := NewHTTPAdapter(srv.URL).WithRetry(&RetryPolicy{
		MaxAttempts: 5,
		InitialWait: 5 * time.Millisecond,
		MaxWait:     50 * time.Millisecond,
		Multiplier:  2.0,
		Jitter:      0,
		OnAttempt: func(attempt int, wait time.Duration, lastErr error) {
			atomic.AddInt32(&attempts, 1)
		},
	})

	res, err := a.Invoke(context.Background(), Task{ID: "t", Type: "x", Input: nil})
	require.NoError(t, err)
	assert.Equal(t, "completed", res.Status)
	assert.GreaterOrEqual(t, atomic.LoadInt32(&attempts), int32(2), "OnAttempt must fire for every retry")
}

func TestHTTPAdapterGivesUpAfterMaxAttempts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusInternalServerError)
	}))
	defer srv.Close()

	a := NewHTTPAdapter(srv.URL).WithRetry(&RetryPolicy{
		MaxAttempts: 3, InitialWait: time.Millisecond, MaxWait: time.Millisecond, Multiplier: 2,
	})
	_, err := a.Invoke(context.Background(), Task{ID: "t", Type: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "after 3 attempts")
}
