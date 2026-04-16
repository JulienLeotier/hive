ALTER TABLE tasks ADD COLUMN IF NOT EXISTS checkpoint_at TEXT;
CREATE INDEX IF NOT EXISTS idx_tasks_checkpoint_at ON tasks(checkpoint_at);
