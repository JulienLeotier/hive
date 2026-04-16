package adapter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeCodeAdapterDeclare(t *testing.T) {
	a := NewClaudeCodeAdapter("/path/to/skill", "my-skill")
	caps, err := a.Declare(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "my-skill", caps.Name)
	assert.Contains(t, caps.TaskTypes, "claude-code-skill")
}

func TestClaudeCodeAdapterHealth(t *testing.T) {
	a := NewClaudeCodeAdapter("/path/to/skill", "my-skill")
	status, err := a.Health(context.Background())
	require.NoError(t, err)
	// Claude CLI may or may not be installed in test env
	assert.Contains(t, []string{"healthy", "unavailable"}, status.Status)
}

func TestClaudeCodeAdapterCheckpoint(t *testing.T) {
	a := NewClaudeCodeAdapter("/path/to/skill", "my-skill")
	cp, err := a.Checkpoint(context.Background())
	require.NoError(t, err)
	assert.Nil(t, cp.Data)
}
