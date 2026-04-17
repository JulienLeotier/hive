-- Webhook delivery history. Recording every attempt (including retries and
-- failures) lets operators audit integrations and debug flaky downstreams
-- without having to correlate logs manually.
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    webhook_name TEXT NOT NULL,
    event_type TEXT NOT NULL,
    attempt INTEGER NOT NULL,
    status_code INTEGER NOT NULL,
    error_message TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_name ON webhook_deliveries(webhook_name, created_at DESC);
