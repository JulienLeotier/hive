package storage

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPostgresMigrationsRunAgainstRealDB opens a live Postgres (URL from
// POSTGRES_URL / HIVE_POSTGRES_URL env vars) and vérifie que les
// migrations post-pivot (schema BMAD) appliquent proprement.
//
// Skip quand aucune URL n'est fournie pour que `go test ./...` reste
// infrastructure-free. Pour le run en local :
//
//	docker run -d --rm -p 5432:5432 -e POSTGRES_PASSWORD=hive postgres:16
//	POSTGRES_URL='postgres://postgres:hive@localhost:5432/postgres?sslmode=disable' \
//	  go test ./internal/storage/ -run Postgres -v
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

	// schema_versions doit contenir une ligne par migration de 001 à 025.
	var count int
	require.NoError(t, store.DB.QueryRow(`SELECT COUNT(*) FROM schema_versions`).Scan(&count))
	assert.GreaterOrEqual(t, count, 25, "toutes les migrations (001-025) doivent avoir été appliquées")

	// Tables live du produit BMAD post-pivot.
	for _, table := range []string{
		"projects", "epics", "stories", "acceptance_criteria",
		"reviews", "bmad_phase_steps",
		"project_conversations", "project_messages",
		"events", "audit_log",
	} {
		var exists bool
		err := store.DB.QueryRow(
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)`, table,
		).Scan(&exists)
		require.NoError(t, err)
		assert.Truef(t, exists, "table %s must exist after migrations", table)
	}

	// Tables pré-pivot droppées par migration 025 — ne doivent PAS exister.
	for _, table := range []string{
		"agents", "tasks", "workflows", "api_keys", "costs",
		"knowledge", "webhooks", "invoices", "rbac_users",
		"dialog_threads", "budget_alerts", "auctions", "bids",
		"federation_links", "cluster_members", "optimizations",
	} {
		var exists bool
		err := store.DB.QueryRow(
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)`, table,
		).Scan(&exists)
		require.NoError(t, err)
		assert.Falsef(t, exists, "table %s doit être droppée par la migration 025", table)
	}

	// Colonnes BMAD clés ajoutées par les migrations récentes.
	for _, col := range []struct{ table, column string }{
		{"projects", "cost_cap_usd"},
		{"projects", "total_cost_usd"},
		{"projects", "failure_stage"},
		{"projects", "is_existing"},
		{"projects", "repo_url"},
		{"bmad_phase_steps", "cost_usd"},
	} {
		var exists bool
		err := store.DB.QueryRow(
			`SELECT EXISTS (
			   SELECT 1 FROM information_schema.columns
			   WHERE table_name = $1 AND column_name = $2
			 )`, col.table, col.column,
		).Scan(&exists)
		require.NoError(t, err)
		assert.Truef(t, exists, "%s.%s doit exister", col.table, col.column)
	}
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
