ALTER TABLE agents ADD COLUMN IF NOT EXISTS node_id TEXT DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_agents_node ON agents(node_id);
