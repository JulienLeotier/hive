-- Story 22.3 AC: router prefers agents on the same node. Adds the node_id
-- column so agents can be tagged with the node that registered them.
ALTER TABLE agents ADD COLUMN node_id TEXT DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_agents_node ON agents(node_id);
