package adapter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// panickyAdapter is an adapter whose Invoke() always panics. Used to prove
// SafeInvoke converts the panic into a structured error instead of crashing
// the caller's goroutine.
type panickyAdapter struct{}

func (panickyAdapter) Declare(ctx context.Context) (AgentCapabilities, error) {
	return AgentCapabilities{}, nil
}

func (panickyAdapter) Invoke(ctx context.Context, t Task) (TaskResult, error) {
	panic("boom: nil map access in adapter")
}

func (panickyAdapter) Health(ctx context.Context) (HealthStatus, error) {
	return HealthStatus{Status: "healthy"}, nil
}

func (panickyAdapter) Checkpoint(ctx context.Context) (Checkpoint, error) {
	return Checkpoint{}, nil
}

func (panickyAdapter) Resume(ctx context.Context, cp Checkpoint) error { return nil }

func TestSafeInvoke_RecoversPanic(t *testing.T) {
	task := Task{ID: "t-1", Type: "test"}
	result, err := SafeInvoke(context.Background(), panickyAdapter{}, task)
	require.Error(t, err, "panic must surface as an error, not crash the goroutine")
	assert.Contains(t, err.Error(), "boom", "error must carry the panic payload")
	assert.Equal(t, "failed", result.Status, "task must be marked failed so the workflow engine records it")
	assert.Equal(t, "t-1", result.TaskID)
}

// niceAdapter returns a normal result.
type niceAdapter struct{}

func (niceAdapter) Declare(ctx context.Context) (AgentCapabilities, error) {
	return AgentCapabilities{}, nil
}

func (niceAdapter) Invoke(ctx context.Context, t Task) (TaskResult, error) {
	return TaskResult{TaskID: t.ID, Status: "completed"}, nil
}

func (niceAdapter) Health(ctx context.Context) (HealthStatus, error) {
	return HealthStatus{Status: "healthy"}, nil
}

func (niceAdapter) Checkpoint(ctx context.Context) (Checkpoint, error) {
	return Checkpoint{}, nil
}

func (niceAdapter) Resume(ctx context.Context, cp Checkpoint) error { return nil }

func TestSafeInvoke_PassesThroughOnSuccess(t *testing.T) {
	result, err := SafeInvoke(context.Background(), niceAdapter{}, Task{ID: "t-2"})
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
}
