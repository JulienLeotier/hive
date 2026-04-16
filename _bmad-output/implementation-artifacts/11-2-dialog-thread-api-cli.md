# Story 11.2: Dialog Thread API & CLI

Status: done

## Story

As a user,
I want to view dialog threads and their messages,
so that I can understand how agents collaborate.

## Acceptance Criteria

1. **Given** dialog threads exist
   **When** the user runs `hive dialogs list`
   **Then** active and recent threads are displayed with: initiator, participant, topic, status, message count

2. **Given** a dialog thread with messages
   **When** the user runs `hive dialogs show <thread-id>`
   **Then** the full conversation is displayed with: sender, content, timestamp for each message

3. **Given** the `--json` flag is provided
   **When** either command runs
   **Then** output is valid JSON for scripting

4. **Given** no dialog threads exist
   **When** `hive dialogs list` runs
   **Then** a helpful empty state message is displayed

## Tasks / Subtasks

- [x] Task 1: Dialogs list command (AC: #1, #3, #4)
  - [x] Add `hive dialogs list` cobra subcommand
  - [x] Query dialog_threads joined with message count
  - [x] Display table: TOPIC, INITIATOR, PARTICIPANT, STATUS, MESSAGES
  - [x] Support `--json` flag for JSON output
  - [x] Empty state: "No dialog threads found."
- [x] Task 2: Dialogs show command (AC: #2, #3)
  - [x] Add `hive dialogs show <thread-id>` cobra subcommand
  - [x] Query dialog_messages for the given thread in chronological order
  - [x] Display each message: `[timestamp] sender: content`
  - [x] Support `--json` flag for JSON output
  - [x] Error message if thread not found
- [x] Task 3: JSON output support (AC: #3)
  - [x] Both commands use `json.NewEncoder` for consistent JSON formatting
  - [x] Thread list includes message_count in JSON
  - [x] Message list includes all fields: sender_id, content, created_at

## Dev Notes

### Architecture Compliance

- **Cobra CLI** — follows existing subcommand pattern (`hive dialogs list`, `hive dialogs show`)
- **JSON output** — consistent `--json` flag pattern matching other CLI commands
- **Config/store pattern** — load config, open store, query, close — same as other commands
- **Table output** — uses `fmt.Printf` with fixed-width formatting for alignment

### Key Design Decisions

- Dialog commands are under `hive dialogs` namespace (not `hive dialog`) for consistency with plural resource naming in CLI
- Thread list shows message count via SQL `LEFT JOIN` + `COUNT(*)` for efficiency
- Show command displays messages in chronological order (oldest first) for natural conversation reading
- Thread ID is required as a positional argument (not a flag) for `show` command — follows convention of `hive logs --workflow <id>`

### Integration Points

- `internal/cli/` — dialogs subcommands (list, show)
- `internal/storage/sqlite.go` — database access for dialog queries
- `internal/config/config.go` — config loading for data directory

### References

- [Source: _bmad-output/planning-artifacts/prd.md#FR78, FR79]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 11.2]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- `hive dialogs list` shows all threads with message count
- `hive dialogs show <id>` displays full conversation chronologically
- Both commands support --json for scripting integration
- Empty state messages for no-match scenarios
- Table format with fixed-width columns for readability

### Change Log

- 2026-04-16: Story 11.2 implemented — dialog thread CLI with list and show commands

### File List

- internal/cli/agent.go (modified — added dialogs subcommands)
- internal/storage/sqlite.go (reference — database queries)
- internal/config/config.go (reference — config loading)
