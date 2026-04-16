-- v0.2 Migration: Knowledge, Trust History, Dialog, Webhooks, Costs

CREATE TABLE IF NOT EXISTS knowledge (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_type TEXT NOT NULL,
    approach TEXT NOT NULL,
    outcome TEXT NOT NULL,
    context TEXT,
    embedding BLOB,
    created_at TEXT DEFAULT (datetime('now'))
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
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_trust_history_agent ON trust_history(agent_id);

CREATE TABLE IF NOT EXISTS dialog_threads (
    id TEXT PRIMARY KEY,
    initiator_agent_id TEXT NOT NULL,
    participant_agent_id TEXT NOT NULL,
    topic TEXT NOT NULL,
    status TEXT DEFAULT 'active',
    created_at TEXT DEFAULT (datetime('now')),
    completed_at TEXT
);

CREATE TABLE IF NOT EXISTS dialog_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    thread_id TEXT NOT NULL REFERENCES dialog_threads(id),
    sender_agent_id TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_dialog_messages_thread ON dialog_messages(thread_id);

CREATE TABLE IF NOT EXISTS webhooks (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    url TEXT NOT NULL,
    type TEXT NOT NULL,
    event_filter TEXT,
    enabled INTEGER DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS costs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id TEXT NOT NULL,
    agent_name TEXT NOT NULL,
    workflow_id TEXT NOT NULL,
    task_id TEXT NOT NULL,
    cost REAL NOT NULL,
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_costs_agent ON costs(agent_name);
CREATE INDEX IF NOT EXISTS idx_costs_workflow ON costs(workflow_id);
