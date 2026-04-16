# Configuration Reference

Hive uses a YAML configuration file (`hive.yaml`) with environment variable overrides. Configuration is loaded by `config.Load()` in `internal/config/config.go`.

## hive.yaml Options

```yaml
# Application log level: debug, info, warn, error
log_level: info

# Directory for SQLite database and data files
# Default: ~/.hive/data
data_dir: ~/.hive/data

# HTTP server port for API and dashboard
# Default: 8233
port: 8233
```

## Workflow Configuration

The same `hive.yaml` file also defines workflows, parsed by `internal/workflow/parser.go`:

```yaml
name: my-workflow

# Optional trigger definition
trigger:
  type: webhook          # "manual", "webhook", "schedule"
  schedule: "0 * * * *"  # cron expression (schedule trigger only)
  webhook: /hooks/start   # endpoint path (webhook trigger only)

# Task definitions (DAG)
tasks:
  - name: review
    type: code-review          # capability required from agent
    input:                     # arbitrary input passed to agent
      source: pr
    condition: "result.score > 0.8"  # optional conditional execution

  - name: summarize
    type: summarize
    depends_on: [review]       # DAG dependency (array of task names)
    input:
      format: markdown
```

### Task Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Unique task identifier within the workflow |
| `type` | string | yes | Capability name matched against agent declarations |
| `input` | any | no | Arbitrary data passed to the agent's `Invoke` method |
| `depends_on` | string[] | no | Task names that must complete before this task runs |
| `condition` | string | no | Expression evaluated to decide if task should execute |

### Validation Rules

The parser (`workflow.Parse()`) enforces:
- Workflow `name` is required
- At least one task is required
- Task names must be unique
- All `depends_on` references must point to existing tasks
- No self-dependencies
- No circular dependencies (verified via Kahn's algorithm)

Run `hive validate` to check your workflow file.

## Environment Variable Overrides

All config values can be overridden with `HIVE_`-prefixed environment variables. Overrides are applied after loading the YAML file.

| Variable | Config Field | Example |
|---|---|---|
| `HIVE_LOG_LEVEL` | `log_level` | `HIVE_LOG_LEVEL=debug` |
| `HIVE_DATA_DIR` | `data_dir` | `HIVE_DATA_DIR=/var/lib/hive` |
| `HIVE_PORT` | `port` | `HIVE_PORT=9000` |

## Defaults

If `hive.yaml` is missing, Hive uses these defaults (from `config.Default()`):

| Option | Default Value |
|---|---|
| `log_level` | `info` |
| `data_dir` | `~/.hive/data` |
| `port` | `8233` |

The `~` prefix in `data_dir` is expanded to the user's home directory at load time.

## SQLite Configuration

The SQLite database is stored at `<data_dir>/hive.db`. The following pragmas are set by `storage.Open()` in `internal/storage/sqlite.go`:

| Pragma | Value | Purpose |
|---|---|---|
| `journal_mode` | WAL | Concurrent reads during writes |
| `busy_timeout` | 5000 | Wait 5s for locks before failing |
| `journal_size_limit` | 64 MB | Cap WAL file growth |
| `foreign_keys` | ON | Enforce foreign key constraints |
| `synchronous` | NORMAL | Balance durability and performance |

## Database Migrations

Migrations are embedded SQL files in `internal/storage/migrations/`. They run automatically on startup. Applied versions are tracked in the `schema_versions` table. Currently:

- `001_initial.sql` -- agents, events, tasks, workflows, knowledge, trust_history, webhooks, api_keys, costs, audit_log tables

## Circuit Breaker Defaults

Set in `resilience.DefaultBreakerConfig()`:

| Setting | Default | Description |
|---|---|---|
| Threshold | 3 | Consecutive failures to trip the circuit |
| ResetTimeout | 30s | Wait time before trying a half-open request |

## Trust Thresholds

Set in `trust.DefaultThresholds()`:

| Level | Tasks Required | Max Error Rate |
|---|---|---|
| Guided | 50 | 10% |
| Autonomous | 200 | 5% |
| Trusted | 500 | 2% |

See [Trust Configuration](trust-configuration.md) for details.

## API Authentication

Authentication uses Bearer tokens. API keys are generated with bcrypt hashing and stored in the `api_keys` table. If no API keys exist, all requests are allowed (dev mode). See `internal/api/auth.go`.

## Project Templates

The `hive init --template` flag supports:
- `code-review` -- PR review then summarization
- `content-pipeline` -- write, edit, optimize, publish chain
- `research` -- parallel search with aggregation and report
