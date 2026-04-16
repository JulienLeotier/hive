# Story 11.3: Webhook Configuration

Status: done

## Story

As a user,
I want to configure webhooks for event notifications,
so that I'm notified when important things happen in my hive.

## Acceptance Criteria

1. **Given** the user runs `hive webhook add --name slack-alerts --url https://hooks.slack.com/... --type slack --events task.failed,agent.isolated`
   **When** the webhook is registered
   **Then** a matching event triggers a formatted notification to the webhook URL

2. **Given** a webhook is configured with an event filter
   **When** an event occurs that matches the filter
   **Then** the system sends the notification asynchronously

3. **Given** a webhook delivery fails
   **When** the retry policy is applied
   **Then** the system retries 3 times with exponential backoff (2s, 4s delays)

4. **Given** a webhook URL targets a private/internal IP
   **When** the user tries to add it
   **Then** the system rejects it with an SSRF prevention error

## Tasks / Subtasks

- [x] Task 1: Webhook Dispatcher implementation (AC: #1, #2, #3, #4)
  - [x] Create `internal/webhook/dispatcher.go` with `Dispatcher` struct backed by `*sql.DB`
  - [x] Define `Config` struct: ID, Name, URL, Type, EventFilter, Enabled
  - [x] Implement `NewDispatcher(db)` with HTTP client (10s timeout)
  - [x] Implement `Add(ctx, name, url, whType, eventFilter)` with SSRF URL validation
  - [x] Implement `List(ctx)` — returns all webhook configurations
  - [x] Implement `Dispatch(ctx, event)` — loads configs, filters by event type, dispatches async
  - [x] Implement `deliver(cfg, evt)` — POST with 3 retries, exponential backoff (2s, 4s)
- [x] Task 2: SSRF prevention (AC: #4)
  - [x] Implement `validateWebhookURL(rawURL)` — rejects localhost, 127.0.0.1, 0.0.0.0, link-local, metadata endpoints
  - [x] Block private IP ranges: 10.x.x.x, 172.16-31.x.x, 192.168.x.x
  - [x] Only allow http:// and https:// schemes
- [x] Task 3: Event filter matching (AC: #1, #2)
  - [x] Implement `matchesFilter(eventType, filter)` — supports JSON array and comma-separated formats
  - [x] Empty filter matches all events
  - [x] Wildcard suffix matching: `task.*` matches `task.failed`
- [x] Task 4: Webhook table schema (AC: #1)
  - [x] `webhooks` table: id (ULID), name (unique), url, type, event_filter, enabled (boolean), created_at
  - [x] Table created by v0.2 migration
- [x] Task 5: Unit tests (AC: #1, #2, #3, #4)
  - [x] Test Add and List (register webhook, verify retrieval)
  - [x] Test Dispatch matching event (httptest server receives notification)
  - [x] Test Dispatch non-matching event (httptest server not called)
  - [x] Test Slack format (JSON with text field containing event type and source)
  - [x] Test GitHub format (JSON with event_type and client_payload)
  - [x] Test matchesFilter with JSON array, comma-separated, empty filter

## Dev Notes

### Architecture Compliance

- **Async delivery** — `Dispatch()` launches goroutines for each matching webhook, non-blocking to the event publisher
- **HTTP client** — 10-second timeout prevents hanging connections from blocking the retry loop
- **ULID** — webhook config IDs use ULID via `ulid.MustNew` with `crypto/rand.Reader`
- **slog** — structured logging at debug (success), warn (retry), and error (exhausted) levels
- **SSRF prevention** — `validateWebhookURL()` blocks private IPs, localhost, and metadata endpoints

### Key Design Decisions

- Webhook dispatch is fire-and-forget from the publisher's perspective — the event bus publishes, the dispatcher handles delivery asynchronously
- 3 retries with exponential backoff (2s, 4s delays) — total max wait of 6 seconds before giving up
- Event filter supports two formats: JSON array (`["task.failed","agent.isolated"]`) and comma-separated (`task.failed,agent.isolated`) for flexibility
- Wildcard suffix matching (e.g., `task.*`) implemented via string prefix comparison
- SSRF validation runs only on `Add()` — webhooks already in the database are trusted (they passed validation on creation)
- Webhook type determines payload format: `slack` (text message), `github` (repository_dispatch format), `generic` (raw event JSON)

### Integration Points

- `internal/webhook/dispatcher.go` — webhook CRUD and async delivery
- `internal/webhook/dispatcher_test.go` — 6 unit tests
- `internal/event/types.go` — Event struct for webhook payload formatting
- `internal/storage/migrations/` — webhooks table in v0.2 migration

### References

- [Source: _bmad-output/planning-artifacts/prd.md#FR80, FR81, FR83]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 11.3]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Webhook dispatcher with Add, List, Dispatch operations
- Async delivery with 3 retries and exponential backoff (2s, 4s)
- SSRF prevention blocks private IPs, localhost, metadata endpoints
- Event filter matching: JSON array, comma-separated, wildcard suffix
- Slack and GitHub payload formatters for platform-specific notifications
- 6 unit tests covering CRUD, dispatch, filtering, and payload formatting

### Change Log

- 2026-04-16: Story 11.3 implemented — webhook configuration with SSRF prevention, async dispatch, and retry logic

### File List

- internal/webhook/dispatcher.go (new)
- internal/webhook/dispatcher_test.go (new)
