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

// Declare reads the crew's config (agents.yaml / crewai.yaml) to populate
// capability names. Story 13.1 AC: "detects CrewAI crew configuration and
// maps crew capabilities to Hive protocol".
func (a *CrewAIAdapter) Declare(ctx context.Context) (AgentCapabilities, error) {
	caps := AgentCapabilities{Name: a.Name, TaskTypes: []string{"crewai-crew"}}

	for _, name := range []string{"agents.yaml", "crewai.yaml", "config/agents.yaml"} {
		data, err := os.ReadFile(filepath.Join(a.ProjectPath, name))
		if err != nil {
			continue
		}
		types := parseCrewAIRoles(data)
		if len(types) > 0 {
			caps.TaskTypes = types
			break
		}
	}
	return caps, nil
}

// parseCrewAIRoles pulls role names from either a map-of-agent-configs or a
// top-level `agents:` list in CrewAI config YAML.
func parseCrewAIRoles(data []byte) []string {
	// Try as map: {agent_name: {role, goal, ...}}
	var asMap map[string]struct {
		Role string `yaml:"role"`
	}
	if err := yaml.Unmarshal(data, &asMap); err == nil && len(asMap) > 0 {
		out := make([]string, 0, len(asMap))
		for name := range asMap {
			out = append(out, "crewai-"+name)
		}
		return out
	}
	// Try as list under `agents:` key.
	var asList struct {
		Agents []struct {
			Name string `yaml:"name"`
			Role string `yaml:"role"`
		} `yaml:"agents"`
	}
	if err := yaml.Unmarshal(data, &asList); err == nil {
		out := make([]string, 0, len(asList.Agents))
		for _, a := range asList.Agents {
			switch {
			case a.Name != "":
				out = append(out, "crewai-"+a.Name)
			case a.Role != "":
				out = append(out, "crewai-"+a.Role)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
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
