-- Postgres port of migration 002.

CREATE TABLE IF NOT EXISTS knowledge (
    id BIGSERIAL PRIMARY KEY,
    task_type TEXT NOT NULL,
    approach TEXT NOT NULL,
    outcome TEXT NOT NULL,
    context TEXT,
    embedding BYTEA,
    created_at TEXT DEFAULT (to_char(CURRENT_TIMESTAMP, 'YYYY-MM-DD HH24:MI:SS'))
);
CREATE INDEX IF NOT EXISTS idx_knowledge_task_type ON knowledge(task_type);
CREATE INDEX IF NOT EXISTS idx_knowledge_outcome ON knowledge(outcome);

CREATE TABLE IF NOT EXISTS trust_history (
    id TEXT PRIMARY KEY,
    agent_id TEXT NOT NULL,
    old_level TEXT NOT NULL,
    new_level TEXT NOT NULL,
    reason TEXT NOT NULL,
    criteria TEXT,
    created_at TEXT DEFAULT (to_char(CURRENT_TIMESTAMP, 'YYYY-MM-DD HH24:MI:SS'))
);

CREATE TABLE IF NOT EXISTS dialog_threads (
    id TEXT PRIMARY KEY,
    workflow_id TEXT NOT NULL,
    topic TEXT NOT NULL,
    status TEXT DEFAULT 'open',
    created_at TEXT DEFAULT (to_char(CURRENT_TIMESTAMP, 'YYYY-MM-DD HH24:MI:SS')),
    closed_at TEXT
);

CREATE TABLE IF NOT EXISTS dialog_messages (
    id BIGSERIAL PRIMARY KEY,
    thread_id TEXT NOT NULL,
    from_agent TEXT NOT NULL,
    to_agent TEXT,
    content TEXT NOT NULL,
    created_at TEXT DEFAULT (to_char(CURRENT_TIMESTAMP, 'YYYY-MM-DD HH24:MI:SS'))
);
CREATE INDEX IF NOT EXISTS idx_dialog_messages_thread ON dialog_messages(thread_id);

CREATE TABLE IF NOT EXISTS webhooks (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    url TEXT NOT NULL,
    type TEXT NOT NULL,
    event_filter TEXT,
    enabled INTEGER DEFAULT 1,
    created_at TEXT DEFAULT (to_char(CURRENT_TIMESTAMP, 'YYYY-MM-DD HH24:MI:SS'))
);
