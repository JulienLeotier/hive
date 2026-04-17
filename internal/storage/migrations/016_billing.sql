-- Billing infrastructure. Two pieces:
-- 1. costs.tenant_id: without it, we cannot aggregate spend per tenant.
--    Existing single-tenant hives get backfilled with 'default'.
-- 2. invoices: one row per (tenant, billing period). Aggregated from costs
--    by a monthly cron. Status flow is draft then issued then paid. A
--    payment gateway can plug in by reading issued rows and flipping them
--    to paid when the remote payment settles.
ALTER TABLE costs ADD COLUMN tenant_id TEXT NOT NULL DEFAULT 'default';
CREATE INDEX IF NOT EXISTS idx_costs_tenant ON costs(tenant_id);

CREATE TABLE IF NOT EXISTS invoices (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    period_start TEXT NOT NULL,
    period_end TEXT NOT NULL,
    total_amount REAL NOT NULL DEFAULT 0,
    task_count INTEGER NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'USD',
    status TEXT NOT NULL DEFAULT 'draft',
    external_id TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    issued_at TEXT,
    paid_at TEXT,
    UNIQUE (tenant_id, period_start, period_end)
);
CREATE INDEX IF NOT EXISTS idx_invoices_tenant ON invoices(tenant_id, period_start DESC);
CREATE INDEX IF NOT EXISTS idx_invoices_status ON invoices(status);
