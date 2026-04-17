-- Adversarial review A2 (Postgres variant).
ALTER TABLE budget_alerts   ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT 'default';
ALTER TABLE cluster_members ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT 'default';
CREATE INDEX IF NOT EXISTS idx_budget_alerts_tenant   ON budget_alerts(tenant_id);
CREATE INDEX IF NOT EXISTS idx_cluster_members_tenant ON cluster_members(tenant_id);
