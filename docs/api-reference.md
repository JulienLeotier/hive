# API Reference

The Hive API runs at `http://<host>:<port>/api/v1/*` (default port `8233`).

## Authentication

All routes expect `Authorization: Bearer <api-key>` unless no API keys have
been configured (dev mode). Generate keys via `hive api-key generate --name`;
only the bcrypt hash is persisted.

## Roles

Each route declares a `(resource, action)` pair checked by
`auth.RBACMiddleware` against the role resolved from the key's user record in
`rbac_users` (see [enterprise.md](enterprise.md)).

| Role       | Can call                                                  |
|------------|-----------------------------------------------------------|
| `admin`    | everything                                                |
| `operator` | GETs + POST /agents                                       |
| `viewer`   | GETs only                                                 |

## Endpoints

### `GET /api/v1/agents`

Returns every agent with its health, trust level, and capability JSON.

```json
{"data":[{"id":"…","name":"reviewer","type":"http","health_status":"healthy", "trust_level":"guided", …}],"error":null}
```

### `POST /api/v1/agents` (operator+)

Placeholder write endpoint. Registration still flows through the CLI; this
exists so the protected write path is exercisable end-to-end.

```json
{"data":{"status":"accepted"},"error":null}
```

### `GET /api/v1/events`

Query params:

- `type` — event-type prefix (e.g., `task`)
- `source` — exact match
- `since` — RFC 3339 timestamp

Returns up to 50 events, newest first.

### `GET /api/v1/tasks`

Tasks with agent names joined; used by the dashboard to group by workflow.

### `GET /api/v1/costs`

```json
{
  "data": {
    "summaries": [{"agent_name":"reviewer","total_cost":12.34,"task_count":42}],
    "alerts":    [{"agent_name":"reviewer","daily_limit":5,"spend":7.2,"breached":true}]
  },
  "error": null
}
```

### `GET /api/v1/metrics`

Counts by status + event throughput + breaker states.

```json
{
  "data": {
    "agents":           {"total":3,"healthy":2,"degraded":0,"unavailable":1},
    "circuit_breakers": {"total":3,"open":1},
    "tasks":            {"pending":2,"running":1,"completed":39},
    "workflows":        {"idle":1,"completed":12},
    "events":           {"last_minute":8,"last_hour":240},
    "timestamp":        "2026-04-16T20:10:00Z"
  }
}
```

### `GET /ws`

WebSocket endpoint broadcasting every persisted event as JSON. Used by the
dashboard for real-time updates.

## Response envelope

All JSON responses wrap data in:

```json
{"data": <payload>, "error": {"code": "...", "message": "..."} }
```

On success `error` is `null`; on error `data` is `null` and the HTTP status
code is non-2xx.

## Event types

Canonical list (see `internal/event/types.go`):

```
agent.registered / agent.removed / agent.health.up / agent.health.down
agent.isolated / agent.circuit_open / agent.idle / agent.wakeup

task.created / task.assigned / task.self_assigned / task.started
task.completed / task.failed / task.retry / task.failover
task.unroutable / task.federated / task.auction.won / task.skipped

workflow.started / workflow.completed / workflow.failed

decision.task_routed / decision.trust_promoted / decision.retry_attempt

cost.alert
```
