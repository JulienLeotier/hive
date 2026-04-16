package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
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

// Declare auto-detects capabilities from the skill definition. Story 1.5 AC:
// "adapter auto-detects the Claude Code agent's capabilities from its skill
// definition". Reads AGENT.yaml / .claude/AGENT.yaml / skill.yaml — any
// YAML with a `capabilities:` list — and falls back to the generic tag.
func (a *ClaudeCodeAdapter) Declare(ctx context.Context) (AgentCapabilities, error) {
	caps := AgentCapabilities{Name: a.Name, TaskTypes: []string{"claude-code-skill"}}
	candidates := []string{
		"AGENT.yaml",
		".claude/AGENT.yaml",
		"skill.yaml",
		"skill.md", // Markdown frontmatter fallback
	}
	for _, rel := range candidates {
		path := filepath.Join(a.SkillPath, rel)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		parsed := parseSkillCapabilities(data)
		if len(parsed) > 0 {
			caps.TaskTypes = parsed
			break
		}
	}
	return caps, nil
}

// parseSkillCapabilities tolerantly pulls a `capabilities:` list out of YAML
// or the YAML frontmatter of a Markdown file.
func parseSkillCapabilities(data []byte) []string {
	// Strip Markdown frontmatter delimiters if present.
	text := string(data)
	if strings.HasPrefix(text, "---") {
		parts := strings.SplitN(text[3:], "---", 2)
		if len(parts) >= 1 {
			text = parts[0]
		}
	}
	var parsed struct {
		Capabilities []string `yaml:"capabilities"`
	}
	if err := yaml.Unmarshal([]byte(text), &parsed); err != nil {
		return nil
	}
	return parsed.Capabilities
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
