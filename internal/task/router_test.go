package task

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/JulienLeotier/hive/internal/adapter"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRouter(t *testing.T) *Router {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })

	// Insert test agents
	caps, _ := json.Marshal(adapter.AgentCapabilities{
		Name:      "reviewer",
		TaskTypes: []string{"code-review", "lint"},
	})
	st.DB.Exec(`INSERT INTO agents (id, name, type, config, capabilities, health_status) VALUES (?, ?, ?, '{}', ?, 'healthy')`,
		"a1", "reviewer", "http", string(caps))

	caps2, _ := json.Marshal(adapter.AgentCapabilities{
		Name:      "writer",
		TaskTypes: []string{"summarize", "write"},
	})
	st.DB.Exec(`INSERT INTO agents (id, name, type, config, capabilities, health_status) VALUES (?, ?, ?, '{}', ?, 'healthy')`,
		"a2", "writer", "http", string(caps2))

	// Unhealthy agent
	caps3, _ := json.Marshal(adapter.AgentCapabilities{
		Name:      "down-agent",
		TaskTypes: []string{"code-review"},
	})
	st.DB.Exec(`INSERT INTO agents (id, name, type, config, capabilities, health_status) VALUES (?, ?, ?, '{}', ?, 'unavailable')`,
		"a3", "down-agent", "http", string(caps3))

	return NewRouter(st.DB)
}

func TestFindCapableAgent(t *testing.T) {
	router := setupRouter(t)

	id, name, err := router.FindCapableAgent(context.Background(), "code-review")
	require.NoError(t, err)
	assert.Equal(t, "a1", id)
	assert.Equal(t, "reviewer", name)
}

func TestFindCapableAgentDifferentType(t *testing.T) {
	router := setupRouter(t)

	id, name, err := router.FindCapableAgent(context.Background(), "summarize")
	require.NoError(t, err)
	assert.Equal(t, "a2", id)
	assert.Equal(t, "writer", name)
}

func TestFindCapableAgentSkipsUnhealthy(t *testing.T) {
	router := setupRouter(t)

	// down-agent can do code-review but is unhealthy — reviewer should be selected
	id, _, err := router.FindCapableAgent(context.Background(), "code-review")
	require.NoError(t, err)
	assert.Equal(t, "a1", id)
}

func TestFindCapableAgentNoneAvailable(t *testing.T) {
	router := setupRouter(t)

	id, name, err := router.FindCapableAgent(context.Background(), "deploy")
	require.NoError(t, err)
	assert.Empty(t, id)
	assert.Empty(t, name)
}

func TestClaimPendingForAgent(t *testing.T) {
	router := setupRouter(t)
	ctx := context.Background()

	// Insert one pending task the reviewer can handle
	_, err := router.db.ExecContext(ctx,
		`INSERT INTO tasks (id, workflow_id, type, status, input) VALUES ('t1','w1','code-review','pending','{}')`)
	require.NoError(t, err)

	id, err := router.ClaimPendingForAgent(ctx, "reviewer")
	require.NoError(t, err)
	assert.Equal(t, "t1", id)

	// Second claim should return empty — task already assigned
	id2, err := router.ClaimPendingForAgent(ctx, "reviewer")
	require.NoError(t, err)
	assert.Empty(t, id2)
}

func TestClaimPendingSkipsIncapableTypes(t *testing.T) {
	router := setupRouter(t)
	ctx := context.Background()

	_, err := router.db.ExecContext(ctx,
		`INSERT INTO tasks (id, workflow_id, type, status, input) VALUES ('t1','w1','deploy','pending','{}')`)
	require.NoError(t, err)

	id, err := router.ClaimPendingForAgent(ctx, "reviewer")
	require.NoError(t, err)
	assert.Empty(t, id)
}

func TestReassign(t *testing.T) {
	router := setupRouter(t)
	ctx := context.Background()

	_, err := router.db.ExecContext(ctx,
		`INSERT INTO tasks (id, workflow_id, type, status, agent_id, input)
		 VALUES ('t1','w1','code-review','running','a1','{}')`)
	require.NoError(t, err)

	require.NoError(t, router.Reassign(ctx, "t1", "agent isolated"))

	var status, agentID string
	router.db.QueryRowContext(ctx, `SELECT status, COALESCE(agent_id,'') FROM tasks WHERE id='t1'`).
		Scan(&status, &agentID)
	assert.Equal(t, "pending", status)
	assert.Empty(t, agentID)
}

func TestRouterPrefersLocalNode(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	router := NewRouter(st.DB)

	// Remote agent
	capsJSON, _ := json.Marshal(adapter.AgentCapabilities{TaskTypes: []string{"code-review"}})
	_, err = st.DB.Exec(
		`INSERT INTO agents (id, name, type, config, capabilities, health_status, node_id)
		 VALUES ('r','remote','http','{}',?,'healthy','other-node')`, string(capsJSON))
	require.NoError(t, err)
	// Local agent
	_, err = st.DB.Exec(
		`INSERT INTO agents (id, name, type, config, capabilities, health_status, node_id)
		 VALUES ('l','local','http','{}',?,'healthy','local-node')`, string(capsJSON))
	require.NoError(t, err)

	oldID, oldMode := LocalNodeID, RoutingMode
	LocalNodeID = "local-node"
	RoutingMode = "local-first"
	t.Cleanup(func() { LocalNodeID, RoutingMode = oldID, oldMode })

	id, name, err := router.FindCapableAgent(context.Background(), "code-review")
	require.NoError(t, err)
	assert.Equal(t, "l", id)
	assert.Equal(t, "local", name)
}

func TestRouterFallsBackToFederation(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	router := NewRouter(st.DB)

	router.WithFederation(func(ctx context.Context, taskType string) (string, string, bool) {
		if taskType == "exotic-task" {
			return "peer-hive", "https://peer.example.com", true
		}
		return "", "", false
	})

	id, name, err := router.FindCapableAgent(context.Background(), "exotic-task")
	require.NoError(t, err)
	assert.Equal(t, "federation:peer-hive", id)
	assert.Equal(t, "peer-hive", name)
}

func TestReassignAgentTasks(t *testing.T) {
	router := setupRouter(t)
	ctx := context.Background()

	router.db.ExecContext(ctx, `INSERT INTO tasks (id, workflow_id, type, status, agent_id, input) VALUES ('t1','w1','code-review','assigned','a1','{}')`)
	router.db.ExecContext(ctx, `INSERT INTO tasks (id, workflow_id, type, status, agent_id, input) VALUES ('t2','w1','lint','running','a1','{}')`)
	router.db.ExecContext(ctx, `INSERT INTO tasks (id, workflow_id, type, status, agent_id, input) VALUES ('t3','w1','summarize','assigned','a2','{}')`)

	n, err := router.ReassignAgentTasks(ctx, "reviewer", "agent swap")
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	// Writer's task untouched
	var status string
	router.db.QueryRowContext(ctx, `SELECT status FROM tasks WHERE id='t3'`).Scan(&status)
	assert.Equal(t, "assigned", status)
}
