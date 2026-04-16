# CLI Reference

The `hive` CLI is built with Cobra (`github.com/spf13/cobra`). All commands support `--help`.

## Global Flags

| Flag | Default | Description |
|---|---|---|
| `--log-level` | `info` | Log level: `debug`, `info`, `warn`, `error` |

Logging is configured via `slog.TextHandler` writing to stderr.

## Commands

### hive init

Scaffold a new Hive project.

```
hive init [project-name] [flags]
```

**Arguments:**
- `project-name` (optional, default: `my-hive`)

**Flags:**
- `--template string` -- Project template: `code-review`, `content-pipeline`, `research`

**Creates:**
- `<project-name>/hive.yaml` -- workflow configuration
- `<project-name>/agents/` -- agent configs directory
- `<project-name>/README.md` -- getting started

**Examples:**
```bash
hive init my-project
hive init my-project --template code-review
hive init  # creates "my-hive" directory
```

**Source:** `internal/cli/init_cmd.go`

---

### hive add-agent

Register an agent with the hive.

```
hive add-agent [flags]
```

**Flags:**
- `--name string` (required) -- Agent name
- `--type string` (default: `http`) -- Agent type: `http`, `claude-code`, `mcp`
- `--url string` (required) -- Agent URL or path

On registration, Hive calls the agent's `/health` and `/declare` endpoints. The agent must be running and reachable.

**Examples:**
```bash
hive add-agent --name reviewer --type http --url http://localhost:8080
hive add-agent --name coder --type claude-code --url ./skills/code-skill
hive add-agent --name tools --type mcp --url http://localhost:3000
```

**Source:** `internal/cli/agent.go`

---

### hive remove-agent

Remove an agent from the hive.

```
hive remove-agent [name]
```

**Arguments:**
- `name` (required) -- Name of the agent to remove

**Examples:**
```bash
hive remove-agent reviewer
```

**Source:** `internal/cli/agent.go`

---

### hive status

Show hive status -- agents, health, and trust levels.

```
hive status [flags]
```

**Flags:**
- `--json` -- Output in JSON format

**Output columns:** NAME, TYPE, HEALTH, TRUST

**Examples:**
```bash
hive status
hive status --json
hive status --json | jq '.[] | select(.health_status == "healthy")'
```

**Source:** `internal/cli/agent.go`

---

### hive serve

Start the API server and web dashboard.

```
hive serve
```

Starts an HTTP server on the port configured in `hive.yaml` (default: 8233). Serves:
- `/api/v1/*` -- REST API (authenticated if API keys exist)
- `/` -- Web dashboard (Svelte SPA, no auth)

Supports graceful shutdown via SIGINT/SIGTERM.

**Examples:**
```bash
hive serve
hive serve --log-level debug
HIVE_PORT=9000 hive serve
```

**Source:** `internal/cli/serve.go`

---

### hive logs

Query event logs with filtering.

```
hive logs [flags]
```

**Flags:**
- `--type string` -- Filter by event type prefix (e.g., `task`, `agent.health`)
- `--agent string` -- Filter by agent/source name
- `--since string` -- Show events since duration (e.g., `1h`, `30m`, `24h`)
- `--limit int` (default: `50`) -- Maximum events to return
- `--json` -- Output in JSON format

**Examples:**
```bash
hive logs
hive logs --type task --since 1h
hive logs --agent reviewer --limit 100
hive logs --json
```

**Source:** `internal/cli/logs.go`

---

### hive validate

Validate workflow configuration.

```
hive validate [workflow-file]
```

**Arguments:**
- `workflow-file` (optional, default: `hive.yaml`)

Checks:
- YAML syntax
- Required fields (name, tasks, task names, task types)
- Dependency references exist
- No self-dependencies or circular dependencies
- Reports task count and parallel execution levels

**Examples:**
```bash
hive validate
hive validate custom-workflow.yaml
```

**Source:** `internal/cli/validate.go`

---

### hive version

Print version information.

```
hive version
```

Output includes Hive version (set via ldflags at build time), Go version, and OS/architecture.

**Examples:**
```bash
hive version
# hive v0.1.0
#   go:   go1.25
#   os:   darwin/arm64
```

**Source:** `internal/cli/version.go`

---

## Build Commands (Makefile)

| Target | Command | Description |
|---|---|---|
| `make build` | Build dashboard + Go binary | Produces `./hive` |
| `make test` | `go test ./... -v -count=1` | Run all tests |
| `make lint` | `go vet ./...` | Static analysis |
| `make dev` | `go run ./cmd/hive --log-level debug` | Run in dev mode |
| `make serve` | Build then `./hive serve` | Build and serve |
| `make clean` | Remove binary and dashboard dist | Clean build artifacts |
| `make dashboard` | `cd web && npm run build` | Build Svelte dashboard only |
