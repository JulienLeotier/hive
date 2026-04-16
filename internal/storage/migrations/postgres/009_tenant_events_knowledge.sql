ALTER TABLE events ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT 'default';
ALTER TABLE knowledge ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT 'default';
CREATE INDEX IF NOT EXISTS idx_events_tenant ON events(tenant_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_tenant ON knowledge(tenant_id);
