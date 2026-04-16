package autonomy

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchedulerRegisterAndWakeUp(t *testing.T) {
	var count atomic.Int32

	sched := NewScheduler(func(ctx context.Context, name string) error {
		count.Add(1)
		return nil
	})
	defer sched.StopAll()

	sched.Register("test-agent", 50*time.Millisecond)
	time.Sleep(180 * time.Millisecond)

	assert.GreaterOrEqual(t, count.Load(), int32(2))
}

func TestSchedulerUnregister(t *testing.T) {
	var count atomic.Int32

	sched := NewScheduler(func(ctx context.Context, name string) error {
		count.Add(1)
		return nil
	})

	sched.Register("test-agent", 50*time.Millisecond)
	time.Sleep(80 * time.Millisecond)
	sched.Unregister("test-agent")

	snapshot := count.Load()
	time.Sleep(100 * time.Millisecond)

	// No more increments after unregister
	assert.Equal(t, snapshot, count.Load())
}

func TestSchedulerActiveCount(t *testing.T) {
	sched := NewScheduler(func(ctx context.Context, name string) error { return nil })
	defer sched.StopAll()

	require.Equal(t, 0, sched.ActiveCount())

	sched.Register("a", time.Second)
	sched.Register("b", time.Second)
	assert.Equal(t, 2, sched.ActiveCount())

	sched.Unregister("a")
	assert.Equal(t, 1, sched.ActiveCount())
}

// TestSchedulerTriggerWakeUp covers Story 4.2 AC:
// "heartbeats can also be triggered by events (hybrid scheduling)".
func TestSchedulerTriggerWakeUp(t *testing.T) {
	var count atomic.Int32
	sched := NewScheduler(func(ctx context.Context, name string) error {
		count.Add(1)
		return nil
	})
	defer sched.StopAll()

	// Register with a long interval so the tick never fires during the test.
	sched.Register("worker", time.Hour)

	sched.TriggerWakeUp("worker")
	sched.TriggerWakeUp("worker")

	// Give the goroutines a moment to execute the handler.
	time.Sleep(50 * time.Millisecond)

	if count.Load() < 2 {
		t.Fatalf("expected at least 2 handler invocations, got %d", count.Load())
	}
}

func TestSchedulerTriggerIgnoresUnknownAgent(t *testing.T) {
	var count atomic.Int32
	sched := NewScheduler(func(ctx context.Context, name string) error {
		count.Add(1)
		return nil
	})
	defer sched.StopAll()

	sched.TriggerWakeUp("never-registered")
	time.Sleep(20 * time.Millisecond)
	if count.Load() != 0 {
		t.Fatalf("TriggerWakeUp on unknown agent must not run the handler")
	}
}

func TestSchedulerStopAll(t *testing.T) {
	sched := NewScheduler(func(ctx context.Context, name string) error { return nil })

	sched.Register("a", time.Second)
	sched.Register("b", time.Second)
	sched.StopAll()

	assert.Equal(t, 0, sched.ActiveCount())
}
