-- Webhook delivery history — ops-debugging log for outbound webhook attempts.
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id BIGSERIAL PRIMARY KEY,
    webhook_name TEXT NOT NULL,
    event_type TEXT NOT NULL,
    attempt INTEGER NOT NULL,
    status_code INTEGER NOT NULL,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_name ON webhook_deliveries(webhook_name, created_at DESC);
