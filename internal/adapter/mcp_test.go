package adapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPAdapterDeclareWithServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(AgentCapabilities{
			Name:      "mcp-tools",
			TaskTypes: []string{"search", "read"},
		})
	}))
	defer srv.Close()

	a := NewMCPAdapter(srv.URL, "my-mcp")
	caps, err := a.Declare(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "my-mcp", caps.Name) // Name overridden
	assert.Equal(t, []string{"search", "read"}, caps.TaskTypes)
}

func TestMCPAdapterDeclareFallback(t *testing.T) {
	a := NewMCPAdapter("http://localhost:1", "offline-mcp")
	caps, err := a.Declare(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "offline-mcp", caps.Name)
	assert.Contains(t, caps.TaskTypes, "mcp-tool")
}

func TestMCPAdapterHealthDelegates(t *testing.T) {
	srv := newMockAgent(t) // reuse from http_test.go
	defer srv.Close()

	a := NewMCPAdapter(srv.URL, "mcp-test")
	status, err := a.Health(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "healthy", status.Status)
}
