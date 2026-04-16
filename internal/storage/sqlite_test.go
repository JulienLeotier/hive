package storage

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenCreatesDirectoryAndDatabase(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "data")
	store, err := Open(dir)
	require.NoError(t, err)
	defer store.Close()

	assert.FileExists(t, filepath.Join(dir, "hive.db"))
}

func TestOpenRunsMigrations(t *testing.T) {
	store, err := Open(t.TempDir())
	require.NoError(t, err)
	defer store.Close()

	tables := []string{"agents", "events", "tasks", "workflows", "api_keys", "schema_versions"}
	for _, table := range tables {
		var name string
		err := store.DB.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		require.NoError(t, err, "table %s should exist", table)
		assert.Equal(t, table, name)
	}
}

func TestWALModeEnabled(t *testing.T) {
	store, err := Open(t.TempDir())
	require.NoError(t, err)
	defer store.Close()

	var journalMode string
	err = store.DB.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	require.NoError(t, err)
	assert.Equal(t, "wal", journalMode)
}

func TestMigrationsAreIdempotent(t *testing.T) {
	dir := t.TempDir()

	store1, err := Open(dir)
	require.NoError(t, err)
	store1.Close()

	store2, err := Open(dir)
	require.NoError(t, err)
	defer store2.Close()

	var count int
	err = store2.DB.QueryRow("SELECT COUNT(*) FROM schema_versions").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 6, count, "all migrations should be recorded exactly once each")
}

func TestSchemaVersionTracked(t *testing.T) {
	store, err := Open(t.TempDir())
	require.NoError(t, err)
	defer store.Close()

	var version int
	var appliedAt sql.NullString
	err = store.DB.QueryRow("SELECT version, applied_at FROM schema_versions WHERE version = 1").Scan(&version, &appliedAt)
	require.NoError(t, err)
	assert.Equal(t, 1, version)
	assert.True(t, appliedAt.Valid, "applied_at should be set")
}

func TestClose(t *testing.T) {
	store, err := Open(t.TempDir())
	require.NoError(t, err)

	err = store.Close()
	assert.NoError(t, err)

	// Verify DB is actually closed
	err = store.DB.Ping()
	assert.Error(t, err)
}

func TestDirectoryPermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "secure")
	store, err := Open(dir)
	require.NoError(t, err)
	defer store.Close()

	// Verify directory was created (existence check)
	assert.DirExists(t, dir)
}
