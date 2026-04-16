package task

import (
	"context"
	"testing"

	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupStore(t *testing.T) (*Store, *event.Bus) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })

	bus := event.NewBus(st.DB)
	return NewStore(st.DB, bus), bus
}

func TestCreateTask(t *testing.T) {
	store, _ := setupStore(t)

	task, err := store.Create(context.Background(), "wf-1", "code-review", `{"file":"main.go"}`, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, task.ID)
	assert.Equal(t, StatusPending, task.Status)
	assert.Equal(t, "code-review", task.Type)
}

func TestTaskStateMachine(t *testing.T) {
	store, _ := setupStore(t)

	task, err := store.Create(context.Background(), "wf-1", "test", `{}`, nil)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, task.Status)

	err = store.Assign(context.Background(), task.ID, "agent-1")
	require.NoError(t, err)

	err = store.Start(context.Background(), task.ID)
	require.NoError(t, err)

	err = store.Complete(context.Background(), task.ID, `{"result":"ok"}`)
	require.NoError(t, err)

	result, err := store.GetByID(context.Background(), task.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, result.Status)
	assert.Contains(t, result.Output, "ok")
}

func TestTaskFail(t *testing.T) {
	store, _ := setupStore(t)

	task, err := store.Create(context.Background(), "wf-1", "test", `{}`, nil)
	require.NoError(t, err)

	err = store.Assign(context.Background(), task.ID, "agent-1")
	require.NoError(t, err)
	err = store.Start(context.Background(), task.ID)
	require.NoError(t, err)

	err = store.Fail(context.Background(), task.ID, "timeout")
	require.NoError(t, err)

	result, err := store.GetByID(context.Background(), task.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, result.Status)
}

func TestSaveCheckpoint(t *testing.T) {
	store, _ := setupStore(t)

	task, err := store.Create(context.Background(), "wf-1", "test", `{}`, nil)
	require.NoError(t, err)

	err = store.SaveCheckpoint(context.Background(), task.ID, `{"step":3}`)
	require.NoError(t, err)

	result, err := store.GetByID(context.Background(), task.ID)
	require.NoError(t, err)
	assert.Contains(t, result.Checkpoint, "step")
}

func TestListByWorkflow(t *testing.T) {
	store, _ := setupStore(t)

	store.Create(context.Background(), "wf-1", "a", `{}`, nil)
	store.Create(context.Background(), "wf-1", "b", `{}`, nil)
	store.Create(context.Background(), "wf-2", "c", `{}`, nil)

	tasks, err := store.ListByWorkflow(context.Background(), "wf-1")
	require.NoError(t, err)
	assert.Len(t, tasks, 2)
}

func TestListPending(t *testing.T) {
	store, _ := setupStore(t)

	store.Create(context.Background(), "wf-1", "code-review", `{}`, nil)
	store.Create(context.Background(), "wf-1", "summarize", `{}`, nil)

	tasks, err := store.ListPending(context.Background(), "code-review")
	require.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, "code-review", tasks[0].Type)
}

func TestEventsEmittedOnStateChange(t *testing.T) {
	store, bus := setupStore(t)

	var events []event.Event
	bus.Subscribe("task.", func(e event.Event) {
		events = append(events, e)
	})

	task, _ := store.Create(context.Background(), "wf-1", "test", `{}`, nil)
	store.Assign(context.Background(), task.ID, "a1")
	store.Start(context.Background(), task.ID)
	store.Complete(context.Background(), task.ID, `{}`)

	assert.Len(t, events, 4) // created, assigned, started, completed
}
