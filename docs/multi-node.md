# Multi-Node Setup

Run multiple Hive nodes behind a shared PostgreSQL database and a NATS cluster
for horizontal scaling and HA.

## Topology

```
        ┌─ hive node A ─┐
clients ┤─ hive node B ─├── NATS cluster ── PostgreSQL
        └─ hive node C ─┘
```

- PostgreSQL stores agents, tasks, workflows, events history, costs, audit.
- NATS carries real-time events between nodes (subject prefix `hive.events`).
- Each node registers itself in `cluster_members` via `cluster.Roster`.

## Per-node config

```yaml
port: 8233
storage: postgres
postgres_url: postgres://hive:hive@postgres:5432/hive?sslmode=disable
event_bus: nats
nats_url: nats://nats:4222
routing: local-first   # or best-fit
```

Environment overrides: `HIVE_STORAGE`, `HIVE_POSTGRES_URL`, `HIVE_PORT`.

## Cluster roster

```go
roster := cluster.NewRoster(db)
roster.Heartbeat(ctx, manager.Self())
```

- `Heartbeat` is an upsert; call it on a ticker (every 5–10 seconds).
- `MarkStale(maxAge)` moves nodes without a fresh heartbeat to `offline`.
- `Remove(nodeID)` drops a decommissioned node.

## Routing

`cluster.Manager.PickAgent(perNode, taskType)` implements:

- **local-first** (default, matches `routing: local-first` or unset): prefer an
  agent on the current node; fall back to any other node.
- **best-fit**: deterministic round-robin across sorted node IDs.

## PostgreSQL migrations

Parallel set at `internal/storage/migrations/postgres/`. `storage.OpenPostgres`
runs them at startup, tracking applied versions in `schema_versions` just like
the SQLite path.

Known differences:

- `datetime('now')` → `to_char(CURRENT_TIMESTAMP, 'YYYY-MM-DD HH24:MI:SS')`
- `BLOB` → `BYTEA`
- `INTEGER PRIMARY KEY AUTOINCREMENT` → `BIGSERIAL PRIMARY KEY`
- `?` placeholders → `$N`

Some analytical queries (`optimizer.JULIANDAY`) still use SQLite-only
functions; porting them to `EXTRACT(EPOCH FROM …)` is tracked as follow-up.

## Failure behaviour

- A node that dies mid-task: its in-flight rows become stale checkpoints and
  `task.CheckpointSupervisor` reassigns them on the next sweep.
- NATS disconnects: `NATSBus` keeps publishing locally (best-effort); wire
  it to a NATS client configured with reconnect-on-close for full recovery.
- PostgreSQL failover: Hive retries via the standard `database/sql` driver
  pool; there's no custom failover logic — rely on your HA setup.
