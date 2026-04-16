-- Story 2.6 — Periodic checkpoints + stale task reassignment.
-- Adds a timestamp so a supervisor can detect tasks whose checkpoint has gone stale.
ALTER TABLE tasks ADD COLUMN checkpoint_at TEXT;
CREATE INDEX IF NOT EXISTS idx_tasks_checkpoint_at ON tasks(checkpoint_at);
