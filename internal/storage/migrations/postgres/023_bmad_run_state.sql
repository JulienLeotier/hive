CREATE TABLE IF NOT EXISTS bmad_phase_steps (
    id BIGSERIAL PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    phase TEXT NOT NULL,
    command TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'running',
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    cost_usd DOUBLE PRECISION NOT NULL DEFAULT 0,
    reply_preview TEXT,
    error_text TEXT
);
CREATE INDEX IF NOT EXISTS idx_phase_steps_project ON bmad_phase_steps(project_id, started_at DESC);

ALTER TABLE projects ADD COLUMN IF NOT EXISTS total_cost_usd DOUBLE PRECISION NOT NULL DEFAULT 0;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS failure_stage TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS failure_error TEXT;
