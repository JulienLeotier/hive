-- Marketplace publish flag. An agent is only visible in the federated
-- marketplace catalog (GET /api/v1/federation/catalog) when publishable=1,
-- so operators don't accidentally expose internal agents to peers.
ALTER TABLE agents ADD COLUMN publishable INTEGER NOT NULL DEFAULT 0;
