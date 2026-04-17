// Package billing aggregates per-tenant spend from the costs table into
// monthly invoices. The data plumbing stays hive-native; plugging in a
// payment gateway (Stripe, Paddle, etc.) is done by satisfying the
// Gateway interface and calling Generator.PushToGateway on `issued`
// invoices.
//
// Why decoupled: a self-hosted Hive instance may never use a gateway —
// operators just want to know how much each tenant consumed for
// internal chargeback. A SaaS deployment wires Stripe and pushes
// invoices through. Same core table, different downstream.
package billing

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/oklog/ulid/v2"
)

// Status values mirrored as exported constants so callers can switch
// without magic strings leaking.
const (
	StatusDraft  = "draft"
	StatusIssued = "issued"
	StatusPaid   = "paid"
	StatusVoid   = "void"
)

// Invoice is the persisted shape of one tenant's monthly bill.
type Invoice struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	TotalAmount float64   `json:"total_amount"`
	TaskCount   int       `json:"task_count"`
	Currency    string    `json:"currency"`
	Status      string    `json:"status"`
	ExternalID  string    `json:"external_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	IssuedAt    time.Time `json:"issued_at,omitempty"`
	PaidAt      time.Time `json:"paid_at,omitempty"`
}

// Gateway is the plug-in surface for external payment providers. Stripe,
// Paddle, or a custom webhook can satisfy it. The generator calls Register
// once per newly-issued invoice; it's fine for the implementation to be
// idempotent by external_id so re-runs don't create duplicates upstream.
type Gateway interface {
	// Register pushes the invoice to the gateway and returns the external ID
	// that gateway assigned (e.g. a Stripe invoice `in_...`).
	Register(ctx context.Context, inv Invoice) (externalID string, err error)
	// Name is used in log/event tagging so operators can tell which gateway
	// produced a given entry.
	Name() string
}

// Generator turns the per-task costs log into tenant-scoped monthly
// invoices.
type Generator struct {
	db       *sql.DB
	gateway  Gateway
	currency string
}

// NewGenerator builds a generator backed by the hive DB. Currency defaults
// to USD when empty; callers that bill in € / £ / ¥ pass explicitly.
func NewGenerator(db *sql.DB, currency string) *Generator {
	if currency == "" {
		currency = "USD"
	}
	return &Generator{db: db, currency: currency}
}

// WithGateway attaches a payment gateway. Without one, invoices live
// entirely inside the hive — useful for internal chargeback or dry-runs.
func (g *Generator) WithGateway(gw Gateway) *Generator {
	g.gateway = gw
	return g
}

// GenerateForPeriod computes every tenant's spend between start and end and
// upserts one invoice per tenant. Safe to run repeatedly — the unique
// constraint on (tenant_id, period_start, period_end) plus the
// ON CONFLICT clause refresh amounts without duplicating rows.
//
// Newly-created invoices default to `draft`. Call Issue() to move them to
// `issued` (which also pushes to the gateway when one is installed).
func (g *Generator) GenerateForPeriod(ctx context.Context, start, end time.Time) (int, error) {
	rows, err := g.db.QueryContext(ctx,
		`SELECT COALESCE(tenant_id, 'default'), COUNT(*), COALESCE(SUM(cost), 0)
		 FROM costs
		 WHERE created_at >= ? AND created_at < ?
		 GROUP BY tenant_id`,
		start.Format("2006-01-02 15:04:05"),
		end.Format("2006-01-02 15:04:05"))
	if err != nil {
		return 0, fmt.Errorf("aggregating costs: %w", err)
	}
	defer rows.Close()

	type agg struct {
		tenant string
		count  int
		total  float64
	}
	var rowsOut []agg
	for rows.Next() {
		var a agg
		if err := rows.Scan(&a.tenant, &a.count, &a.total); err != nil {
			return 0, err
		}
		rowsOut = append(rowsOut, a)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	written := 0
	for _, a := range rowsOut {
		id := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
		_, err := g.db.ExecContext(ctx,
			`INSERT INTO invoices (id, tenant_id, period_start, period_end,
			    total_amount, task_count, currency, status)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT (tenant_id, period_start, period_end)
			 DO UPDATE SET total_amount = excluded.total_amount,
			               task_count = excluded.task_count`,
			id, a.tenant,
			start.Format("2006-01-02 15:04:05"),
			end.Format("2006-01-02 15:04:05"),
			a.total, a.count, g.currency, StatusDraft,
		)
		if err != nil {
			return written, fmt.Errorf("upserting invoice for tenant %s: %w", a.tenant, err)
		}
		written++
	}
	slog.Info("billing: generated invoices", "period_start", start, "period_end", end, "count", written)
	return written, nil
}

// List returns invoices, newest period first. Pass empty tenant for the
// cross-tenant view (admin).
func (g *Generator) List(ctx context.Context, tenant string, limit int) ([]Invoice, error) {
	if limit <= 0 {
		limit = 100
	}
	q := `SELECT id, tenant_id, period_start, period_end, total_amount,
	             task_count, currency, status,
	             COALESCE(external_id, ''),
	             created_at,
	             COALESCE(issued_at, ''),
	             COALESCE(paid_at, '')
	      FROM invoices`
	var args []any
	if tenant != "" {
		q += ` WHERE tenant_id = ?`
		args = append(args, tenant)
	}
	q += ` ORDER BY period_start DESC LIMIT ?`
	args = append(args, limit)

	rows, err := g.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Invoice
	for rows.Next() {
		var inv Invoice
		var start, end, created, issued, paid string
		if err := rows.Scan(&inv.ID, &inv.TenantID, &start, &end,
			&inv.TotalAmount, &inv.TaskCount, &inv.Currency, &inv.Status,
			&inv.ExternalID, &created, &issued, &paid); err != nil {
			return nil, err
		}
		inv.PeriodStart, _ = time.Parse("2006-01-02 15:04:05", start)
		inv.PeriodEnd, _ = time.Parse("2006-01-02 15:04:05", end)
		inv.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		inv.IssuedAt, _ = time.Parse("2006-01-02 15:04:05", issued)
		inv.PaidAt, _ = time.Parse("2006-01-02 15:04:05", paid)
		out = append(out, inv)
	}
	return out, rows.Err()
}

// Issue flips a draft invoice to `issued` and pushes it to the gateway when
// one is configured. Safe to call more than once — the status check + DB
// update is idempotent; a gateway with proper external_id idempotency will
// also absorb repeats.
func (g *Generator) Issue(ctx context.Context, invoiceID string) error {
	var inv Invoice
	var start, end, created, issued, paid string
	err := g.db.QueryRowContext(ctx,
		`SELECT id, tenant_id, period_start, period_end, total_amount,
		        task_count, currency, status, COALESCE(external_id,''),
		        created_at, COALESCE(issued_at,''), COALESCE(paid_at,'')
		 FROM invoices WHERE id = ?`, invoiceID,
	).Scan(&inv.ID, &inv.TenantID, &start, &end, &inv.TotalAmount,
		&inv.TaskCount, &inv.Currency, &inv.Status, &inv.ExternalID,
		&created, &issued, &paid)
	if err == sql.ErrNoRows {
		return fmt.Errorf("invoice %s not found", invoiceID)
	}
	if err != nil {
		return err
	}
	if inv.Status == StatusPaid || inv.Status == StatusVoid {
		return fmt.Errorf("invoice %s is %s — cannot issue", invoiceID, inv.Status)
	}
	inv.PeriodStart, _ = time.Parse("2006-01-02 15:04:05", start)
	inv.PeriodEnd, _ = time.Parse("2006-01-02 15:04:05", end)

	externalID := inv.ExternalID
	if g.gateway != nil {
		externalID, err = g.gateway.Register(ctx, inv)
		if err != nil {
			return fmt.Errorf("gateway register: %w", err)
		}
		slog.Info("billing: pushed to gateway",
			"gateway", g.gateway.Name(), "invoice", invoiceID, "external_id", externalID)
	}
	_, err = g.db.ExecContext(ctx,
		`UPDATE invoices SET status = ?, issued_at = datetime('now'), external_id = ? WHERE id = ?`,
		StatusIssued, externalID, invoiceID)
	return err
}

// MarkPaid records payment. Called from a gateway webhook, or manually by
// an admin for offline payment flows.
func (g *Generator) MarkPaid(ctx context.Context, invoiceID string) error {
	res, err := g.db.ExecContext(ctx,
		`UPDATE invoices SET status = ?, paid_at = datetime('now')
		 WHERE id = ? AND status IN (?, ?)`,
		StatusPaid, invoiceID, StatusIssued, StatusDraft)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("invoice %s not found or not in issued/draft state", invoiceID)
	}
	return nil
}

// GenerateLastMonth runs GenerateForPeriod over the previous calendar month
// in UTC. Intended to be called from a daily cron — it's idempotent, so
// re-running on the 2nd, 3rd, 4th of a month just refreshes the amounts.
func (g *Generator) GenerateLastMonth(ctx context.Context) (int, error) {
	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	prevEnd := monthStart
	prevStart := prevEnd.AddDate(0, -1, 0)
	return g.GenerateForPeriod(ctx, prevStart, prevEnd)
}
