package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

// Janitor periodically deletes old rows from the append-only tables to keep
// storage growth bounded. Ships sane defaults so a single-tenant deployment
// doesn't need any config; large deployments can tune per table.
//
// Design choices:
//
//   - Only completed tasks are deleted. Running/pending tasks are never
//     expired — an orphaned task should be retried or abandoned deliberately,
//     not silently purged by a timer.
//   - Events: full retention window, no status filter. The dashboard already
//     caps to last 50 anyway.
//   - Costs: long-ish default (1y) because billing review windows are long.
//   - Runs in its own goroutine; cancelled via ctx.
//   - Deletes in a single SQL per table. On huge tables, this may block
//     writers on SQLite. A future improvement is chunked deletion, but it
//     adds complexity we don't need yet at the current scale.

// RetentionConfig is the storage-layer view of config.RetentionBlock. Zero
// values = use defaults; negative values = disable that table.
//
// Post-pivot (migration 025) : tasks et costs n'existent plus comme
// tables — elles appartenaient à la plateforme multi-agents. Seules
// events + audit_log restent append-only à nettoyer.
type RetentionConfig struct {
	EventsDays int
	AuditDays  int
	Interval   time.Duration
}

// resolvedDays applies the "zero = default, negative = disabled" contract.
// Returns (days, enabled).
func resolvedDays(v, dflt int) (int, bool) {
	if v < 0 {
		return 0, false
	}
	if v == 0 {
		return dflt, true
	}
	return v, true
}

// RunRetention starts the background janitor. Returns immediately; the
// goroutine stops when ctx is cancelled.
func RunRetention(ctx context.Context, db *sql.DB, cfg RetentionConfig) {
	interval := cfg.Interval
	if interval <= 0 {
		interval = time.Hour
	}
	go func() {
		// Sweep once on startup so tables with backlog get trimmed before
		// the first scheduled tick (which can be an hour away).
		sweepRetention(ctx, db, cfg)
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				sweepRetention(ctx, db, cfg)
			}
		}
	}()
}

func sweepRetention(ctx context.Context, db *sql.DB, cfg RetentionConfig) {
	if days, ok := resolvedDays(cfg.EventsDays, 90); ok {
		deleteOlderThan(ctx, db, "events", "created_at", days, "")
	}
	if days, ok := resolvedDays(cfg.AuditDays, 365); ok {
		deleteOlderThan(ctx, db, "audit_log", "created_at", days, "")
	}
}

// deleteOlderThan purges rows older than `days` from `table` using `column`
// as the age marker. `extraWhere` is an additional SQL fragment appended as
// `AND ...`. The time bound uses a parameterised cutoff (portable across
// SQLite and Postgres) rather than a dialect-specific `NOW() - INTERVAL`.
//
// SECURITY PRECONDITION: table, column, and extraWhere are formatted into
// the query without escaping. This is safe ONLY because every caller in
// sweepRetention passes a hard-coded string literal. Do NOT expose this
// function to any path that could accept user input for those arguments —
// add a validated enum or allowlist first.
func deleteOlderThan(ctx context.Context, db *sql.DB, table, column string, days int, extraWhere string) {
	cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour).Format("2006-01-02 15:04:05")
	q := fmt.Sprintf(`DELETE FROM %s WHERE %s < ? %s`, table, column, extraWhere)
	res, err := db.ExecContext(ctx, q, cutoff)
	if err != nil {
		slog.Warn("retention sweep failed", "table", table, "error", err)
		return
	}
	n, _ := res.RowsAffected()
	if n > 0 {
		slog.Info("retention sweep",
			"table", table, "column", column, "older_than_days", days, "rows_deleted", n)
	}
}
