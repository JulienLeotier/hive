package storage

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPostgresMigrationsRunAgainstRealDB opens a live Postgres (URL from
// POSTGRES_URL / HIVE_POSTGRES_URL env vars) and asserts all 9 translated
// migrations apply cleanly. Story 22.1 end-to-end.
//
// Skips when no URL is supplied so the main `go test ./...` pass stays
// infrastructure-free. To run it locally:
//
//   docker run -d --rm -p 5432:5432 -e POSTGRES_PASSWORD=hive postgres:16
//   POSTGRES_URL='postgres://postgres:hive@localhost:5432/postgres?sslmode=disable' \
//     go test ./internal/storage/ -run Postgres -v
func TestPostgresMigrationsRunAgainstRealDB(t *testing.T) {
	url := os.Getenv("POSTGRES_URL")
	if url == "" {
		url = os.Getenv("HIVE_POSTGRES_URL")
	}
	if url == "" {
		t.Skip("POSTGRES_URL not set; skipping real-Postgres integration")
	}

	store, err := OpenPostgres(url)
	require.NoError(t, err)
	defer store.Close()

	// schema_versions should have one row per migration file.
	var count int
	require.NoError(t, store.DB.QueryRow(`SELECT COUNT(*) FROM schema_versions`).Scan(&count))
	assert.GreaterOrEqual(t, count, 9, "all translated postgres migrations must have been applied")

	// Key tables must exist.
	for _, table := range []string{
		"agents", "events", "tasks", "workflows", "api_keys", "costs",
		"knowledge", "trust_history", "dialog_threads", "dialog_messages", "webhooks",
		"budget_alerts", "agent_trust_overrides",
		"auctions", "bids", "agent_tokens", "federation_links", "audit_log",
		"rbac_users", "cluster_members",
		"optimizations",
	} {
		var exists bool
		err := store.DB.QueryRow(
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)`, table,
		).Scan(&exists)
		require.NoError(t, err)
		assert.Truef(t, exists, "table %s must exist after migrations", table)
	}

	// tenant_id columns must exist on the core tables.
	for _, table := range []string{"agents", "tasks", "workflows", "events", "knowledge"} {
		var exists bool
		err := store.DB.QueryRow(
			`SELECT EXISTS (
			   SELECT 1 FROM information_schema.columns
			   WHERE table_name = $1 AND column_name = 'tenant_id'
			 )`, table,
		).Scan(&exists)
		require.NoError(t, err)
		assert.Truef(t, exists, "tenant_id column must exist on %s", table)
	}

	// node_id on agents (Story 22.3).
	var hasNodeID bool
	err = store.DB.QueryRow(
		`SELECT EXISTS (
		   SELECT 1 FROM information_schema.columns
		   WHERE table_name = 'agents' AND column_name = 'node_id'
		 )`).Scan(&hasNodeID)
	require.NoError(t, err)
	assert.True(t, hasNodeID, "node_id column must exist on agents")
}

// TestPostgresMigrationsAreIdempotent runs OpenPostgres twice and verifies
// the second call doesn't re-apply migrations.
func TestPostgresMigrationsAreIdempotent(t *testing.T) {
	url := os.Getenv("POSTGRES_URL")
	if url == "" {
		url = os.Getenv("HIVE_POSTGRES_URL")
	}
	if url == "" {
		t.Skip("POSTGRES_URL not set; skipping real-Postgres integration")
	}

	s1, err := OpenPostgres(url)
	require.NoError(t, err)
	var firstCount int
	require.NoError(t, s1.DB.QueryRow(`SELECT COUNT(*) FROM schema_versions`).Scan(&firstCount))
	s1.Close()

	s2, err := OpenPostgres(url)
	require.NoError(t, err)
	defer s2.Close()
	var secondCount int
	require.NoError(t, s2.DB.QueryRow(`SELECT COUNT(*) FROM schema_versions`).Scan(&secondCount))

	assert.Equal(t, firstCount, secondCount, "migration count must be stable across re-opens")
}
