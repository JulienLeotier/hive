-- BMAD primitives (Postgres).
CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    idea TEXT NOT NULL,
    prd TEXT,
    workdir TEXT,
    status TEXT NOT NULL DEFAULT 'draft',
    tenant_id TEXT NOT NULL DEFAULT 'default',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_projects_status ON projects(status);
CREATE INDEX IF NOT EXISTS idx_projects_tenant ON projects(tenant_id);

CREATE TABLE IF NOT EXISTS epics (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT,
    ordering INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_epics_project ON epics(project_id, ordering);

CREATE TABLE IF NOT EXISTS stories (
    id TEXT PRIMARY KEY,
    epic_id TEXT NOT NULL REFERENCES epics(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT,
    ordering INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending',
    iterations INTEGER NOT NULL DEFAULT 0,
    agent_id TEXT,
    branch TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_stories_epic ON stories(epic_id, ordering);
CREATE INDEX IF NOT EXISTS idx_stories_status ON stories(status);

CREATE TABLE IF NOT EXISTS acceptance_criteria (
    id BIGSERIAL PRIMARY KEY,
    story_id TEXT NOT NULL REFERENCES stories(id) ON DELETE CASCADE,
    ordering INTEGER NOT NULL DEFAULT 0,
    text TEXT NOT NULL,
    passed INTEGER NOT NULL DEFAULT 0,
    verified_at TIMESTAMPTZ,
    verified_by TEXT
);
CREATE INDEX IF NOT EXISTS idx_ac_story ON acceptance_criteria(story_id, ordering);

CREATE TABLE IF NOT EXISTS reviews (
    id BIGSERIAL PRIMARY KEY,
    story_id TEXT NOT NULL REFERENCES stories(id) ON DELETE CASCADE,
    iteration INTEGER NOT NULL,
    reviewer_agent_id TEXT,
    verdict TEXT NOT NULL,
    feedback TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_reviews_story ON reviews(story_id, iteration DESC);
