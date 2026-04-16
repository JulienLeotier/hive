package autonomy

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/JulienLeotier/hive/internal/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPlanHotReload verifies Story 4.1 AC: editing PLAN.yaml takes effect at
// the next wake-up.
func TestPlanHotReload(t *testing.T) {
	obs := setupObs(t)
	bus := event.NewBus(obs.db)
	h := NewDefaultHandler(obs, nil, NewIdleTracker(5), bus)

	dir := t.TempDir()
	planPath := filepath.Join(dir, "PLAN.yaml")

	// v1
	require.NoError(t, os.WriteFile(planPath, []byte(`
heartbeat: 60s
initial_state: idle
states:
  - name: idle
`), 0o644))

	h.WithPlan(planPath)
	require.NoError(t, h.Handle(context.Background(), "worker"))
	plan := h.CurrentPlan()
	require.NotNil(t, plan)
	assert.Equal(t, "idle", plan.InitialState)

	// v2 — rewrite the file with a different initial state. Bump mtime so the
	// stat check actually triggers a reload on machines with coarse mtime
	// granularity (some CI setups use 1s resolution).
	time.Sleep(1100 * time.Millisecond)
	require.NoError(t, os.WriteFile(planPath, []byte(`
heartbeat: 60s
initial_state: working
states:
  - name: working
  - name: idle
`), 0o644))

	require.NoError(t, h.Handle(context.Background(), "worker"))
	plan = h.CurrentPlan()
	require.NotNil(t, plan)
	assert.Equal(t, "working", plan.InitialState)
}
