-- Marketplace publish flag (Postgres).
ALTER TABLE agents ADD COLUMN IF NOT EXISTS publishable INTEGER NOT NULL DEFAULT 0;
