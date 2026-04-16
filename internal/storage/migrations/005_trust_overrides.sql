-- Story 9.3 — Per-task-type trust level overrides.
-- An agent may be promoted to "autonomous" for code-review while staying "guided" for deploy.
CREATE TABLE IF NOT EXISTS agent_trust_overrides (
    agent_id   TEXT NOT NULL,
    task_type  TEXT NOT NULL,
    level      TEXT NOT NULL,
    reason     TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (agent_id, task_type)
);
CREATE INDEX IF NOT EXISTS idx_trust_overrides_agent ON agent_trust_overrides(agent_id);
