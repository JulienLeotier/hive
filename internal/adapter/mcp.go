package adapter

import (
	"context"
	"fmt"
)

// MCPAdapter connects to an MCP (Model Context Protocol) server and exposes
// its tools as Hive agent capabilities.
type MCPAdapter struct {
	// ServerURL is the MCP server endpoint (http:// or stdio://).
	ServerURL string
	Name      string
	http      *HTTPAdapter
}

// NewMCPAdapter creates an adapter for an MCP server.
// For HTTP-based MCP servers, it delegates to the HTTP adapter.
func NewMCPAdapter(serverURL, name string) *MCPAdapter {
	return &MCPAdapter{
		ServerURL: serverURL,
		Name:      name,
		http:      NewHTTPAdapter(serverURL),
	}
}

func (a *MCPAdapter) Declare(ctx context.Context) (AgentCapabilities, error) {
	// Try to get tools list from MCP server
	caps, err := a.http.Declare(ctx)
	if err != nil {
		// Fallback: return generic MCP capabilities
		return AgentCapabilities{
			Name:      a.Name,
			TaskTypes: []string{"mcp-tool"},
		}, nil
	}
	caps.Name = a.Name
	return caps, nil
}

func (a *MCPAdapter) Invoke(ctx context.Context, task Task) (TaskResult, error) {
	result, err := a.http.Invoke(ctx, task)
	if err != nil {
		return TaskResult{
			TaskID: task.ID,
			Status: "failed",
			Error:  fmt.Sprintf("MCP invocation failed: %s", err),
		}, nil
	}
	return result, nil
}

func (a *MCPAdapter) Health(ctx context.Context) (HealthStatus, error) {
	return a.http.Health(ctx)
}

func (a *MCPAdapter) Checkpoint(ctx context.Context) (Checkpoint, error) {
	return a.http.Checkpoint(ctx)
}

func (a *MCPAdapter) Resume(ctx context.Context, cp Checkpoint) error {
	return a.http.Resume(ctx, cp)
}

// Verify interface compliance at compile time.
var _ Adapter = (*MCPAdapter)(nil)
