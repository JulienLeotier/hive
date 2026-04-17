-- Billing infrastructure (Postgres).
ALTER TABLE costs ADD COLUMN IF NOT EXISTS tenant_id TEXT NOT NULL DEFAULT 'default';
CREATE INDEX IF NOT EXISTS idx_costs_tenant ON costs(tenant_id);

CREATE TABLE IF NOT EXISTS invoices (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    total_amount DOUBLE PRECISION NOT NULL DEFAULT 0,
    task_count INTEGER NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'USD',
    status TEXT NOT NULL DEFAULT 'draft',
    external_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    issued_at TIMESTAMPTZ,
    paid_at TIMESTAMPTZ,
    UNIQUE (tenant_id, period_start, period_end)
);
CREATE INDEX IF NOT EXISTS idx_invoices_tenant ON invoices(tenant_id, period_start DESC);
CREATE INDEX IF NOT EXISTS idx_invoices_status ON invoices(status);
