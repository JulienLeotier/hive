-- Persistance de l'état de chaque invocation de skill BMAD. Sert à :
--   - montrer la progression en temps réel dans le dashboard
--     (quelle skill tourne, combien en reste, token count courant)
--   - accumuler le coût total par projet
--   - permettre un retry ciblé sur la dernière étape qui a planté
CREATE TABLE IF NOT EXISTS bmad_phase_steps (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    phase TEXT NOT NULL,                   -- planning | iteration | story:<story_id> | review:<story_id> | retrospective
    command TEXT NOT NULL,                 -- /bmad-create-prd etc.
    started_at TEXT NOT NULL DEFAULT (datetime('now')),
    finished_at TEXT,
    status TEXT NOT NULL DEFAULT 'running', -- running | done | failed
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    cost_usd REAL NOT NULL DEFAULT 0,
    reply_preview TEXT,                    -- 600 premiers caractères pour le dashboard
    error_text TEXT
);
CREATE INDEX IF NOT EXISTS idx_phase_steps_project ON bmad_phase_steps(project_id, started_at DESC);

-- Colonnes projet dérivées des steps
ALTER TABLE projects ADD COLUMN total_cost_usd REAL NOT NULL DEFAULT 0;
ALTER TABLE projects ADD COLUMN failure_stage TEXT;
ALTER TABLE projects ADD COLUMN failure_error TEXT;
