# Story 10.4: Knowledge CLI

Status: done

## Story

As a user,
I want to view and manage knowledge entries via CLI,
so that I can audit what my hive has learned.

## Acceptance Criteria

1. **Given** knowledge entries exist
   **When** the user runs `hive knowledge list --type code-review`
   **Then** entries are displayed with: task type, approach summary, outcome, age

2. **Given** knowledge entries exist
   **When** the user runs `hive knowledge search "how to handle timeouts"`
   **Then** semantically similar entries are returned ranked by relevance

3. **Given** no knowledge entries match the filter
   **When** the list or search command runs
   **Then** a helpful empty-state message is displayed

## Tasks / Subtasks

- [x] Task 1: Knowledge list command (AC: #1, #3)
  - [x] Add `hive knowledge list` cobra subcommand
  - [x] `--type` flag filters by task type via `Store.ListByType()`
  - [x] Display entries in table format: TYPE, APPROACH, OUTCOME, AGE
  - [x] Age calculated as human-readable duration (e.g., "2d ago", "1h ago")
  - [x] Empty state: "No knowledge entries found."
- [x] Task 2: Knowledge search command (AC: #2, #3)
  - [x] Add `hive knowledge search <query>` cobra subcommand
  - [x] Pass query string to `Store.Search()` with default limit 5
  - [x] Display results with rank number, task type, approach, outcome
  - [x] Empty state: "No matching knowledge entries found."
- [x] Task 3: JSON output support (AC: #1, #2)
  - [x] Both commands support `--json` flag for machine-readable output
  - [x] JSON output uses `json.NewEncoder` for consistent formatting

## Dev Notes

### Architecture Compliance

- **Cobra CLI** — follows existing command pattern with subcommands and flags
- **slog** — no additional logging beyond existing Store methods
- **JSON output** — consistent with other CLI commands (`hive status --json`, `hive logs --json`)
- **Config/store pattern** — commands load config, open store, create knowledge.Store, execute, close

### Key Design Decisions

- Knowledge CLI uses two subcommands under `hive knowledge`: `list` and `search` — following the existing CLI hierarchy pattern
- List filters by exact task type match (not prefix) — consistent with how knowledge entries are categorized
- Search delegates entirely to `Store.Search()` which handles keyword matching and recency weighting
- Age display uses relative time for readability — "2d ago" is more useful than an absolute timestamp for quick scanning
- Default search limit of 5 matches the Store default — can be overridden with `--limit` flag

### Integration Points

- `internal/cli/` — knowledge subcommands (list, search)
- `internal/knowledge/store.go` — `ListByType()` and `Search()` methods
- `internal/config/config.go` — config loading for data directory
- `internal/storage/sqlite.go` — database open/close

### References

- [Source: _bmad-output/planning-artifacts/prd.md#FR75]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 10.4]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- `hive knowledge list --type <type>` displays entries filtered by task type
- `hive knowledge search "<query>"` returns relevance-ranked results
- Both commands support `--json` flag for machine-readable output
- Human-readable age display ("2d ago") for quick scanning
- Empty state messages for no-match scenarios

### Change Log

- 2026-04-16: Story 10.4 implemented — knowledge CLI with list and search subcommands

### File List

- internal/cli/agent.go (modified — added knowledge subcommands)
- internal/knowledge/store.go (reference — ListByType, Search methods)
- internal/config/config.go (reference — config loading)
- internal/storage/sqlite.go (reference — database access)
