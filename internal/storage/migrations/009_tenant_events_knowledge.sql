-- Story 21.4 AC: "each tenant has isolated: agents, workflows, tasks, events,
-- knowledge". Adds tenant_id to events + knowledge tables.
ALTER TABLE events ADD COLUMN tenant_id TEXT DEFAULT 'default';
ALTER TABLE knowledge ADD COLUMN tenant_id TEXT DEFAULT 'default';
CREATE INDEX IF NOT EXISTS idx_events_tenant ON events(tenant_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_tenant ON knowledge(tenant_id);
