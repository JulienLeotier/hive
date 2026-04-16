# Contributing to Hive

Hive is an open-source AI agent orchestration platform. Contributions are welcome.

## Prerequisites

- Go 1.25+
- Node.js 18+ (for dashboard)
- Git

## Project Structure

```
hive/
  cmd/hive/          -- entry point (main.go)
  internal/
    adapter/         -- Agent Adapter Protocol implementations
    agent/           -- agent registration and management
    api/             -- HTTP API server and auth
    audit/           -- audit logging
    auth/            -- RBAC roles and permissions
    autonomy/        -- behavioral plans and heartbeat scheduler
    cli/             -- CLI commands (Cobra)
    cluster/         -- multi-node support (future)
    config/          -- YAML config loading with env overrides
    cost/            -- per-agent cost tracking
    dashboard/       -- embedded Svelte assets
    event/           -- event bus (pub/sub + SQLite persistence)
    federation/      -- cross-hive protocol (future)
    hivehub/         -- template registry (future)
    knowledge/       -- shared knowledge layer
    market/          -- auction-based task allocation (future)
    optimizer/       -- self-optimization analyzer (future)
    resilience/      -- circuit breaker pattern
    storage/         -- SQLite connection and migrations
    task/            -- task state machine and routing
    trust/           -- graduated autonomy engine
    webhook/         -- notification delivery (Slack, GitHub, generic)
    workflow/        -- workflow parser and execution
    ws/              -- WebSocket hub for dashboard
  web/               -- Svelte 5 dashboard source
  templates/         -- project templates (code-review, content-pipeline, research)
  protocol/          -- adapter protocol spec
  docs/              -- documentation
```

## Development Workflow

### Build and Run

```bash
make build          # build dashboard + Go binary
make dev            # run with debug logging (no build)
make serve          # build then serve
make test           # run all tests
make lint           # go vet
make clean          # remove build artifacts
```

### Running Tests

```bash
go test ./... -v -count=1
```

Tests are co-located with source files (`foo.go` + `foo_test.go`). Use `testify` for assertions. Use `httptest` for HTTP handler tests.

### Dashboard Development

```bash
cd web && npm install
cd web && npm run dev   # Vite dev server with hot reload
```

Build for production: `make dashboard` (or `cd web && npm run build`).

## Code Style

### Go Conventions

- **Packages**: lowercase, single word (`agent`, `event`, `task`)
- **Exported types**: PascalCase (`AgentManager`, `EventBus`)
- **Unexported**: camelCase (`handleEvent`, `routeTask`)
- **Interfaces**: verb-noun or -er suffix (`Adapter`, `Router`)
- **Files**: snake_case (`circuit_breaker.go`)
- **Error handling**: wrap with context using `fmt.Errorf("doing X: %w", err)`
- **Logging**: use `log/slog` (structured), never `fmt.Println`
- **IDs**: ULID via `oklog/ulid`

### Database Conventions

- Tables: plural snake_case (`agents`, `events`)
- Columns: snake_case (`agent_id`, `created_at`)
- Indexes: `idx_{table}_{column}`

### API Conventions

- Routes: `/api/v1/{resource}` with plural nouns
- Response envelope: `{"data": ..., "error": null}`
- JSON fields: snake_case
- Timestamps: ISO 8601 UTC

### Event Naming

Dot notation, past tense: `task.created`, `agent.registered`, `workflow.completed`.

## Adding a New Adapter

1. Create `internal/adapter/myframework.go`
2. Implement the `Adapter` interface (5 methods)
3. Add compile-time check: `var _ Adapter = (*MyAdapter)(nil)`
4. Write tests in `internal/adapter/myframework_test.go`
5. See [Adapter Guide](adapter-guide.md) for full details

## Adding a CLI Command

1. Create a new file in `internal/cli/` (e.g., `mycommand.go`)
2. Define a `cobra.Command` variable
3. Register it in an `init()` function with `rootCmd.AddCommand()`
4. All commands should load config from `hive.yaml` and open storage if needed

## Adding a Database Migration

1. Create a new SQL file in `internal/storage/migrations/` with the next version number (e.g., `002_my_change.sql`)
2. Migrations are embedded via `//go:embed *.sql` in `embed.go`
3. They run automatically on startup; versions are tracked in `schema_versions`

## Testing Guidelines

- All exported functions should have tests
- Use table-driven tests for multiple scenarios
- Use `t.TempDir()` for tests that need file I/O
- Use in-memory SQLite for database tests
- Test circuit breaker state transitions explicitly

## Pull Request Process

1. Fork the repository
2. Create a feature branch from `main`
3. Make your changes following the code style above
4. Ensure `make test` and `make lint` pass
5. Write or update tests for your changes
6. Submit a PR with a clear description of what and why

## Architecture Principles

- **Single binary**: everything embeds into one Go binary
- **No external dependencies at runtime**: SQLite, event bus, dashboard all embedded
- **Interface-based boundaries**: enables future component swaps
- **Event sourcing**: all state changes emit events to the bus
- **Package-per-domain**: one package per concept, no `utils` packages
