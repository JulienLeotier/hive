package audit

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupLogger(t *testing.T) *Logger {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	return NewLogger(st.DB)
}

func TestLogAndQuery(t *testing.T) {
	l := setupLogger(t)
	ctx := context.Background()

	require.NoError(t, l.Log(ctx, "agent.register", "alice", "agents/worker", "initial"))
	require.NoError(t, l.Log(ctx, "agent.remove", "bob", "agents/worker", "deprecated"))

	entries, err := l.Query(ctx, time.Now().Add(-time.Hour), 10)
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}

func TestQueryRespectsSinceFilter(t *testing.T) {
	l := setupLogger(t)
	ctx := context.Background()

	require.NoError(t, l.Log(ctx, "a", "x", "r", "d"))

	// Future since → no rows.
	entries, err := l.Query(ctx, time.Now().Add(time.Hour), 10)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestExportJSON(t *testing.T) {
	l := setupLogger(t)
	ctx := context.Background()
	require.NoError(t, l.Log(ctx, "a", "x", "r", "d"))
	entries, _ := l.Query(ctx, time.Now().Add(-time.Hour), 10)

	data, err := l.ExportJSON(entries)
	require.NoError(t, err)

	var parsed []map[string]any
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.Len(t, parsed, 1)
	assert.Equal(t, "a", parsed[0]["action"])
}

func TestExportCSVEscapesInjection(t *testing.T) {
	l := setupLogger(t)
	ctx := context.Background()
	// Values starting with = / + / - / @ must be prefixed with '.
	require.NoError(t, l.Log(ctx, "=cmd|calc", "bob", "r", "d"))
	entries, _ := l.Query(ctx, time.Now().Add(-time.Hour), 10)

	csv := l.ExportCSV(entries)
	assert.True(t, strings.Contains(csv, "'=cmd|calc"), "CSV injection must be neutralised")
}
