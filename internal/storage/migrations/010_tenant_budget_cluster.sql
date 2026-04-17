-- Adversarial review A2: budget_alerts and cluster_members lacked tenant_id,
-- letting any tenant read budgets or cluster topology set by another tenant.
-- Add tenant_id with the same 'default' backfill convention as migration 006.
ALTER TABLE budget_alerts   ADD COLUMN tenant_id TEXT DEFAULT 'default';
ALTER TABLE cluster_members ADD COLUMN tenant_id TEXT DEFAULT 'default';
CREATE INDEX IF NOT EXISTS idx_budget_alerts_tenant   ON budget_alerts(tenant_id);
CREATE INDEX IF NOT EXISTS idx_cluster_members_tenant ON cluster_members(tenant_id);
