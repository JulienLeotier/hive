# Hive

Universal AI agent orchestration platform. Register heterogeneous agents
(HTTP, MCP, LangChain, AutoGen, custom), define workflows as DAGs, and let
Hive route tasks, track cost, and surface live health — all from a single
Go binary with an embedded dashboard.

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
| CLI + server | `cmd/hive/`, `internal/cli/` | `hive serve`, `hive add-agent`, ... |
| API | `internal/api/` | REST `/api/v1/*`, WS `/ws`, Prom `/metrics`, `/healthz`, `/readyz` |
| Agents | `internal/agent/`, `internal/adapter/` | Registration + adapter protocol (HTTP, MCP, LangChain, AutoGen) |
| Workflows | `internal/workflow/` | YAML DAG parser + execution engine |
| Federation | `internal/federation/` | Hive-to-hive with mTLS, hop limits, cert-at-rest encryption |
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
- **Federation certs at rest.** Set `HIVE_MASTER_KEY` to envelope-encrypt
  the TLS material stored for federated peers.
- **Retention.** Events, completed tasks, costs, audit are purged on a
  timer. Defaults under `retention:` in `hive.yaml`.

## Deeper docs

- [Architecture overview](docs/architecture-overview.md)
- [Configuration reference](docs/configuration.md)
- [CLI reference](docs/cli-reference.md)
- [API reference](docs/api-reference.md)
- [Adapter guide](docs/adapters-guide.md)
- [Federation](docs/federation.md) · [Multi-node](docs/multi-node.md)
- [Cost management](docs/cost-management.md) · [Optimization](docs/optimization.md)
- [Knowledge layer](docs/knowledge-layer.md) · [Trust levels](docs/trust-configuration.md)
- [NFR assessment](docs/nfr-assessment-2026-04-17.md) · [Adversarial review](docs/architecture-adversarial-review-2026-04-17.md)
- [ADRs](docs/adr/README.md)

## License

See [LICENSE](LICENSE) (or ping the maintainer if missing).
