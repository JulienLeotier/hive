package trust

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetAndReadOverride(t *testing.T) {
	e := setupEngine(t)
	ctx := context.Background()

	require.NoError(t, e.SetOverride(ctx, "a1", "code-review", LevelAutonomous, "proven on code-review"))

	level, err := e.EffectiveLevel(ctx, "a1", "code-review")
	require.NoError(t, err)
	assert.Equal(t, LevelAutonomous, level)

	// Other task types fall back to base level
	level, err = e.EffectiveLevel(ctx, "a1", "deploy")
	require.NoError(t, err)
	assert.Equal(t, "scripted", level, "no override → base level from agents table")
}

func TestOverrideUpsert(t *testing.T) {
	e := setupEngine(t)
	ctx := context.Background()

	require.NoError(t, e.SetOverride(ctx, "a1", "code-review", LevelGuided, "starting"))
	require.NoError(t, e.SetOverride(ctx, "a1", "code-review", LevelAutonomous, "proven"))

	level, err := e.EffectiveLevel(ctx, "a1", "code-review")
	require.NoError(t, err)
	assert.Equal(t, LevelAutonomous, level)

	overrides, err := e.ListOverrides(ctx, "a1")
	require.NoError(t, err)
	assert.Len(t, overrides, 1)
}

func TestInvalidOverrideRejected(t *testing.T) {
	e := setupEngine(t)
	err := e.SetOverride(context.Background(), "a1", "any", "bogus-level", "")
	assert.Error(t, err)
}

func TestRemoveOverride(t *testing.T) {
	e := setupEngine(t)
	ctx := context.Background()

	require.NoError(t, e.SetOverride(ctx, "a1", "code-review", LevelAutonomous, "x"))
	require.NoError(t, e.RemoveOverride(ctx, "a1", "code-review"))

	level, err := e.EffectiveLevel(ctx, "a1", "code-review")
	require.NoError(t, err)
	assert.Equal(t, "scripted", level)
}
