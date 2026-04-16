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
