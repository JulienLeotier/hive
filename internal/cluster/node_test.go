package cluster

import (
	"context"
	"testing"
	"time"

	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRoster(t *testing.T) *Roster {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	return NewRoster(st.DB)
}

func TestHeartbeatAndList(t *testing.T) {
	r := setupRoster(t)
	ctx := context.Background()

	require.NoError(t, r.Heartbeat(ctx, &Node{ID: "n1", Hostname: "host-1", Address: ":7777", Status: "active"}))
	require.NoError(t, r.Heartbeat(ctx, &Node{ID: "n2", Hostname: "host-2", Address: ":7778", Status: "active"}))

	nodes, err := r.List(ctx)
	require.NoError(t, err)
	assert.Len(t, nodes, 2)
}

func TestHeartbeatIsUpsert(t *testing.T) {
	r := setupRoster(t)
	ctx := context.Background()

	require.NoError(t, r.Heartbeat(ctx, &Node{ID: "n1", Hostname: "a", Address: ":1", Status: "active"}))
	require.NoError(t, r.Heartbeat(ctx, &Node{ID: "n1", Hostname: "b", Address: ":2", Status: "draining"}))

	nodes, _ := r.List(ctx)
	require.Len(t, nodes, 1)
	assert.Equal(t, "b", nodes[0].Hostname)
	assert.Equal(t, "draining", nodes[0].Status)
}

func TestMarkStaleMovesNodesOffline(t *testing.T) {
	r := setupRoster(t)
	ctx := context.Background()
	require.NoError(t, r.Heartbeat(ctx, &Node{ID: "n1", Hostname: "a", Address: ":1", Status: "active"}))

	// Force the heartbeat into the past.
	_, err := r.db.ExecContext(ctx, `UPDATE cluster_members SET last_heartbeat = datetime('now','-1 hour')`)
	require.NoError(t, err)

	n, err := r.MarkStale(ctx, time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	nodes, _ := r.List(ctx)
	assert.Equal(t, "offline", nodes[0].Status)
}

func TestPickAgentPrefersLocal(t *testing.T) {
	m := NewManager(Config{NodeID: "self", RoutingMode: "local-first"})
	perNode := map[string][]string{
		"self":  {"alpha"},
		"other": {"beta"},
	}
	assert.Equal(t, "alpha", m.PickAgent(perNode, "x"))
}

func TestPickAgentBestFitFallsBack(t *testing.T) {
	m := NewManager(Config{NodeID: "self", RoutingMode: "best-fit"})
	perNode := map[string][]string{
		"other": {"beta"},
	}
	assert.Equal(t, "beta", m.PickAgent(perNode, "x"))
}

// TestPickAgentBestFit_SpreadsAcrossNodes proves that best-fit no longer
// pins every task onto the alphabetically-first node. Given 4 nodes each
// holding one agent, a varied set of task types should land on at least 2
// distinct nodes. The previous first-match impl would always pick "node-a".
func TestPickAgentBestFit_SpreadsAcrossNodes(t *testing.T) {
	m := NewManager(Config{NodeID: "self", RoutingMode: "best-fit"})
	perNode := map[string][]string{
		"node-a": {"agent-a"},
		"node-b": {"agent-b"},
		"node-c": {"agent-c"},
		"node-d": {"agent-d"},
	}
	types := []string{"review", "summarize", "translate", "embed", "classify", "extract"}
	picked := map[string]bool{}
	for _, tt := range types {
		picked[m.PickAgent(perNode, tt)] = true
	}
	assert.GreaterOrEqual(t, len(picked), 2,
		"different task types should land on different nodes (got picks: %v)", picked)
}

// TestPickAgentBestFit_Deterministic: same task type always returns the
// same agent. Lets schedulers running in different processes agree on
// routing without coordination.
func TestPickAgentBestFit_Deterministic(t *testing.T) {
	m := NewManager(Config{NodeID: "self", RoutingMode: "best-fit"})
	perNode := map[string][]string{
		"node-a": {"agent-a"},
		"node-b": {"agent-b"},
		"node-c": {"agent-c"},
	}
	first := m.PickAgent(perNode, "summarize")
	for i := 0; i < 10; i++ {
		assert.Equal(t, first, m.PickAgent(perNode, "summarize"),
			"same task type must pick the same agent every call")
	}
}
