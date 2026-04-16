CREATE TABLE IF NOT EXISTS auctions (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL,
    strategy TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open',
    winner_bid_id TEXT,
    opened_at TEXT DEFAULT (to_char(CURRENT_TIMESTAMP, 'YYYY-MM-DD HH24:MI:SS')),
    closed_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_auctions_task ON auctions(task_id);
CREATE INDEX IF NOT EXISTS idx_auctions_status ON auctions(status);

CREATE TABLE IF NOT EXISTS bids (
    id TEXT PRIMARY KEY,
    auction_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    agent_name TEXT NOT NULL,
    price REAL NOT NULL,
    est_duration_ms BIGINT NOT NULL,
    reputation REAL NOT NULL,
    won INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (to_char(CURRENT_TIMESTAMP, 'YYYY-MM-DD HH24:MI:SS'))
);
CREATE INDEX IF NOT EXISTS idx_bids_auction ON bids(auction_id);

CREATE TABLE IF NOT EXISTS agent_tokens (
    agent_name TEXT PRIMARY KEY,
    balance REAL NOT NULL DEFAULT 0,
    updated_at TEXT DEFAULT (to_char(CURRENT_TIMESTAMP, 'YYYY-MM-DD HH24:MI:SS'))
);

CREATE TABLE IF NOT EXISTS federation_links (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    url TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    shared_caps TEXT NOT NULL DEFAULT '[]',
    ca_cert TEXT,
    client_cert TEXT,
    client_key TEXT,
    last_heartbeat TEXT,
    created_at TEXT DEFAULT (to_char(CURRENT_TIMESTAMP, 'YYYY-MM-DD HH24:MI:SS'))
);

CREATE TABLE IF NOT EXISTS audit_log (
    id BIGSERIAL PRIMARY KEY,
    action TEXT NOT NULL,
    actor TEXT NOT NULL,
    resource TEXT NOT NULL,
    detail TEXT,
    tenant_id TEXT DEFAULT 'default',
    created_at TEXT DEFAULT (to_char(CURRENT_TIMESTAMP, 'YYYY-MM-DD HH24:MI:SS'))
);
CREATE INDEX IF NOT EXISTS idx_audit_actor ON audit_log(actor);
CREATE INDEX IF NOT EXISTS idx_audit_tenant ON audit_log(tenant_id);

CREATE TABLE IF NOT EXISTS rbac_users (
    subject TEXT PRIMARY KEY,
    role TEXT NOT NULL,
    tenant_id TEXT NOT NULL DEFAULT 'default',
    created_at TEXT DEFAULT (to_char(CURRENT_TIMESTAMP, 'YYYY-MM-DD HH24:MI:SS'))
);

ALTER TABLE agents    ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT 'default';
ALTER TABLE tasks     ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT 'default';
ALTER TABLE workflows ADD COLUMN IF NOT EXISTS tenant_id TEXT DEFAULT 'default';
CREATE INDEX IF NOT EXISTS idx_agents_tenant ON agents(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tasks_tenant ON tasks(tenant_id);

CREATE TABLE IF NOT EXISTS cluster_members (
    node_id TEXT PRIMARY KEY,
    hostname TEXT NOT NULL,
    address TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    last_heartbeat TEXT DEFAULT (to_char(CURRENT_TIMESTAMP, 'YYYY-MM-DD HH24:MI:SS'))
);
