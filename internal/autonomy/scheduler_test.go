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

func TestSchedulerStopAll(t *testing.T) {
	sched := NewScheduler(func(ctx context.Context, name string) error { return nil })

	sched.Register("a", time.Second)
	sched.Register("b", time.Second)
	sched.StopAll()

	assert.Equal(t, 0, sched.ActiveCount())
}
