-- Agent versioning (Vague 1 — product gaps pass).
-- Separate rows let the same agent_name run at multiple versions in parallel,
-- which is the standard canary / A-B rollout pattern. Existing rows get '1.0.0'
-- so upgrades are non-breaking.
ALTER TABLE agents ADD COLUMN IF NOT EXISTS version TEXT NOT NULL DEFAULT '1.0.0';
CREATE INDEX IF NOT EXISTS idx_agents_name_version ON agents(name, version);
