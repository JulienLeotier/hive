CREATE TABLE IF NOT EXISTS optimizations (
    id TEXT PRIMARY KEY,
    setting    TEXT NOT NULL,
    old_value  REAL,
    new_value  REAL,
    rationale  TEXT,
    baseline   TEXT,
    applied_at TEXT DEFAULT (to_char(CURRENT_TIMESTAMP, 'YYYY-MM-DD HH24:MI:SS')),
    tenant_id  TEXT DEFAULT 'default'
);
CREATE INDEX IF NOT EXISTS idx_optimizations_setting ON optimizations(setting);
CREATE INDEX IF NOT EXISTS idx_optimizations_applied ON optimizations(applied_at);
