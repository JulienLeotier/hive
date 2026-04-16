package autonomy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckConstraintsAllowsCleanInput(t *testing.T) {
	id := &AgentIdentity{
		Name:        "worker",
		Constraints: []string{"never production", "only in staging"},
	}
	v := id.CheckConstraints(map[string]any{"env": "staging", "message": "hello world"})
	assert.Empty(t, v)
}

func TestCheckConstraintsRejectsBannedKeyword(t *testing.T) {
	id := &AgentIdentity{
		Name:        "worker",
		Constraints: []string{"never production"},
	}
	v := id.CheckConstraints(map[string]any{"target": "deploy to production"})
	assert.Len(t, v, 1)
	assert.Contains(t, v[0].Constraint, "production")
}

func TestCheckConstraintsEnforcesEnvScope(t *testing.T) {
	id := &AgentIdentity{
		Constraints: []string{"only in staging"},
	}
	v := id.CheckConstraints(map[string]any{"env": "production", "msg": "hi"})
	assert.Len(t, v, 1)
	assert.Contains(t, v[0].Reason, "production")

	v = id.CheckConstraints(map[string]any{"env": "staging"})
	assert.Empty(t, v)
}

func TestCheckConstraintsIgnoresUnknownDSL(t *testing.T) {
	id := &AgentIdentity{
		Constraints: []string{"be nice", "do no harm"},
	}
	v := id.CheckConstraints(map[string]any{"anything": "goes"})
	assert.Empty(t, v, "free-form constraints pass through — they're documentation, not enforcement")
}
