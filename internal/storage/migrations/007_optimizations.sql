-- Story 23.1 AC: v1.0 migration creates an `optimizations` table for persisted
-- tuning decisions so baselines can survive restarts.
CREATE TABLE IF NOT EXISTS optimizations (
    id TEXT PRIMARY KEY,
    setting     TEXT NOT NULL,
    old_value   REAL,
    new_value   REAL,
    rationale   TEXT,
    baseline    TEXT, -- JSON blob of the TrendSnapshot at apply time
    applied_at  TEXT DEFAULT (datetime('now')),
    tenant_id   TEXT DEFAULT 'default'
);
CREATE INDEX IF NOT EXISTS idx_optimizations_setting ON optimizations(setting);
CREATE INDEX IF NOT EXISTS idx_optimizations_applied ON optimizations(applied_at);
