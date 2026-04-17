-- PM intake conversation. The user describes an idea on the /projects
-- form and the PM agent asks clarifying questions until it has enough to
-- produce a PRD. Each agent role the BMAD flow needs (PM, Architect,
-- Reviewer later) gets its own conversation row so we can reopen the PM
-- mid-build to refine scope.
CREATE TABLE IF NOT EXISTS project_conversations (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    role TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_project_conversations_project ON project_conversations(project_id, role);

CREATE TABLE IF NOT EXISTS project_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id TEXT NOT NULL REFERENCES project_conversations(id) ON DELETE CASCADE,
    author TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_project_messages_conv ON project_messages(conversation_id, id);
