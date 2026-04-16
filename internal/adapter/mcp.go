package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
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

// isStdio reports whether the MCP server is accessed via a local subprocess
// (stdio:// or file:// URL scheme). Story 1.6 AC.
func (a *MCPAdapter) isStdio() bool {
	return strings.HasPrefix(a.ServerURL, "stdio://") || strings.HasPrefix(a.ServerURL, "file://")
}

// stdioCommand returns the local command path extracted from stdio://path/to/bin.
func (a *MCPAdapter) stdioCommand() string {
	for _, prefix := range []string{"stdio://", "file://"} {
		if strings.HasPrefix(a.ServerURL, prefix) {
			return strings.TrimPrefix(a.ServerURL, prefix)
		}
	}
	return a.ServerURL
}

func (a *MCPAdapter) Declare(ctx context.Context) (AgentCapabilities, error) {
	if a.isStdio() {
		// Invoke the binary with `tools/list` to discover capabilities.
		caps, err := a.stdioCall(ctx, "tools/list", nil)
		if err != nil {
			return AgentCapabilities{Name: a.Name, TaskTypes: []string{"mcp-tool"}}, nil
		}
		var tools []struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal(caps, &tools)
		types := make([]string, 0, len(tools))
		for _, t := range tools {
			types = append(types, t.Name)
		}
		if len(types) == 0 {
			types = []string{"mcp-tool"}
		}
		return AgentCapabilities{Name: a.Name, TaskTypes: types}, nil
	}

	// HTTP MCP server
	caps, err := a.http.Declare(ctx)
	if err != nil {
		return AgentCapabilities{
			Name:      a.Name,
			TaskTypes: []string{"mcp-tool"},
		}, nil
	}
	caps.Name = a.Name
	return caps, nil
}

func (a *MCPAdapter) Invoke(ctx context.Context, task Task) (TaskResult, error) {
	if a.isStdio() {
		payload, err := a.stdioCall(ctx, task.Type, task.Input)
		if err != nil {
			return TaskResult{TaskID: task.ID, Status: "failed", Error: err.Error()}, nil
		}
		return TaskResult{TaskID: task.ID, Status: "completed", Output: json.RawMessage(payload)}, nil
	}
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
	if a.isStdio() {
		// For stdio transport, the server binary is live when the file exists.
		cmd := a.stdioCommand()
		if _, err := exec.LookPath(cmd); err != nil {
			return HealthStatus{Status: "unavailable", Message: err.Error()}, nil
		}
		return HealthStatus{Status: "healthy"}, nil
	}
	return a.http.Health(ctx)
}

func (a *MCPAdapter) Checkpoint(ctx context.Context) (Checkpoint, error) {
	if a.isStdio() {
		// Stdio MCP servers are typically stateless per-call.
		return Checkpoint{Data: map[string]any{}}, nil
	}
	return a.http.Checkpoint(ctx)
}

func (a *MCPAdapter) Resume(ctx context.Context, cp Checkpoint) error {
	if a.isStdio() {
		return nil
	}
	return a.http.Resume(ctx, cp)
}

// stdioCall runs the local MCP binary, pipes a JSON-RPC-ish request in via
// stdin, and returns the raw stdout. Not a full MCP implementation — it's
// enough to satisfy the adapter contract for CLI-based MCP tools.
func (a *MCPAdapter) stdioCall(ctx context.Context, method string, params any) ([]byte, error) {
	body, err := json.Marshal(map[string]any{"method": method, "params": params})
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, a.stdioCommand())
	cmd.Stdin = bytes.NewReader(body)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("mcp stdio call failed: %w", err)
	}
	return out, nil
}

// Verify interface compliance at compile time.
var _ Adapter = (*MCPAdapter)(nil)
