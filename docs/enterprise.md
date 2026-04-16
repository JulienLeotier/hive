# Enterprise Deployment

SSO, RBAC, audit, and multi-tenant wiring for production use.

## Roles

Three built-in roles (`internal/auth/rbac.go`):

| Role       | Permissions                                                    |
|------------|----------------------------------------------------------------|
| `admin`    | `{*, *}` — full access                                         |
| `operator` | read + write on agents, workflows, tasks; read on events       |
| `viewer`   | read-only on agents, workflows, tasks, events                  |

API routes declare their resource/action via `auth.RBACMiddleware("agents", "write")`.
A viewer posting to `POST /api/v1/agents` receives `403 Forbidden`.

## User directory

```bash
hive users add alice@example.com admin
hive users add bob@example.com   operator --tenant acme
hive users list
```

Stored in `rbac_users` (subject, role, tenant_id). The API resolver middleware
(`roleResolver` in `internal/api/server.go`) maps the authenticated API key
name → subject → `(role, tenant_id)` and stashes them in the request context
for `RBACMiddleware` to enforce.

### OIDC

The `subject` field is designed for OIDC `sub` claims. Current v1.0 ships with
API-key auth and expects you to add a user record for each key. A future
release will close the loop by validating a JWT on each request and deriving
`subject` from the token.

## Audit

Every sensitive action calls `audit.Logger.Log(ctx, action, actor, resource, detail)`.
Entries include a `tenant_id` column (default `'default'`) so exports can be
tenant-scoped.

```bash
hive audit list --since 24h
hive audit export --since 30d --format json --output audit.json
hive audit export --format csv --output audit.csv
```

`ExportCSV` prefixes values starting with `=`, `+`, `-`, or `@` with a single
quote so CSV injection into spreadsheets is neutralised.

## Multi-tenant

```bash
hive tenant create acme
hive tenant list
```

- Every core table (`agents`, `tasks`, `workflows`) carries a `tenant_id` column
  with the default `'default'` so existing single-tenant data keeps working.
- The API resolver attaches the authenticated user's tenant to the request
  context via `auth.WithTenant()`; downstream queries can filter rows by it.
- v1.0 isolation is application-layer. If you need hard isolation use
  Postgres Row-Level Security policies (`storage: postgres`).

## Deployment hardening

- Run with `HIVE_STORAGE=postgres` for higher write concurrency.
- Front the server with TLS (Caddy / nginx / ALB).
- Rotate API keys via `hive api-key generate` and revoke old ones with `hive api-key delete`.
- Monitor `decision.*` events to audit why the orchestrator made specific choices.
