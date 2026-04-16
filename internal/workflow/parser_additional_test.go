package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAllCollectsMultipleIssues(t *testing.T) {
	cfg := &Config{
		Name:  "",
		Tasks: []TaskDef{{Name: "a", Type: "x"}, {Name: "a", Type: "y"}}, // duplicate
	}
	issues := ValidateAll(cfg)
	require.NotEmpty(t, issues)
	// Expect at least: name required + duplicate name.
	assert.GreaterOrEqual(t, len(issues), 2)
}

func TestValidateAllCatchesCycle(t *testing.T) {
	cfg := &Config{
		Name: "wf",
		Tasks: []TaskDef{
			{Name: "a", Type: "x", DependsOn: []string{"b"}},
			{Name: "b", Type: "x", DependsOn: []string{"a"}},
		},
	}
	issues := ValidateAll(cfg)
	require.NotEmpty(t, issues)
	found := false
	for _, i := range issues {
		if i.Error() == "circular dependency detected in workflow tasks" {
			found = true
		}
	}
	assert.True(t, found, "cycle should surface as an issue")
}

func TestCriticalPathSingleChain(t *testing.T) {
	tasks := []TaskDef{
		{Name: "a", Type: "x"},
		{Name: "b", Type: "x", DependsOn: []string{"a"}},
		{Name: "c", Type: "x", DependsOn: []string{"b"}},
	}
	path := CriticalPath(tasks)
	assert.Equal(t, []string{"a", "b", "c"}, path)
}

func TestCriticalPathParallelBranches(t *testing.T) {
	tasks := []TaskDef{
		{Name: "start", Type: "x"},
		{Name: "short", Type: "x", DependsOn: []string{"start"}},
		{Name: "m1", Type: "x", DependsOn: []string{"start"}},
		{Name: "m2", Type: "x", DependsOn: []string{"m1"}},
		{Name: "end", Type: "x", DependsOn: []string{"short", "m2"}},
	}
	path := CriticalPath(tasks)
	// longest dependency chain: start -> m1 -> m2 -> end (4 nodes)
	assert.Equal(t, []string{"start", "m1", "m2", "end"}, path)
}

func TestValidateAllRejectsSelfDep(t *testing.T) {
	cfg := &Config{
		Name:  "wf",
		Tasks: []TaskDef{{Name: "a", Type: "x", DependsOn: []string{"a"}}},
	}
	issues := ValidateAll(cfg)
	foundSelf := false
	for _, i := range issues {
		if i.Error() == "task a cannot depend on itself" {
			foundSelf = true
		}
	}
	assert.True(t, foundSelf)
}
