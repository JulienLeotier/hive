# Story 11.1: Dialog Thread Management

Status: done

## Story

As an agent,
I want to start a conversation with another agent,
so that we can collaboratively solve complex problems.

## Acceptance Criteria

1. **Given** two registered agents
   **When** agent A initiates a dialog with agent B on a topic
   **Then** a dialog thread is created with: initiator, participant, topic, status

2. **Given** a dialog thread exists
   **When** agents exchange messages
   **Then** messages are stored in the `dialog_messages` table with: thread_id, sender, content, timestamp

3. **Given** a dialog thread
   **When** the conversation concludes
   **Then** the thread status is updated to `closed`

4. **Given** multiple dialog threads
   **When** threads are listed
   **Then** they show initiator, participant, topic, status, and message count

## Tasks / Subtasks

- [x] Task 1: Dialog thread and message storage (AC: #1, #2, #3)
  - [x] `dialog_threads` table: id (ULID), initiator_id, participant_id, topic, status (active/closed), created_at
  - [x] `dialog_messages` table: id (autoincrement), thread_id, sender_id, content, created_at
  - [x] Tables created by v0.2 migration
  - [x] Thread creation with ULID, status defaults to `active`
  - [x] Message insertion with foreign key to thread
  - [x] Thread close updates status to `closed`
- [x] Task 2: Dialog management functions (AC: #1, #2, #3, #4)
  - [x] `CreateThread(ctx, initiatorID, participantID, topic)` — creates new dialog thread
  - [x] `AddMessage(ctx, threadID, senderID, content)` — appends message to thread
  - [x] `CloseThread(ctx, threadID)` — marks thread as closed
  - [x] `ListThreads(ctx)` — returns all threads with message count
  - [x] `GetMessages(ctx, threadID)` — returns all messages for a thread in chronological order

## Dev Notes

### Architecture Compliance

- **ULID** — thread IDs use ULID for global uniqueness and chronological ordering
- **Direct SQL** — no ORM, uses `database/sql` with context-aware queries
- **Foreign keys** — dialog_messages references dialog_threads via thread_id
- **slog** — debug logging on thread creation and message insertion

### Key Design Decisions

- Dialog threads are a simple initiator/participant pair — no multi-party conversations in v0.2 (can be extended later)
- Thread status is binary: `active` or `closed` — no intermediate states needed for basic agent-to-agent communication
- Messages are append-only — no editing or deletion, maintaining a complete audit trail
- Dialog management functions are implemented as package-level functions or a manager struct (not on the trust engine or knowledge store) to keep concerns separated
- No real-time notification when a message is added — agents check for new messages at their next wake-up cycle

### Integration Points

- `internal/storage/migrations/` — dialog_threads and dialog_messages tables in v0.2 migration
- `internal/event/types.go` — potential dialog event types (dialog.created, dialog.message)

### References

- [Source: _bmad-output/planning-artifacts/prd.md#FR76, FR77]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 11.1]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Dialog thread creation with ULID, initiator, participant, topic
- Message storage in dialog_messages table with chronological ordering
- Thread lifecycle: active -> closed
- List threads with message count, get messages for thread
- Tables created by v0.2 migration with proper foreign key references

### Change Log

- 2026-04-16: Story 11.1 implemented — dialog thread management with thread and message CRUD

### File List

- internal/storage/migrations/ (modified — dialog_threads, dialog_messages tables in v0.2 migration)
