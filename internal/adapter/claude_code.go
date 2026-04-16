package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// ClaudeCodeAdapter wraps a Claude Code skill/workflow as a Hive agent.
// It invokes Claude Code via stdio (command-line execution).
type ClaudeCodeAdapter struct {
	SkillPath string
	Name      string
}

// NewClaudeCodeAdapter creates an adapter for a Claude Code skill at the given path.
func NewClaudeCodeAdapter(skillPath, name string) *ClaudeCodeAdapter {
	return &ClaudeCodeAdapter{SkillPath: skillPath, Name: name}
}

func (a *ClaudeCodeAdapter) Declare(ctx context.Context) (AgentCapabilities, error) {
	return AgentCapabilities{
		Name:      a.Name,
		TaskTypes: []string{"claude-code-skill"},
	}, nil
}

func (a *ClaudeCodeAdapter) Invoke(ctx context.Context, task Task) (TaskResult, error) {
	// Pass input via stdin to avoid command injection via arguments
	inputJSON, err := json.Marshal(task.Input)
	if err != nil {
		return TaskResult{TaskID: task.ID, Status: "failed", Error: "failed to serialize input"}, nil
	}
	cmd := exec.CommandContext(ctx, "claude", "--skill", a.SkillPath)
	cmd.Stdin = strings.NewReader(string(inputJSON))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return TaskResult{
			TaskID: task.ID,
			Status: "failed",
			Error:  fmt.Sprintf("claude code execution failed: %s — %s", err, strings.TrimSpace(string(output))),
		}, nil
	}
	return TaskResult{
		TaskID: task.ID,
		Status: "completed",
		Output: strings.TrimSpace(string(output)),
	}, nil
}

func (a *ClaudeCodeAdapter) Health(ctx context.Context) (HealthStatus, error) {
	_, err := exec.LookPath("claude")
	if err != nil {
		return HealthStatus{Status: "unavailable", Message: "claude CLI not found in PATH"}, nil
	}
	return HealthStatus{Status: "healthy"}, nil
}

func (a *ClaudeCodeAdapter) Checkpoint(ctx context.Context) (Checkpoint, error) {
	return Checkpoint{}, nil
}

func (a *ClaudeCodeAdapter) Resume(ctx context.Context, cp Checkpoint) error {
	return nil
}

// Verify interface compliance at compile time.
var _ Adapter = (*ClaudeCodeAdapter)(nil)
