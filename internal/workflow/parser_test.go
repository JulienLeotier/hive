package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseValidWorkflow(t *testing.T) {
	yaml := []byte(`
name: code-review
tasks:
  - name: review
    type: code-review
    input: {file: "main.go"}
  - name: summarize
    type: summarize
    depends_on: [review]
`)
	cfg, err := Parse(yaml)
	require.NoError(t, err)
	assert.Equal(t, "code-review", cfg.Name)
	assert.Len(t, cfg.Tasks, 2)
	assert.Equal(t, []string{"review"}, cfg.Tasks[1].DependsOn)
}

func TestParseMissingName(t *testing.T) {
	yaml := []byte(`
tasks:
  - name: review
    type: code-review
`)
	_, err := Parse(yaml)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestParseNoTasks(t *testing.T) {
	yaml := []byte(`
name: empty
tasks: []
`)
	_, err := Parse(yaml)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one task")
}

func TestParseDuplicateTaskName(t *testing.T) {
	yaml := []byte(`
name: dup
tasks:
  - name: review
    type: code-review
  - name: review
    type: lint
`)
	_, err := Parse(yaml)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate task name")
}

func TestParseUnknownDependency(t *testing.T) {
	yaml := []byte(`
name: bad-dep
tasks:
  - name: review
    type: code-review
    depends_on: [ghost]
`)
	_, err := Parse(yaml)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown task ghost")
}

func TestParseSelfDependency(t *testing.T) {
	yaml := []byte(`
name: self-dep
tasks:
  - name: review
    type: code-review
    depends_on: [review]
`)
	_, err := Parse(yaml)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot depend on itself")
}

func TestParseCircularDependency(t *testing.T) {
	yaml := []byte(`
name: circular
tasks:
  - name: a
    type: step
    depends_on: [c]
  - name: b
    type: step
    depends_on: [a]
  - name: c
    type: step
    depends_on: [b]
`)
	_, err := Parse(yaml)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
}

func TestParseWithTrigger(t *testing.T) {
	yaml := []byte(`
name: scheduled
trigger:
  type: schedule
  schedule: "*/5 * * * *"
tasks:
  - name: check
    type: health-check
`)
	cfg, err := Parse(yaml)
	require.NoError(t, err)
	assert.NotNil(t, cfg.Trigger)
	assert.Equal(t, "schedule", cfg.Trigger.Type)
}

func TestTopologicalSort(t *testing.T) {
	tasks := []TaskDef{
		{Name: "a", Type: "step"},
		{Name: "b", Type: "step", DependsOn: []string{"a"}},
		{Name: "c", Type: "step", DependsOn: []string{"a"}},
		{Name: "d", Type: "step", DependsOn: []string{"b", "c"}},
	}

	levels, err := TopologicalSort(tasks)
	require.NoError(t, err)
	assert.Len(t, levels, 3)
	assert.Len(t, levels[0], 1) // [a]
	assert.Len(t, levels[1], 2) // [b, c] parallel
	assert.Len(t, levels[2], 1) // [d]
}

func TestTopologicalSortFlat(t *testing.T) {
	tasks := []TaskDef{
		{Name: "a", Type: "step"},
		{Name: "b", Type: "step"},
		{Name: "c", Type: "step"},
	}

	levels, err := TopologicalSort(tasks)
	require.NoError(t, err)
	assert.Len(t, levels, 1)    // all in parallel
	assert.Len(t, levels[0], 3)
}
