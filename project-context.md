# Hive Project Context

> LLM-optimized project reference. Updated 2026-04-16.

## 1. Project Overview

Hive is an open-source AI agent orchestration platform that coordinates agents from any framework (Claude Code, CrewAI, LangChain, AutoGen, MCP servers, OpenAI Assistants, generic HTTP) through a standardized **Agent Adapter Protocol**. It ships as a single Go binary with embedded SQLite, an in-process event bus, and an embedded SvelteKit dashboard. Agents carry behavioral plans (YAML state machines) defining autonomous observe-decide-act cycles, with graduated trust earned through track record.

## 2. Tech Stack

| Component | Version/Library |
|---|---|
| Language | Go 1.25.0 |
| CLI framework | spf13/cobra v1.10.2 |
| Database | modernc.org/sqlite v1.48.2 (pure-Go, no CGO) |
| IDs | oklog/ulid/v2 v2.1.1 |
| WebSocket | gorilla/websocket v1.5.3 |
| Config/workflow | gopkg.in/yaml.v3 v3.0.1 |
| Auth | golang.org/x/crypto v0.50.0 (bcrypt) |
| Testing | stretchr/testify v1.11.1 |
| Dashboard | SvelteKit (web/ directory, embedded via `go:embed`) |

**Zero external runtime dependencies.** No Docker, no message broker, no external DB.

## 3. Project Structure

```
cmd/hive/main.go              Entry point, calls cli.Execute()
internal/
  cli/                         Cobra commands (root, serve, add-agent, remove-agent, status, logs, validate, init, version)
  config/                      YAML config + HIVE_* env overrides (port 8233, data ~/.hive/data)
  storage/                     SQLite open/close, WAL mode, auto-migration
    migrations/                Embedded SQL files (001_initial.sql)
  adapter/                     Agent Adapter Protocol interface + implementations
    adapter.go                 Core Adapter interface (5 methods: Declare, Invoke, Health, Checkpoint, Resume)
    http.go                    HTTPAdapter — generic HTTP/JSON agents
    claude_code.go             ClaudeCodeAdapter — invokes `claude` CLI via subprocess
    mcp.go                     MCPAdapter — Model Context Protocol servers
    crewai.go                  CrewAIAdapter — Python subprocess
    langchain.go               LangChainAdapter — LangServe HTTP
    autogen.go                 AutoGenAdapter — Microsoft AutoGen HTTP
    openai.go                  OpenAIAdapter — OpenAI Assistants API (threads/runs)
  agent/                       Agent struct + Manager (CRUD on agents table)
  event/                       In-process event bus, SQLite-persisted, pub/sub with prefix matching
  task/                        Task state machine (pending->assigned->running->completed/failed) + Router
  workflow/                    YAML parser, DAG validation (Kahn's algorithm), topological sort
  autonomy/                    Behavioral plans (AGENT.yaml + PLAN.yaml), heartbeat scheduler
  trust/                       Graduated trust engine (supervised->guided->autonomous->trusted)
  resilience/                  Circuit breaker (closed->open->half-open), BreakerRegistry
  knowledge/                   Shared knowledge layer — keyword search with recency decay
  cost/                        Per-agent/task cost tracking
  webhook/                     Outbound webhook dispatcher (Slack, GitHub, generic) with SSRF protection
  ws/                          WebSocket hub for real-time event broadcast to dashboard
  dashboard/                   Embedded SvelteKit SPA (go:embed dist/*)
  api/                         HTTP API server + Bearer token auth (bcrypt hashed, prefix-indexed)
  auth/                        RBAC (admin/operator/viewer roles, fail-closed)
  audit/                       Compliance audit logger with CSV injection protection
  market/                      Market-based task auction (lowest-cost, fastest, best-reputation strategies)
  optimizer/                   Historical execution analyzer (slow agents, idle agents, parallel opportunities)
  hivehub/                     HiveHub template registry client
  cluster/                     Multi-node cluster config (NATS, Postgres, local-first routing) — scaffolded
  federation/                  Cross-hive federation (capability discovery, shared capabilities) — scaffolded
web/                           SvelteKit dashboard source (built via `npm run build`, output to internal/dashboard/dist/)
```

## 4. Key Patterns

**Naming:** Standard Go — unexported helpers, exported public API. Package names are singular nouns (`agent`, `event`, `task`). Types named after their concept (`Manager`, `Store`, `Bus`, `Router`, `Engine`).

**IDs:** ULIDs everywhere (`oklog/ulid/v2` with `crypto/rand`). Sortable, no collisions.

**Error handling:** `fmt.Errorf("context: %w", err)` wrapping consistently. Functions return `(value, error)`. Methods log with `slog` at appropriate levels. Panic recovery in event subscriber delivery.

**Database access:** Direct `database/sql` — no ORM. Parameterized queries everywhere. Timestamps stored as `TEXT` in `datetime('now')` format, parsed with `time.Parse("2006-01-02 15:04:05", ...)`. `COALESCE` for nullable columns.

**Concurrency:** `sync.Mutex` / `sync.RWMutex` for shared state. WebSocket write mutex per client. Event delivery is synchronous within goroutine but panic-safe.

**Interface compliance:** `var _ Adapter = (*HTTPAdapter)(nil)` compile-time checks on all adapter implementations.

**Testing:** Table-driven tests with `testify/assert` and `testify/require`. Tests use in-memory SQLite (`:memory:`). Test files are co-located (`*_test.go`).

**Config:** YAML file (`hive.yaml`) with environment variable overrides (`HIVE_PORT`, `HIVE_DATA_DIR`, `HIVE_LOG_LEVEL`). Missing config file is not an error — defaults apply.

**Security patterns:**
- API auth: Bearer token, bcrypt-hashed keys, prefix-indexed for O(1) lookup, fail-closed
- SSRF prevention: webhook URLs validated against private IP ranges
- Response body limits: `io.LimitReader` (10MB) on all HTTP responses
- CSV injection protection in audit exports
- RBAC: admin/operator/viewer with fail-closed middleware
- No auth required when no API keys exist (dev mode)

## 5. Build & Run Commands

```bash
make build         # Build dashboard (npm) then Go binary → ./hive
make test          # go test ./... -v -count=1
make lint          # go vet ./...
make dev           # go run with debug logging
make serve         # Build then run ./hive serve
make dashboard     # cd web && npm run build
make clean         # Remove binary and dashboard dist
```

Version injected via ldflags: `-X github.com/JulienLeotier/hive/internal/cli.Version=$(VERSION)`

## 6. Architecture Summary

**Event-driven, single-binary architecture.** No external dependencies at runtime.

**Core flow:** CLI/API -> Manager registers agents -> Workflows parsed from YAML -> Tasks created with DAG dependencies -> Router matches tasks to capable agents -> Adapter protocol invokes agents -> Event bus persists + broadcasts all state changes -> Trust engine evaluates after completions -> Knowledge store captures learnings.

**Agent Adapter Protocol (5 methods):**
```go
type Adapter interface {
    Declare(ctx) (AgentCapabilities, error)  // What can you do?
    Invoke(ctx, Task) (TaskResult, error)    // Do this task
    Health(ctx) (HealthStatus, error)        // Are you alive?
    Checkpoint(ctx) (Checkpoint, error)      // Save your state
    Resume(ctx, Checkpoint) error            // Restore your state
}
```

**Adapters implemented:** HTTP (generic), Claude Code (CLI subprocess), MCP (HTTP delegation), CrewAI (Python subprocess), LangChain (LangServe HTTP), AutoGen (HTTP), OpenAI Assistants (Assistants API v2).

**Event bus:** In-process, SQLite-backed. Publish persists first (guaranteed delivery), then delivers to prefix-matching subscribers. Events use dot notation (`agent.registered`, `task.completed`, `workflow.started`).

**Task state machine:** `pending -> assigned -> running -> completed | failed`. Each transition emits an event. Checkpoint/resume supported on running tasks.

**Workflow DAG:** YAML-defined tasks with `depends_on` edges. Validated with Kahn's algorithm (cycle detection). `TopologicalSort` returns parallel-execution levels.

**Trust levels:** `supervised -> guided -> autonomous -> trusted`. Auto-promotion based on task count + error rate thresholds. Only promotes, never auto-demotes. History tracked in `trust_history` table.

**Autonomy:** Agents have PLAN.yaml behavioral state machines with heartbeat intervals. Scheduler wakes agents on their heartbeat. Each state has observe/action/transition rules.

**Circuit breakers:** Per-agent, 3-failure threshold, 30s reset. States: closed -> open -> half-open.

## 7. Database Schema

SQLite with WAL mode. Single migration file (`001_initial.sql`).

| Table | Purpose |
|---|---|
| `agents` | Registered agents (id, name, type, config, capabilities, plan, health_status, trust_level) |
| `events` | Append-only event log (type, source, payload). Indexed on type, source, created_at |
| `tasks` | Task state machine (workflow_id, type, status, agent_id, input, output, checkpoint, depends_on) |
| `workflows` | Registered workflows (name, config JSON, status) |
| `api_keys` | Bearer token auth (name, key_hash bcrypt, key_prefix for O(1) lookup) |
| `schema_versions` | Migration tracking |

Tables referenced in code but not yet in migrations (created at runtime or planned):
- `trust_history` — trust level change audit trail
- `knowledge` — shared knowledge entries (task_type, approach, outcome, context)
- `costs` — per-task cost tracking
- `webhooks` — outbound webhook configurations
- `audit_log` — compliance audit entries

## 8. CLI Commands

| Command | Description |
|---|---|
| `hive serve` | Start API server + dashboard (default port 8233) |
| `hive add-agent --name X --type http --url URL` | Register an agent |
| `hive remove-agent NAME` | Remove an agent |
| `hive status [--json]` | Show agents, health, trust levels |
| `hive logs [--type X] [--agent X] [--since 1h] [--limit 50] [--json]` | Query event logs |
| `hive validate [workflow.yaml]` | Validate workflow DAG |
| `hive init [name] [--template code-review\|content-pipeline\|research]` | Scaffold new project |
| `hive version` | Print version, Go version, OS/arch |

Global flag: `--log-level debug|info|warn|error`

## 9. API Endpoints

| Method | Path | Description |
|---|---|
| GET | `/api/v1/agents` | List all agents |
| GET | `/api/v1/events?type=X&source=X&since=RFC3339` | Query events |
| GET | `/api/v1/metrics` | Agent health counts + circuit breaker states |
| GET | `/` | Dashboard (SvelteKit SPA) |
| WS | `/ws` | Real-time event stream |

Auth: `Authorization: Bearer hive_<64-hex-chars>`. No auth required if no API keys exist (dev mode).

## 10. Important Rules for AI Agents

### DO
- Use `context.Context` as the first parameter in all functions that do I/O or DB
- Use `fmt.Errorf("verb noun: %w", err)` for error wrapping — keep the chain
- Use `ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)` for all IDs
- Use parameterized queries (`?` placeholders) for all SQL — never string interpolation
- Use `COALESCE` in SELECT for nullable TEXT columns
- Use `io.LimitReader` when reading HTTP response bodies
- Store timestamps as TEXT with `datetime('now')`, parse with `"2006-01-02 15:04:05"` layout
- Add `var _ Interface = (*Impl)(nil)` compile-time checks for interface implementations
- Keep adapter implementations thin — delegate to HTTPAdapter where possible
- Use `slog` (not `log`) for all logging
- Use `log/slog` structured logging with key-value pairs
- Emit events for all state transitions via the event bus
- Write table-driven tests with `testify/assert`
- Use `sql.NullString` or `COALESCE` for nullable DB fields
- Return early on errors — no deep nesting

### DON'T
- Don't use an ORM — direct `database/sql` only
- Don't use `log.Printf` — use `slog.Info/Debug/Warn/Error`
- Don't create goroutines without shutdown mechanisms (use channels or context)
- Don't store raw API keys — only bcrypt hashes
- Don't trust user-supplied URLs without SSRF validation
- Don't use `json.Marshal` for SQL values — use parameterized queries
- Don't skip error returns from `db.ExecContext` or `db.QueryContext`
- Don't use `time.Now()` in SQL — use `datetime('now')` for consistency
- Don't add external runtime dependencies (no Redis, no Kafka, no Postgres in single-node mode)
- Don't modify the `policy` map in `auth/rbac.go` at runtime
- Don't use `math/rand` — use `crypto/rand` for all randomness
- Don't forget to `defer rows.Close()` after every `QueryContext`
- Don't add new tables without a numbered migration file in `internal/storage/migrations/`
