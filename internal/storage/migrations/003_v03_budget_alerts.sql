-- v0.3 Migration: Budget Alerts

CREATE TABLE IF NOT EXISTS budget_alerts (
    id TEXT PRIMARY KEY,
    agent_name TEXT NOT NULL,
    daily_limit REAL NOT NULL,
    enabled INTEGER DEFAULT 1,
    last_alerted_date TEXT,
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_budget_alerts_agent ON budget_alerts(agent_name);
