package autonomy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseIdentity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "AGENT.yaml")
	os.WriteFile(path, []byte(`
name: code-reviewer
role: Reviews code for quality
capabilities:
  - code-review
  - lint
constraints:
  - never modify production data
anti_patterns:
  - generating busywork
`), 0644)

	id, err := ParseIdentity(path)
	require.NoError(t, err)
	assert.Equal(t, "code-reviewer", id.Name)
	assert.Equal(t, []string{"code-review", "lint"}, id.Capabilities)
	assert.Len(t, id.Constraints, 1)
	assert.Len(t, id.AntiPatterns, 1)
}

func TestParseIdentityMissingName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "AGENT.yaml")
	os.WriteFile(path, []byte(`role: something`), 0644)

	_, err := ParseIdentity(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestParsePlan(t *testing.T) {
	yaml := []byte(`
heartbeat: "60s"
initial_state: idle
states:
  - name: idle
    observe: [backlog, events]
    actions:
      - when: "backlog.count > 0"
        do: claim_task
      - when: "backlog.count == 0"
        do: idle
    transitions:
      - to: working
        when: "task_claimed"
  - name: working
    observe: [task_status]
    actions:
      - when: "task.complete"
        do: report_result
    transitions:
      - to: idle
        when: "task.reported"
`)
	plan, err := ParsePlanBytes(yaml)
	require.NoError(t, err)
	assert.Equal(t, "60s", plan.Heartbeat)
	assert.Equal(t, "idle", plan.InitialState)
	assert.Len(t, plan.States, 2)
	assert.Len(t, plan.States[0].Actions, 2)
	assert.Len(t, plan.States[0].Transitions, 1)
}

func TestParsePlanMissingHeartbeat(t *testing.T) {
	yaml := []byte(`
initial_state: idle
states:
  - name: idle
`)
	_, err := ParsePlanBytes(yaml)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "heartbeat")
}

func TestParsePlanMissingInitialState(t *testing.T) {
	yaml := []byte(`
heartbeat: "30s"
states:
  - name: idle
`)
	_, err := ParsePlanBytes(yaml)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "initial_state")
}

func TestParsePlanInvalidInitialState(t *testing.T) {
	yaml := []byte(`
heartbeat: "30s"
initial_state: ghost
states:
  - name: idle
`)
	_, err := ParsePlanBytes(yaml)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found in states")
}

func TestParsePlanInvalidTransition(t *testing.T) {
	yaml := []byte(`
heartbeat: "30s"
initial_state: idle
states:
  - name: idle
    transitions:
      - to: ghost
        when: "always"
`)
	_, err := ParsePlanBytes(yaml)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown state ghost")
}
