package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// CrewAIAdapter wraps a CrewAI crew as a Hive agent.
// Invokes CrewAI via Python subprocess.
type CrewAIAdapter struct {
	ProjectPath string
	Name        string
}

// NewCrewAIAdapter creates an adapter for a CrewAI project.
func NewCrewAIAdapter(projectPath, name string) *CrewAIAdapter {
	return &CrewAIAdapter{ProjectPath: projectPath, Name: name}
}

func (a *CrewAIAdapter) Declare(ctx context.Context) (AgentCapabilities, error) {
	return AgentCapabilities{
		Name:      a.Name,
		TaskTypes: []string{"crewai-crew"},
	}, nil
}

func (a *CrewAIAdapter) Invoke(ctx context.Context, task Task) (TaskResult, error) {
	inputJSON, _ := json.Marshal(task.Input)
	cmd := exec.CommandContext(ctx, "python", "-m", "crewai", "run", "--input", string(inputJSON))
	cmd.Dir = a.ProjectPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return TaskResult{TaskID: task.ID, Status: "failed",
			Error: fmt.Sprintf("crewai execution failed: %s — %s", err, strings.TrimSpace(string(output)))}, nil
	}
	return TaskResult{TaskID: task.ID, Status: "completed", Output: strings.TrimSpace(string(output))}, nil
}

func (a *CrewAIAdapter) Health(ctx context.Context) (HealthStatus, error) {
	if _, err := exec.LookPath("python"); err != nil {
		return HealthStatus{Status: "unavailable", Message: "python not found in PATH"}, nil
	}
	return HealthStatus{Status: "healthy"}, nil
}

func (a *CrewAIAdapter) Checkpoint(ctx context.Context) (Checkpoint, error) { return Checkpoint{}, nil }
func (a *CrewAIAdapter) Resume(ctx context.Context, cp Checkpoint) error    { return nil }

var _ Adapter = (*CrewAIAdapter)(nil)
