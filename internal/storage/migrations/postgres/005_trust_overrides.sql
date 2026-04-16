CREATE TABLE IF NOT EXISTS agent_trust_overrides (
    agent_id   TEXT NOT NULL,
    task_type  TEXT NOT NULL,
    level      TEXT NOT NULL,
    reason     TEXT,
    created_at TEXT DEFAULT (to_char(CURRENT_TIMESTAMP, 'YYYY-MM-DD HH24:MI:SS')),
    PRIMARY KEY (agent_id, task_type)
);
CREATE INDEX IF NOT EXISTS idx_trust_overrides_agent ON agent_trust_overrides(agent_id);
