package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	agentPkg "github.com/JulienLeotier/hive/internal/agent"
	eventPkg "github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/knowledge"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/JulienLeotier/hive/internal/trust"
	"github.com/JulienLeotier/hive/internal/webhook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestV02EndToEnd exercises the Story 12.2 acceptance criteria in one test:
// register agent → "run workflow" (simulate task completions) → trust promotes
// → knowledge is recorded → webhook fires → dashboard-facing events are emitted.
func TestV02EndToEnd(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()

	ctx := context.Background()
	bus := eventPkg.NewBus(st.DB)

	// Collect every event type the integration is supposed to emit so we can
	// assert the full flow happened.
	var observed atomic.Value
	observed.Store(map[string]int{})
	track := func(e eventPkg.Event) {
		cur := observed.Load().(map[string]int)
		next := map[string]int{}
		for k, v := range cur {
			next[k] = v
		}
		next[e.Type]++
		observed.Store(next)
	}
	bus.Subscribe("*", track)

	// 1. Register an agent (manager publishes agent.registered via bus).
	_, err = st.DB.ExecContext(ctx,
		`INSERT INTO agents (id, name, type, config, capabilities, health_status, trust_level)
		 VALUES ('a1','reviewer','http','{}','{"task_types":["code-review"]}','healthy','scripted')`)
	require.NoError(t, err)
	_, _ = bus.Publish(ctx, "agent.registered", "reviewer",
		map[string]string{"id": "a1", "type": "http", "url": "http://fake"})

	// 2. Webhook configured on task events — stand up a receiver to verify fires.
	var webhookFired int32
	recvSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&webhookFired, 1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer recvSrv.Close()

	// Seed webhook row so the dispatcher has something to deliver to.
	_, err = st.DB.ExecContext(ctx,
		`INSERT INTO webhooks (id, name, url, type, event_filter, enabled)
		 VALUES ('wh1', 'ci-webhook', ?, 'generic', 'task.*', 1)`, recvSrv.URL)
	require.NoError(t, err)

	dispatcher := webhook.NewDispatcher(st.DB)
	// Subscribe the dispatcher to every event so webhooks fire on matching types.
	bus.Subscribe("*", func(e eventPkg.Event) {
		dispatcher.Dispatch(context.Background(), e)
	})

	// 3. Simulate "run workflow" = a stream of completed tasks.
	engine := trust.NewEngine(st.DB, trust.Thresholds{
		GuidedAfterTasks:     3,
		GuidedMaxErrorRate:   0.5,
		AutonomousAfterTasks: 6,
		AutonomousMaxError:   0.2,
	}).WithBus(bus)

	for i := 0; i < 8; i++ {
		_, _ = st.DB.ExecContext(ctx,
			`INSERT INTO tasks (id, workflow_id, type, status, agent_id, input, output, started_at, completed_at)
			 VALUES (?, 'wf-1', 'code-review', 'completed', 'a1', '{}', '{"score":0.9}', datetime('now'), datetime('now'))`,
			"t-"+string(rune('a'+i)))
		_, _ = bus.Publish(ctx, "task.completed", "reviewer", map[string]string{"task_id": "t-x", "result": "ok"})
	}

	// 4. Trust engine evaluates and promotes.
	promoted, newLevel, err := engine.Evaluate(ctx, "a1")
	require.NoError(t, err)
	require.True(t, promoted, "agent should have been promoted past scripted")
	assert.NotEqual(t, "scripted", newLevel)

	// 5. Knowledge layer records an approach.
	kStore := knowledge.NewStore(st.DB).WithEmbedder(knowledge.NewHashingEmbedder(128))
	require.NoError(t, kStore.Record(ctx, "code-review", "check for null pointers", "success", `{"lang":"go"}`))

	found, err := kStore.VectorSearch(ctx, "null pointer check", 5)
	require.NoError(t, err)
	assert.NotEmpty(t, found, "vector search should recall the recorded approach")

	// 6. Give the webhook dispatcher a moment to deliver.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&webhookFired) > 0 {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	require.Greater(t, atomic.LoadInt32(&webhookFired), int32(0), "webhook should have fired on task.completed")

	// 7. Dashboard-facing events must include at least one agent + task event
	//    so a subscribed client sees updates (this is what the WS broadcasts).
	events, err := bus.Query(ctx, eventPkg.QueryOpts{})
	require.NoError(t, err)
	types := map[string]bool{}
	for _, e := range events {
		types[e.Type] = true
	}
	assert.True(t, types["agent.registered"], "dashboard must observe agent lifecycle")
	assert.True(t, types["task.completed"], "dashboard must observe task lifecycle")
	assert.Truef(t, anyWith(types, "decision."), "dashboard must observe decision events (trust promotion)")

	_ = agentPkg.NewManager(st.DB) // guard against import-only warnings
}

func anyWith(m map[string]bool, prefix string) bool {
	for k := range m {
		if strings.HasPrefix(k, prefix) {
			return true
		}
	}
	return false
}

func init() {
	// Keep json import alive without unused-variable complaints if the test
	// evolves to assert payload bodies directly.
	_ = json.Marshal
}
