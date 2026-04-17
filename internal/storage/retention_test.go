package storage

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Post-pivot : retention ne touche plus qu'events + audit_log. La
// table tasks a été droppée en migration 025. Ce test vérifie :
// (a) les events anciens sont purgés au-delà du window
// (b) les events récents sont conservés
// (c) la ligne audit_log ancienne est purgée au window séparé
func TestRetention_SweepEventsAndAudit(t *testing.T) {
	store, err := Open(t.TempDir())
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	oldEvt := time.Now().UTC().Add(-100 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	newEvt := time.Now().UTC().Add(-10 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	_, err = store.DB.ExecContext(ctx,
		`INSERT INTO events (type, source, payload, created_at) VALUES
		 ('t1', 's', '{}', ?), ('t2', 's', '{}', ?)`, oldEvt, newEvt)
	require.NoError(t, err)

	// audit_log : default retention = 365 days
	oldAudit := time.Now().UTC().Add(-400 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	_, err = store.DB.ExecContext(ctx,
		`INSERT INTO audit_log (actor, action, resource, created_at) VALUES ('u', 'a', 'r', ?)`,
		oldAudit)
	require.NoError(t, err)

	sweepRetention(ctx, store.DB, RetentionConfig{}) // defaults

	var evtCount, auditCount int
	require.NoError(t, store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM events`).Scan(&evtCount))
	require.NoError(t, store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_log`).Scan(&auditCount))

	assert.Equal(t, 1, evtCount, "100d-old event should be purged, 10d-old kept")
	assert.Equal(t, 0, auditCount, "400d-old audit entry should be purged at 365d window")
}

func TestRetention_NegativeDisables(t *testing.T) {
	store, err := Open(t.TempDir())
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()
	oldEvt := time.Now().UTC().Add(-1000 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	_, err = store.DB.ExecContext(ctx,
		`INSERT INTO events (type, source, payload, created_at) VALUES ('old', 's', '{}', ?)`, oldEvt)
	require.NoError(t, err)

	sweepRetention(ctx, store.DB, RetentionConfig{EventsDays: -1})

	var n int
	require.NoError(t, store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM events`).Scan(&n))
	assert.Equal(t, 1, n, "retention disabled → no purge")
}
