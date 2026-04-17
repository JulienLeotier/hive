-- Retention janitor DELETE hot paths (see SQLite variant for rationale).
CREATE INDEX IF NOT EXISTS idx_costs_created     ON costs(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_log_created ON audit_log(created_at);
CREATE INDEX IF NOT EXISTS idx_tasks_completed   ON tasks(completed_at);
