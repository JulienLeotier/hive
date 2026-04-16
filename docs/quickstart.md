# Quickstart -- Getting Started in 5 Minutes

Hive orchestrates AI agents from any framework through a standardized open protocol. This guide takes you from zero to a running hive.

## Prerequisites

- Go 1.25+ installed
- Node.js 18+ (for dashboard build)
- Git

## Install from Source

```bash
git clone https://github.com/JulienLeotier/hive.git
cd hive
make build
```

This compiles the Go binary and the Svelte dashboard into a single `./hive` executable. The `Makefile` runs `cd web && npm run build` first, then `go build -o hive ./cmd/hive`.

## Step 1: Scaffold a Project

```bash
./hive init my-project
cd my-project
```

This creates:
- `hive.yaml` -- workflow configuration with an example task
- `agents/` -- directory for agent configs
- `README.md` -- project readme

Use a template for a preconfigured workflow:

```bash
./hive init my-project --template code-review
./hive init my-project --template content-pipeline
./hive init my-project --template research
```

## Step 2: Register an Agent

Register an HTTP-based agent:

```bash
hive add-agent --name my-agent --type http --url http://localhost:8080
```

Supported agent types: `http`, `claude-code`, `mcp`.

On registration, Hive calls the agent's `/health` and `/declare` endpoints to verify connectivity and discover capabilities. The agent is stored in SQLite with an initial trust level of `scripted`.

## Step 3: Start the Server

```bash
hive serve
```

This starts the API server and web dashboard on port 8233 (default). Open `http://localhost:8233` to view the dashboard.

For development with debug logging:

```bash
hive serve --log-level debug
```

## Step 4: Check Status

```bash
hive status
```

Output:

```
NAME                 TYPE       HEALTH       TRUST
----                 ----       ------       -----
my-agent             http       healthy      scripted

Total: 1 agents
```

For JSON output (useful in scripts):

```bash
hive status --json
```

## Step 5: Validate Your Workflow

```bash
hive validate
```

This parses `hive.yaml`, checks task dependencies for cycles via topological sort, and reports the number of tasks and parallel groups.

## Step 6: Query Event Logs

```bash
hive logs
hive logs --type task --since 1h --limit 20
hive logs --agent my-agent --json
```

## Step 7: Remove an Agent

```bash
hive remove-agent my-agent
```

## Example hive.yaml

```yaml
name: my-project
tasks:
  - name: review
    type: code-review
    input:
      source: pr
  - name: summarize
    type: summarize
    depends_on: [review]
    input:
      format: markdown
```

Tasks declare their `type` (a capability name). The task router in `internal/task/router.go` matches tasks to agents that declared that capability via `FindCapableAgent()`.

## What's Next

- [Adapter Guide](adapter-guide.md) -- write a custom adapter
- [Configuration](configuration.md) -- all hive.yaml options
- [Dashboard Guide](dashboard-guide.md) -- using the web dashboard
- [Trust Configuration](trust-configuration.md) -- graduated autonomy
- [CLI Reference](cli-reference.md) -- all commands and flags
