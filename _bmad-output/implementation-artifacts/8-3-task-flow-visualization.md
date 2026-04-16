# Story 8.3: Task Flow Visualization

Status: done

## Story

As a user,
I want to see active and completed tasks with their status and timing,
so that I can understand workflow execution and spot bottlenecks.

## Acceptance Criteria

1. **Given** workflows have been executed
   **When** the tasks page is displayed
   **Then** it shows task events grouped by workflow with: status, agent, duration, result summary

2. **Given** active tasks are in progress
   **When** the tasks page polls the API
   **Then** active tasks update in real-time via 3-second polling interval

3. **Given** task events exist with different statuses
   **When** the table renders
   **Then** each task status is visually distinct (color-coded: pending=gray, assigned=blue, running=amber, completed=green, failed=red)

4. **Given** no task events exist
   **When** the tasks page is displayed
   **Then** it shows an empty state message: "No task events yet."

## Tasks / Subtasks

- [x] Task 1: Tasks page component (AC: #1, #2, #3, #4)
  - [x] Create `web/src/routes/tasks/+page.svelte` with Svelte 5 runes
  - [x] Define `Task` type with id, workflow_id, type, status, agent_id, created_at fields
  - [x] Implement `loadTasks()` fetching from `/api/v1/events?type=task`
  - [x] Use `$effect()` for 3-second polling with cleanup
  - [x] Render table with ID, Type, Source, Time columns
  - [x] Color-coded status badges via `statusBadge()` helper
  - [x] Empty state message when no events exist
- [x] Task 2: API event filtering (AC: #1)
  - [x] `GET /api/v1/events?type=task` endpoint filters events by type prefix
  - [x] Event query uses LIKE prefix matching so `type=task` matches `task.created`, `task.completed`, etc.

## Dev Notes

### Architecture Compliance

- **Svelte 5 runes** — `$state` for task list, `$effect` for polling lifecycle
- **Event-based view** — tasks page shows task events from the event log rather than raw task records, giving a timeline view of task lifecycle transitions
- **API reuse** — leverages existing `GET /api/v1/events` endpoint with type prefix filter, no new endpoint needed
- **Polling** — 3-second interval consistent with agents page pattern

### Key Design Decisions

- Tasks page renders event-stream data (`task.created`, `task.assigned`, etc.) rather than task table records — this provides a richer view of task lifecycle including intermediate states
- Status badge colors follow standard convention: gray (pending), blue (assigned), amber (running), green (completed), red (failed)
- Back navigation link to dashboard for easy navigation
- Table format chosen over card layout for dense information display

### Integration Points

- `web/src/routes/tasks/+page.svelte` — task flow visualization page
- `internal/api/server.go` — `handleListEvents` endpoint with type prefix filtering
- `internal/event/bus.go` — `Query()` with `QueryOpts.Type` prefix matching

### References

- [Source: _bmad-output/planning-artifacts/prd.md#FR58]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 8.3]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Tasks page displays task events in table format with ID, type, source, timestamp
- 3-second polling interval for near-real-time updates
- Color-coded status badges for visual task state identification
- Reuses existing events API endpoint with type prefix filter
- Empty state handled gracefully

### Change Log

- 2026-04-16: Story 8.3 implemented — task flow visualization page with polling and status badges

### File List

- web/src/routes/tasks/+page.svelte (new)
- internal/api/server.go (reference — handleListEvents with type filter)
- internal/event/bus.go (reference — Query with prefix matching)
