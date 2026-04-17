-- Retention janitor runs periodic DELETE FROM costs/audit_log WHERE
-- created_at < ?. Without an index on created_at the delete is a full table
-- scan — fine at 10k rows, painful at millions. events already has
-- idx_events_created from migration 001. tasks has idx_tasks_status plus
-- completed_at filter — small follow-up: add completed_at index too.
CREATE INDEX IF NOT EXISTS idx_costs_created     ON costs(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_log_created ON audit_log(created_at);
CREATE INDEX IF NOT EXISTS idx_tasks_completed   ON tasks(completed_at);
