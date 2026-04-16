package event

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupBus(t *testing.T) *Bus {
	store, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	return NewBus(store.DB)
}

func TestPublishPersistsEvent(t *testing.T) {
	bus := setupBus(t)

	evt, err := bus.Publish(context.Background(), TaskCreated, "system", map[string]string{"task_id": "t1"})
	require.NoError(t, err)
	assert.Greater(t, evt.ID, int64(0))
	assert.Equal(t, TaskCreated, evt.Type)
	assert.Equal(t, "system", evt.Source)
}

func TestPublishDeliversToSubscribers(t *testing.T) {
	bus := setupBus(t)

	var received []Event
	var mu sync.Mutex

	bus.Subscribe("task.", func(e Event) {
		mu.Lock()
		received = append(received, e)
		mu.Unlock()
	})

	_, err := bus.Publish(context.Background(), TaskCreated, "system", nil)
	require.NoError(t, err)
	_, err = bus.Publish(context.Background(), TaskCompleted, "system", nil)
	require.NoError(t, err)
	// This should NOT be delivered (different prefix)
	_, err = bus.Publish(context.Background(), AgentRegistered, "system", nil)
	require.NoError(t, err)

	mu.Lock()
	assert.Len(t, received, 2)
	mu.Unlock()
}

func TestSubscribeWildcard(t *testing.T) {
	bus := setupBus(t)

	var count int
	bus.Subscribe("*", func(e Event) { count++ })

	bus.Publish(context.Background(), TaskCreated, "a", nil)
	bus.Publish(context.Background(), AgentRegistered, "b", nil)
	bus.Publish(context.Background(), WorkflowStarted, "c", nil)

	assert.Equal(t, 3, count)
}

func TestEventOrdering(t *testing.T) {
	bus := setupBus(t)

	for i := 0; i < 10; i++ {
		_, err := bus.Publish(context.Background(), TaskCreated, "system", map[string]int{"seq": i})
		require.NoError(t, err)
	}

	events, err := bus.Query(context.Background(), QueryOpts{Type: "task"})
	require.NoError(t, err)
	assert.Len(t, events, 10)

	// Verify strict ordering by ID
	for i := 1; i < len(events); i++ {
		assert.Greater(t, events[i].ID, events[i-1].ID)
	}
}

func TestQueryByType(t *testing.T) {
	bus := setupBus(t)

	bus.Publish(context.Background(), TaskCreated, "system", nil)
	bus.Publish(context.Background(), TaskCompleted, "system", nil)
	bus.Publish(context.Background(), AgentRegistered, "system", nil)

	events, err := bus.Query(context.Background(), QueryOpts{Type: "task"})
	require.NoError(t, err)
	assert.Len(t, events, 2)
}

func TestQueryBySource(t *testing.T) {
	bus := setupBus(t)

	bus.Publish(context.Background(), TaskCreated, "agent-a", nil)
	bus.Publish(context.Background(), TaskCreated, "agent-b", nil)

	events, err := bus.Query(context.Background(), QueryOpts{Source: "agent-a"})
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, "agent-a", events[0].Source)
}

func TestQueryWithLimit(t *testing.T) {
	bus := setupBus(t)

	for i := 0; i < 20; i++ {
		bus.Publish(context.Background(), TaskCreated, "system", nil)
	}

	events, err := bus.Query(context.Background(), QueryOpts{Limit: 5})
	require.NoError(t, err)
	assert.Len(t, events, 5)
}

func TestQuerySince(t *testing.T) {
	bus := setupBus(t)

	bus.Publish(context.Background(), TaskCreated, "system", nil)
	// Query since future time should return nothing
	events, err := bus.Query(context.Background(), QueryOpts{Since: time.Now().Add(time.Hour)})
	require.NoError(t, err)
	assert.Len(t, events, 0)
}
