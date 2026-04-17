-- BMAD primitives. The product pivot from orchestration platform to
-- autonomous product factory hinges on these tables. Each user idea becomes
-- a project. The PM agent turns it into a PRD. The Architect agent
-- decomposes the PRD into epics and stories. The Dev and Reviewer agents
-- iterate on each story until its acceptance criteria pass. Everything
-- else (workflows, federation, marketplace, billing) stays wired but is
-- cosmetically hidden in BMAD mode.

CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    idea TEXT NOT NULL,                       -- raw one-liner the user typed
    prd TEXT,                                 -- structured PRD produced by the PM agent
    workdir TEXT,                             -- local filesystem path the build happens in
    status TEXT NOT NULL DEFAULT 'draft',     -- draft | planning | building | review | shipped | failed
    tenant_id TEXT NOT NULL DEFAULT 'default',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_projects_status ON projects(status);
CREATE INDEX IF NOT EXISTS idx_projects_tenant ON projects(tenant_id);

CREATE TABLE IF NOT EXISTS epics (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT,
    ordering INTEGER NOT NULL DEFAULT 0,       -- execution order within the project
    status TEXT NOT NULL DEFAULT 'pending',    -- pending | in_progress | done | blocked
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_epics_project ON epics(project_id, ordering);

CREATE TABLE IF NOT EXISTS stories (
    id TEXT PRIMARY KEY,
    epic_id TEXT NOT NULL REFERENCES epics(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT,
    ordering INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending',    -- pending | dev | review | qa | done | blocked
    iterations INTEGER NOT NULL DEFAULT 0,     -- how many dev/review rounds ran
    agent_id TEXT,                             -- currently-assigned dev agent
    branch TEXT,                               -- git branch for this story
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_stories_epic ON stories(epic_id, ordering);
CREATE INDEX IF NOT EXISTS idx_stories_status ON stories(status);

-- Acceptance criteria are what makes BMAD deterministic: a story is done
-- when every AC passes. Separate table (rather than a JSON blob on stories)
-- so we can ORDER, INDEX, and compute pass rates without parsing.
CREATE TABLE IF NOT EXISTS acceptance_criteria (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    story_id TEXT NOT NULL REFERENCES stories(id) ON DELETE CASCADE,
    ordering INTEGER NOT NULL DEFAULT 0,
    text TEXT NOT NULL,
    passed INTEGER NOT NULL DEFAULT 0,         -- 0 = unmet, 1 = met
    verified_at TEXT,
    verified_by TEXT                           -- agent id (reviewer) that flipped it
);
CREATE INDEX IF NOT EXISTS idx_ac_story ON acceptance_criteria(story_id, ordering);

-- Review records track each dev→reviewer cycle so we can see why stories
-- iterated and what feedback drove each retry.
CREATE TABLE IF NOT EXISTS reviews (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    story_id TEXT NOT NULL REFERENCES stories(id) ON DELETE CASCADE,
    iteration INTEGER NOT NULL,
    reviewer_agent_id TEXT,
    verdict TEXT NOT NULL,                     -- pass | fail | needs_changes
    feedback TEXT,
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_reviews_story ON reviews(story_id, iteration DESC);
