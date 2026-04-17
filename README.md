# Hive

Hive is a **local BMAD product factory**. Describe what you want built,
the PM agent turns it into a PRD, the Architect decomposes into epics
and stories with acceptance criteria, and Claude Code drives a
Dev/Reviewer loop until every AC passes — commits land in your workdir,
no human in the loop. One Go binary, one SvelteKit dashboard, one
database (SQLite by default).

**Flow:** Idea → PM Q&A → PRD → Architect → Dev + Review → Shipped.
Everything is piloted from the web dashboard; there's no CLI for the
user-facing build flow. See `web/src/routes/projects/` for the UI entry
points and `internal/devloop/` for the autonomous loop.

Running the real `claude` CLI end-to-end against a throwaway project:

```bash
# Requires: claude CLI on PATH, go 1.25+, jq, curl.
# HIVE_E2E_TIMEOUT=1200  # seconds; default 20 min
./scripts/claude-e2e.sh
```

The script spins up a temp hive on port 18233, creates a tiny project
("write a CLI that prints a random compliment"), finalises the intake,
lets the real Claude Code adapter drive the dev loop, and verifies the
project flips to `shipped`. For quick adapter-only plumbing checks:
`go test -tags claude_e2e ./internal/devloop`.

---

Below is the legacy README from before the BMAD pivot. Some of this is
stale — federation, marketplace, billing, workflow DAGs, etc. are no
longer part of the product surface. Kept here until it's rewritten.

## Legacy description

Universal AI agent orchestration platform. Register heterogeneous agents
(HTTP, Claude Code, MCP, CrewAI, LangChain, AutoGen, OpenAI), compose
workflows as DAGs (YAML or visual builder), and let Hive route tasks,
track cost, bill tenants, and surface live health — all from a single
Go binary with an embedded dashboard.

**Highlights:** first-run setup wizard · visual drag-drop workflow
builder · federated agent marketplace · SMTP + Slack ops alerts · OTLP
traces · backup/restore CLI · multi-tenant + multi-node (Postgres + NATS)
· Go SDK for integrators.

## Quickstart (Docker)

```bash
git clone https://github.com/JulienLeotier/hive.git
cd hive
docker compose up -d
```

Open <http://localhost:8233>. The first-run wizard walks you through
creating an admin user + API key. See
[docs/getting-started.md](docs/getting-started.md) for the 5-minute
end-to-end tour (register an agent, fire a workflow, wire alerts, back
up).

## Quickstart (from source)

```bash
# Build everything (Go binary + Svelte dashboard baked in)
make build

# Run the server on http://localhost:8233
./hive serve

# In another terminal: register an HTTP agent
./hive add-agent --name writer --type http --url http://localhost:8080

# Submit a workflow
./hive run ./examples/content-pipeline/workflow.yaml
```

On first boot there are no API keys, so auth is bypassed (dev mode).
See [docs/configuration.md](docs/configuration.md) to harden for
production.

## Development

`make dev` runs Vite (HMR on `:5173`) and the Go server (auto-rebuild via
`air` on `:8233`) side by side. Vite proxies `/api` and `/ws` to the Go
backend so you work on one URL (`http://localhost:5173`) with live reload
on both sides.

```bash
make dev         # full-stack loop
make dev-web     # Vite only
make dev-api     # air only
make test        # full Go + integration suite
make lint        # go vet
```

## What's in here

| Area | Path | Notes |
|---|---|---|
| CLI + server | `cmd/hive/`, `internal/cli/` | `hive serve`, `hive run`, `hive backup`, `hive restore`, ... |
| API | `internal/api/` | REST `/api/v1/*`, webhooks `/hooks/*`, WS `/ws`, Prom `/metrics`, health probes |
| Agents | `internal/agent/`, `internal/adapter/` | Registration + adapter protocol (HTTP, Claude Code, MCP, CrewAI, LangChain, AutoGen, OpenAI) |
| Workflows | `internal/workflow/` | YAML DAG parser + execution engine + schedule/webhook triggers |
| Federation | `internal/federation/` | Hive-to-hive with mTLS, hop limits, cert-at-rest encryption |
| Marketplace | `internal/api/marketplace` | Aggregated catalog of agents published by federated peers |
| Billing | `internal/billing/` | Monthly invoice generation from costs, pluggable payment gateway |
| Notify | `internal/notify/` | SMTP + Slack notifiers for ops events (task.failed, cost.alert, agent.isolated) |
| Tracing | `internal/tracing/` | OTLP exporter; spans on HTTP + adapter + workflow |
| SDK | `sdk/` | Public Go client for third-party integrations |
| Storage | `internal/storage/` | SQLite (default) or Postgres via `storage=postgres` |
| Dashboard | `web/`, `internal/dashboard/` | SvelteKit static build, embedded in the binary |

## Production notes

- **TLS.** Set `HIVE_TLS_CERT` + `HIVE_TLS_KEY` (or `tls:` block in
  `hive.yaml`). Without TLS, the server starts in plaintext HTTP —
  fine for dev, not for prod. Security headers (CSP, HSTS-on-TLS,
  X-Frame-Options, X-Content-Type-Options, Referrer-Policy) always ship.
- **Auth.** Generate an API key with `hive api-key generate <name>`, or
  configure OIDC in `hive.yaml` under `oidc:`. When any key exists,
  auth is required on every `/api/v1/*` request.
- **Storage.** SQLite is fine for a single node with low write concurrency.
  Set `storage: postgres` + `postgres_url` for multi-writer workloads.
  See [ADR 0004](docs/adr/0004-sqlite-is-dev-only.md).
- **Tenancy.** Every handler filters by the caller's tenant. See
  [ADR 0003](docs/adr/0003-tenant-isolation-contract.md).
- **Secrets at rest.** Set `HIVE_MASTER_KEY` to envelope-encrypt the
  federation TLS material *and* outbound webhook URLs (which may embed
  bearer tokens). Same key, one `enc:v1:` format across the board.
- **Observability.** Set `OTEL_EXPORTER_OTLP_ENDPOINT` (or
  `observability.traces.endpoint` in `hive.yaml`) to ship traces to any
  OTLP-compatible backend — Jaeger, Tempo, Honeycomb, Grafana Cloud.
  Prometheus scrape lives on `/metrics`.
- **Alerts.** Configure `notifications.email` (SMTP) and/or
  `notifications.slack` (webhook URL) in `hive.yaml` to page on
  `task.failed`, `cost.alert`, and `agent.isolated`. Each event type is
  debounced per 60s by default to avoid storm spam.
- **Retention.** Events, completed tasks, costs, audit are purged on a
  timer. Defaults under `retention:` in `hive.yaml`.
- **Backups.** `hive backup backup.tar.gz` snapshots the SQLite DB via
  VACUUM INTO without blocking writers. Postgres deployments should use
  `pg_dump` instead.

## Deeper docs

- [Getting started](docs/getting-started.md) — 5-minute walkthrough
- [Architecture overview](docs/architecture-overview.md)
- [Configuration reference](docs/configuration.md)
- [CLI reference](docs/cli-reference.md)
- [API reference](docs/api-reference.md)
- [Adapter guide](docs/adapters-guide.md)
- [Tracing / observability](docs/tracing.md)
- [Marketplace](docs/marketplace.md)
- [Notifications](docs/notifications.md)
- [Federation](docs/federation.md) · [Multi-node](docs/multi-node.md)
- [Cost management](docs/cost-management.md) · [Optimization](docs/optimization.md)
- [Knowledge layer](docs/knowledge-layer.md) · [Trust levels](docs/trust-configuration.md)
- [NFR assessment](docs/nfr-assessment-2026-04-17.md) · [Adversarial review](docs/architecture-adversarial-review-2026-04-17.md)
- [ADRs](docs/adr/README.md)

## License

See [LICENSE](LICENSE) (or ping the maintainer if missing).
