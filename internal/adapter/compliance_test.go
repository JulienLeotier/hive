package adapter

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// goodAdapter is a stub that satisfies every compliance check.
type goodAdapter struct{}

func (goodAdapter) Declare(context.Context) (AgentCapabilities, error) {
	return AgentCapabilities{Name: "good", TaskTypes: []string{"ping"}}, nil
}
func (goodAdapter) Invoke(_ context.Context, t Task) (TaskResult, error) {
	return TaskResult{TaskID: t.ID, Status: "completed"}, nil
}
func (goodAdapter) Health(context.Context) (HealthStatus, error) {
	return HealthStatus{Status: "healthy"}, nil
}
func (goodAdapter) Checkpoint(context.Context) (Checkpoint, error) {
	return Checkpoint{Data: map[string]any{"ok": true}}, nil
}
func (goodAdapter) Resume(context.Context, Checkpoint) error { return nil }

// badAdapter violates half the checks.
type badAdapter struct{}

func (badAdapter) Declare(context.Context) (AgentCapabilities, error) {
	return AgentCapabilities{}, nil // empty Name + TaskTypes
}
func (badAdapter) Invoke(_ context.Context, t Task) (TaskResult, error) {
	return TaskResult{TaskID: "wrong-id"}, nil // mismatched task id
}
func (badAdapter) Health(context.Context) (HealthStatus, error) {
	return HealthStatus{}, errors.New("down")
}
func (badAdapter) Checkpoint(context.Context) (Checkpoint, error) { return Checkpoint{}, nil }
func (badAdapter) Resume(context.Context, Checkpoint) error       { return errors.New("nope") }

func TestComplianceAllPasses(t *testing.T) {
	res := RunCompliance(goodAdapter{}, ComplianceOptions{})
	require.True(t, res.OK(), res.Summary())
	assert.Contains(t, res.Passed, "declare")
	assert.Contains(t, res.Passed, "invoke")
	assert.Contains(t, res.Passed, "checkpoint")
	assert.Contains(t, res.Passed, "resume")
}

func TestComplianceReportsFailures(t *testing.T) {
	res := RunCompliance(badAdapter{}, ComplianceOptions{})
	assert.False(t, res.OK())
	assert.Contains(t, res.Failed, "declare.name")
	assert.Contains(t, res.Failed, "health")
	assert.Contains(t, res.Failed, "invoke.task_id")
	assert.Contains(t, res.Failed, "resume")
}

func TestComplianceHonoursSkips(t *testing.T) {
	res := RunCompliance(goodAdapter{}, ComplianceOptions{
		SkipInvoke:     true,
		SkipCheckpoint: true,
	})
	assert.True(t, res.OK())
	assert.Contains(t, res.Skipped, "invoke")
	assert.Contains(t, res.Skipped, "checkpoint")
}
