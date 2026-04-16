package dialog

import (
	"context"
	"testing"

	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupManager(t *testing.T) *Manager {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	return NewManager(st.DB)
}

func TestCreateThreadAndAddMessage(t *testing.T) {
	m := setupManager(t)
	ctx := context.Background()

	th, err := m.CreateThread(ctx, "alice", "bob", "architecture-review")
	require.NoError(t, err)
	assert.Equal(t, "alice", th.InitiatorAgentID)
	assert.Equal(t, "bob", th.ParticipantAgentID)
	assert.Equal(t, "architecture-review", th.Topic)
	assert.Equal(t, "active", th.Status)

	msg, err := m.AddMessage(ctx, th.ID, "alice", "what do you think of the proposal?")
	require.NoError(t, err)
	assert.Equal(t, th.ID, msg.ThreadID)
	assert.Equal(t, "alice", msg.SenderAgentID)
}

func TestListThreadsIncludesNewThread(t *testing.T) {
	m := setupManager(t)
	ctx := context.Background()

	_, _ = m.CreateThread(ctx, "a", "b", "x")
	_, _ = m.CreateThread(ctx, "c", "d", "y")

	threads, err := m.ListThreads(ctx)
	require.NoError(t, err)
	assert.Len(t, threads, 2)
}

func TestCloseThread(t *testing.T) {
	m := setupManager(t)
	ctx := context.Background()

	th, err := m.CreateThread(ctx, "a", "b", "topic")
	require.NoError(t, err)

	require.NoError(t, m.CloseThread(ctx, th.ID))

	threads, _ := m.ListThreads(ctx)
	require.Len(t, threads, 1)
	assert.Equal(t, "completed", threads[0].Status)
}

func TestGetMessagesReturnsChronologicalOrder(t *testing.T) {
	m := setupManager(t)
	ctx := context.Background()

	th, _ := m.CreateThread(ctx, "a", "b", "topic")
	_, _ = m.AddMessage(ctx, th.ID, "a", "first")
	_, _ = m.AddMessage(ctx, th.ID, "b", "second")
	_, _ = m.AddMessage(ctx, th.ID, "a", "third")

	msgs, err := m.GetMessages(ctx, th.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 3)
	assert.Equal(t, "first", msgs[0].Content)
	assert.Equal(t, "third", msgs[2].Content)
}
