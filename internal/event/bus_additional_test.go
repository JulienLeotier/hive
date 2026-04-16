package event

import (
	"context"
	"testing"
	"time"

	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBusSubscriberIsolation verifies subscriber panics don't crash the bus.
func TestBusSubscriberIsolation(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()

	bus := NewBus(st.DB)
	called := 0
	bus.Subscribe("task", func(e Event) {
		panic("boom")
	})
	bus.Subscribe("task", func(e Event) {
		called++
	})

	_, err = bus.Publish(context.Background(), "task.created", "test", nil)
	require.NoError(t, err)
	assert.Equal(t, 1, called, "panicking subscriber must not block other subscribers")
}

func TestBusQueryWithSourceFilter(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()

	bus := NewBus(st.DB)
	ctx := context.Background()
	_, _ = bus.Publish(ctx, "task.created", "agent-a", nil)
	_, _ = bus.Publish(ctx, "task.created", "agent-b", nil)

	events, err := bus.Query(ctx, QueryOpts{Source: "agent-a"})
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "agent-a", events[0].Source)
}

func TestBusQueryWithSinceFilter(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()

	bus := NewBus(st.DB)
	ctx := context.Background()
	_, _ = bus.Publish(ctx, "task.created", "x", nil)

	// Future since → no rows.
	events, err := bus.Query(ctx, QueryOpts{Since: time.Now().Add(time.Hour)})
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestBusPublishErrShim(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()

	bus := NewBus(st.DB)
	err = bus.PublishErr(context.Background(), "t", "s", map[string]int{"a": 1})
	require.NoError(t, err)
}
