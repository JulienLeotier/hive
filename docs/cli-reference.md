# CLI Reference

The `hive` CLI is built with Cobra (`github.com/spf13/cobra`). All commands support `--help`. Global flag: `--log-level` (default: `info`; values: `debug`, `info`, `warn`, `error`).

## hive init

Scaffold a new project. Source: `internal/cli/init_cmd.go`

```
hive init [project-name] [--template <name>]
```

- `project-name` defaults to `my-hive`
- `--template`: `code-review`, `content-pipeline`, `research`
- Creates `hive.yaml`, `agents/`, and `README.md` in the project directory

```bash
hive init my-project --template code-review
```

## hive add-agent

Register an agent. Source: `internal/cli/agent.go`

```
hive add-agent --name <name> --url <url> [--type <type>]
```

- `--name` (required) -- agent name
- `--url` (required) -- agent URL or path
- `--type` (default: `http`) -- `http`, `claude-code`, `mcp`

Calls `/health` and `/declare` on the agent to verify connectivity. Agent starts at trust level `scripted`.

```bash
hive add-agent --name reviewer --type http --url http://localhost:8080
```

## hive remove-agent

Remove an agent by name. Source: `internal/cli/agent.go`

```bash
hive remove-agent reviewer
```

## hive status

Show agents, health, and trust levels. Source: `internal/cli/agent.go`

```
hive status [--json]
```

Columns: NAME, TYPE, HEALTH, TRUST. Use `--json` for machine-readable output.

```bash
hive status
hive status --json | jq '.[] | select(.health_status == "healthy")'
```

## hive serve

Start the API server and dashboard on port 8233 (default). Source: `internal/cli/serve.go`

```bash
hive serve
HIVE_PORT=9000 hive serve --log-level debug
```

Serves `/api/v1/*` (REST API with optional auth) and `/` (Svelte dashboard). Graceful shutdown on SIGINT/SIGTERM.

## hive logs

Query event logs with filtering. Source: `internal/cli/logs.go`

```
hive logs [--type <prefix>] [--agent <name>] [--since <duration>] [--limit <n>] [--json]
```

- `--type` -- event type prefix (e.g., `task`, `agent.health`)
- `--agent` -- filter by source name
- `--since` -- duration (e.g., `1h`, `30m`)
- `--limit` -- max events (default: 50)

```bash
hive logs --type task --since 1h --limit 20
```

## hive validate

Validate workflow YAML. Source: `internal/cli/validate.go`

```
hive validate [workflow-file]
```

Defaults to `hive.yaml`. Checks syntax, required fields, dependency references, and circular dependencies (Kahn's algorithm). Reports task count and parallel execution levels.

## hive version

Print version, Go version, and OS/arch. Source: `internal/cli/version.go`

```bash
hive version
# hive dev
#   go:   go1.25
#   os:   darwin/arm64
```

## Makefile Targets

| Target | Description |
|---|---|
| `make build` | Build dashboard + Go binary (produces `./hive`) |
| `make test` | Run all tests (`go test ./... -v -count=1`) |
| `make lint` | Static analysis (`go vet ./...`) |
| `make dev` | Run with debug logging |
| `make serve` | Build then serve |
| `make clean` | Remove binary and dashboard dist |
| `make dashboard` | Build Svelte dashboard only |
