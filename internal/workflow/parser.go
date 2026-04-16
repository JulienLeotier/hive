package workflow

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the parsed representation of a hive.yaml workflow file.
type Config struct {
	Name    string       `yaml:"name"`
	Tasks   []TaskDef    `yaml:"tasks"`
	Trigger *TriggerDef  `yaml:"trigger,omitempty"`
}

// TaskDef defines a single task within a workflow.
type TaskDef struct {
	Name      string   `yaml:"name"`
	Type      string   `yaml:"type"` // capability required (e.g., "code-review")
	Input     any      `yaml:"input,omitempty"`
	DependsOn []string `yaml:"depends_on,omitempty"`
	Condition string   `yaml:"condition,omitempty"` // e.g., "upstream.review.score > 0.8"
	// Default marks this task as the "else" branch. Runs iff no sibling at the
	// same DAG level (same DependsOn set) had a condition that evaluated to true.
	Default bool `yaml:"default,omitempty"`
}

// TriggerDef defines how a workflow is triggered.
type TriggerDef struct {
	Type     string `yaml:"type"`     // "manual", "webhook", "schedule"
	Schedule string `yaml:"schedule,omitempty"` // cron expression
	Webhook  string `yaml:"webhook,omitempty"`  // endpoint path
}

// ParseFile reads and parses a workflow YAML file.
func ParseFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading workflow file %s: %w", path, err)
	}
	return Parse(data)
}

// Parse parses workflow YAML bytes.
func Parse(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing workflow YAML: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func validate(cfg *Config) error {
	if cfg.Name == "" {
		return fmt.Errorf("workflow name is required")
	}
	if len(cfg.Tasks) == 0 {
		return fmt.Errorf("workflow must have at least one task")
	}

	taskNames := make(map[string]bool)
	for _, t := range cfg.Tasks {
		if t.Name == "" {
			return fmt.Errorf("task name is required")
		}
		if t.Type == "" {
			return fmt.Errorf("task %s: type is required", t.Name)
		}
		if taskNames[t.Name] {
			return fmt.Errorf("duplicate task name: %s", t.Name)
		}
		taskNames[t.Name] = true
	}

	// Validate dependency references
	for _, t := range cfg.Tasks {
		for _, dep := range t.DependsOn {
			if !taskNames[dep] {
				return fmt.Errorf("task %s depends on unknown task %s", t.Name, dep)
			}
			if dep == t.Name {
				return fmt.Errorf("task %s cannot depend on itself", t.Name)
			}
		}
	}

	// Detect circular dependencies via topological sort
	if err := detectCycles(cfg.Tasks); err != nil {
		return err
	}

	return nil
}

func detectCycles(tasks []TaskDef) error {
	// Build adjacency list
	graph := make(map[string][]string)
	inDegree := make(map[string]int)

	for _, t := range tasks {
		graph[t.Name] = nil
		inDegree[t.Name] = 0
	}
	for _, t := range tasks {
		for _, dep := range t.DependsOn {
			graph[dep] = append(graph[dep], t.Name)
			inDegree[t.Name]++
		}
	}

	// Kahn's algorithm
	var queue []string
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	visited := 0
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		visited++

		for _, neighbor := range graph[node] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if visited != len(tasks) {
		return fmt.Errorf("circular dependency detected in workflow tasks")
	}
	return nil
}

// TopologicalSort returns tasks in dependency order.
func TopologicalSort(tasks []TaskDef) ([][]TaskDef, error) {
	if err := detectCycles(tasks); err != nil {
		return nil, err
	}

	graph := make(map[string][]string)
	inDegree := make(map[string]int)
	taskMap := make(map[string]TaskDef)

	for _, t := range tasks {
		taskMap[t.Name] = t
		inDegree[t.Name] = 0
	}
	for _, t := range tasks {
		for _, dep := range t.DependsOn {
			graph[dep] = append(graph[dep], t.Name)
			inDegree[t.Name]++
		}
	}

	// Group by levels (tasks at same level can run in parallel)
	var levels [][]TaskDef
	var queue []string
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	for len(queue) > 0 {
		level := make([]TaskDef, 0, len(queue))
		var next []string

		for _, name := range queue {
			level = append(level, taskMap[name])
			for _, neighbor := range graph[name] {
				inDegree[neighbor]--
				if inDegree[neighbor] == 0 {
					next = append(next, neighbor)
				}
			}
		}

		levels = append(levels, level)
		queue = next
	}

	return levels, nil
}
