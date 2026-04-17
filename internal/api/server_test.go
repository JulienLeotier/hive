package api

import (
	"testing"

	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/require"
)

// setupServer builds a test server backed by a temp SQLite. Shared by
// every *_test.go in the package.
func setupServer(t *testing.T) *Server {
	t.Helper()
	store, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	return NewServer(event.NewBus(store.DB))
}
