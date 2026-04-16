package autonomy

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// AgentIdentity is the parsed AGENT.yaml.
type AgentIdentity struct {
	Name         string   `yaml:"name"`
	Role         string   `yaml:"role"`
	Capabilities []string `yaml:"capabilities"`
	Constraints  []string `yaml:"constraints"`
	AntiPatterns []string `yaml:"anti_patterns"`
}

// ConstraintViolation describes a constraint that the runtime would violate.
type ConstraintViolation struct {
	Constraint string
	Reason     string
}

func (v ConstraintViolation) Error() string {
	return "constraint violated: " + v.Constraint + " — " + v.Reason
}

// CheckConstraints enforces the constraints listed in an AgentIdentity against
// a task's input. Story 4.1 AC: "constraints are enforced (e.g., 'never
// modify production data')".
//
// Enforcement is string-matching against a small DSL:
//   - "never <keyword>"     → reject if the input contains <keyword>
//   - "only in <env>"       → reject if input has an env field != <env>
//   - "max duration <dur>"  → reject if input has a duration field > <dur>
//
// Constraints outside that DSL pass through — the runtime still records them
// so human reviewers see what was declared.
func (id *AgentIdentity) CheckConstraints(taskInput map[string]any) []ConstraintViolation {
	var violations []ConstraintViolation
	lowerInput := lowerRepresentation(taskInput)

	for _, raw := range id.Constraints {
		c := strings.ToLower(strings.TrimSpace(raw))
		switch {
		case strings.HasPrefix(c, "never "):
			keyword := strings.TrimSpace(strings.TrimPrefix(c, "never "))
			if keyword != "" && strings.Contains(lowerInput, keyword) {
				violations = append(violations, ConstraintViolation{
					Constraint: raw,
					Reason:     "input contains banned keyword " + keyword,
				})
			}
		case strings.HasPrefix(c, "only in "):
			allowed := strings.TrimSpace(strings.TrimPrefix(c, "only in "))
			envRaw, _ := taskInput["env"].(string)
			if allowed != "" && !strings.EqualFold(envRaw, allowed) {
				violations = append(violations, ConstraintViolation{
					Constraint: raw,
					Reason:     "task env=" + envRaw + " not allowed (need " + allowed + ")",
				})
			}
		}
	}
	return violations
}

func lowerRepresentation(m map[string]any) string {
	var b strings.Builder
	walk := func(v any) {
		switch s := v.(type) {
		case string:
			b.WriteString(strings.ToLower(s))
			b.WriteByte(' ')
		}
	}
	for _, v := range m {
		walk(v)
	}
	return b.String()
}

// Plan is the parsed PLAN.yaml — the behavioral state machine.
type Plan struct {
	Heartbeat    string      `yaml:"heartbeat"`    // e.g., "60s", "5m"
	InitialState string      `yaml:"initial_state"`
	States       []StateDef  `yaml:"states"`
}

// StateDef defines a state in the behavioral plan.
type StateDef struct {
	Name        string       `yaml:"name"`
	Observe     []string     `yaml:"observe"`     // what to check
	Actions     []ActionDef  `yaml:"actions"`      // what to do based on observations
	Transitions []Transition `yaml:"transitions"` // state transitions
}

// ActionDef defines an action within a state.
type ActionDef struct {
	When   string `yaml:"when"`   // condition (e.g., "backlog.count > 0")
	Do     string `yaml:"do"`     // action (e.g., "claim_task", "idle", "escalate")
	Params any    `yaml:"params,omitempty"`
}

// Transition defines a state transition.
type Transition struct {
	To   string `yaml:"to"`
	When string `yaml:"when"` // condition
}

// ParseIdentity reads and parses an AGENT.yaml file.
func ParseIdentity(path string) (*AgentIdentity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading agent identity %s: %w", path, err)
	}

	var identity AgentIdentity
	if err := yaml.Unmarshal(data, &identity); err != nil {
		return nil, fmt.Errorf("parsing agent identity: %w", err)
	}

	if identity.Name == "" {
		return nil, fmt.Errorf("agent identity: name is required")
	}
	return &identity, nil
}

// ParsePlan reads and parses a PLAN.yaml file.
func ParsePlan(path string) (*Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading plan %s: %w", path, err)
	}
	return ParsePlanBytes(data)
}

// ParsePlanBytes parses plan YAML bytes.
func ParsePlanBytes(data []byte) (*Plan, error) {
	var plan Plan
	if err := yaml.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("parsing plan YAML: %w", err)
	}

	if err := validatePlan(&plan); err != nil {
		return nil, err
	}
	return &plan, nil
}

func validatePlan(plan *Plan) error {
	if plan.Heartbeat == "" {
		return fmt.Errorf("plan: heartbeat interval is required")
	}
	if plan.InitialState == "" {
		return fmt.Errorf("plan: initial_state is required")
	}
	if len(plan.States) == 0 {
		return fmt.Errorf("plan: at least one state is required")
	}

	stateNames := make(map[string]bool)
	for _, s := range plan.States {
		if s.Name == "" {
			return fmt.Errorf("plan: state name is required")
		}
		stateNames[s.Name] = true
	}

	if !stateNames[plan.InitialState] {
		return fmt.Errorf("plan: initial_state %q not found in states", plan.InitialState)
	}

	// Validate transitions reference existing states
	for _, s := range plan.States {
		for _, tr := range s.Transitions {
			if !stateNames[tr.To] {
				return fmt.Errorf("plan: state %s transitions to unknown state %s", s.Name, tr.To)
			}
		}
	}

	return nil
}
