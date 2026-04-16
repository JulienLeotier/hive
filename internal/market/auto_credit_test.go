package market

import (
	"context"
	"testing"
	"time"

	"github.com/JulienLeotier/hive/internal/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAutoCreditOnTaskCompletion exercises Story 18.3:
// "agent earns tokens proportional to task value".
func TestAutoCreditOnTaskCompletion(t *testing.T) {
	st := setupStore(t)
	bus := event.NewBus(st.db)

	// Seed an agent + a completed task.
	_, err := st.db.Exec(
		`INSERT INTO agents (id, name, type, config, capabilities, health_status)
		 VALUES ('a1','cheap','http','{}','{}','healthy')`)
	require.NoError(t, err)
	_, err = st.db.Exec(
		`INSERT INTO tasks (id, workflow_id, type, status, agent_id, input)
		 VALUES ('t1','w','x','completed','a1','{}')`)
	require.NoError(t, err)

	credit := NewAutoCredit(st.db, st, 1.0)
	credit.Attach(bus)

	_, err = bus.Publish(context.Background(), event.TaskCompleted, "system",
		map[string]any{"task_id": "t1", "duration_ms": int64(100)})
	require.NoError(t, err)

	// Subscribers run synchronously in Bus.deliver, but the handler does a DB
	// read so the wallet update is done by the time Publish returns.
	time.Sleep(30 * time.Millisecond)

	balance, err := st.Balance(context.Background(), "cheap")
	require.NoError(t, err)
	assert.Greater(t, balance, 0.0, "completion should credit tokens")
}
