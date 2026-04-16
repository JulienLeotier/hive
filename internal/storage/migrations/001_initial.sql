CREATE TABLE IF NOT EXISTS schema_versions (
    version INTEGER PRIMARY KEY,
    applied_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS agents (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    type TEXT NOT NULL,
    config TEXT NOT NULL DEFAULT '{}',
    capabilities TEXT NOT NULL DEFAULT '{}',
    plan TEXT,
    health_status TEXT DEFAULT 'unknown',
    trust_level TEXT DEFAULT 'scripted',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL,
    source TEXT NOT NULL,
    payload TEXT NOT NULL DEFAULT '{}',
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(type);
CREATE INDEX IF NOT EXISTS idx_events_source ON events(source);
CREATE INDEX IF NOT EXISTS idx_events_created ON events(created_at);

CREATE TABLE IF NOT EXISTS tasks (
    id TEXT PRIMARY KEY,
    workflow_id TEXT NOT NULL,
    type TEXT NOT NULL,
    status TEXT DEFAULT 'pending',
    agent_id TEXT,
    input TEXT NOT NULL DEFAULT '{}',
    output TEXT,
    checkpoint TEXT,
    depends_on TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    started_at TEXT,
    completed_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_workflow ON tasks(workflow_id);

CREATE TABLE IF NOT EXISTS workflows (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    config TEXT NOT NULL DEFAULT '{}',
    status TEXT DEFAULT 'idle',
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS api_keys (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    key_hash TEXT NOT NULL,
    key_prefix TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys(key_prefix);
