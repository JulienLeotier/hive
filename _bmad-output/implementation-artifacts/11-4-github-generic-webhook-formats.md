# Story 11.4: GitHub & Generic Webhook Formats

Status: done

## Story

As a user,
I want webhook notifications in GitHub and generic formats,
so that I can integrate with my existing tools.

## Acceptance Criteria

1. **Given** a webhook configured with `--type github`
   **When** a matching event occurs
   **Then** the notification is formatted as a GitHub repository_dispatch payload with `event_type` and `client_payload`

2. **Given** a webhook configured with `--type generic`
   **When** a matching event occurs
   **Then** the notification contains the full event as JSON: id, type, source, payload, created_at

3. **Given** a webhook configured with `--type slack`
   **When** a matching event occurs
   **Then** the notification is a Slack-compatible JSON payload with a `text` field containing event summary

4. **Given** any webhook type
   **When** the payload is generated
   **Then** the Content-Type header is `application/json`

## Tasks / Subtasks

- [x] Task 1: Payload format functions (AC: #1, #2, #3)
  - [x] Implement `formatPayload(whType, evt)` — dispatches to type-specific formatters
  - [x] Slack format: `{"text": "[Hive] <type> from <source>: <payload>"}`
  - [x] GitHub format: `{"event_type": "<type>", "client_payload": {"source": "...", "payload": "..."}}`
  - [x] Generic format: `{"id": ..., "type": "...", "source": "...", "payload": "...", "created_at": "..."}`
  - [x] All payloads JSON-encoded via `json.Marshal`
- [x] Task 2: Content-Type header (AC: #4)
  - [x] `Content-Type: application/json` set on all webhook POST requests in `deliver()`
- [x] Task 3: Unit tests (AC: #1, #2, #3)
  - [x] Test Slack format contains event type and source in text field
  - [x] Test GitHub format has event_type at top level and client_payload with source
  - [x] Test generic format contains all event fields

## Dev Notes

### Architecture Compliance

- **json.Marshal** — all payload formatting uses Go's standard library JSON encoder
- **Pattern matching** — `formatPayload` uses a switch statement on webhook type string, defaulting to generic format
- **No external dependencies** — payload formatting is pure Go string/JSON manipulation

### Key Design Decisions

- Three webhook types supported: `slack`, `github`, `generic` — covers the most common integration targets
- Slack format uses a single `text` field for maximum compatibility with Slack's incoming webhooks API
- GitHub format follows the `repository_dispatch` event structure, enabling GitHub Actions workflows to be triggered by Hive events
- Generic format provides the raw event data — suitable for custom integrations, logging services, or webhook relay platforms
- Unknown webhook types fall through to the `generic` format rather than erroring — defensive design
- All formatters use `json.Marshal` which handles escaping automatically — prevents injection via event payload content

### Integration Points

- `internal/webhook/dispatcher.go` — `formatPayload()` function with type-specific formatting
- `internal/webhook/dispatcher_test.go` — TestSlackFormat, TestGitHubFormat tests

### References

- [Source: _bmad-output/planning-artifacts/prd.md#FR82]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 11.4]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Three webhook payload formats: Slack (text message), GitHub (repository_dispatch), Generic (full event JSON)
- All payloads JSON-encoded with Content-Type: application/json
- Unknown types default to generic format
- Slack format embeds event type and source in human-readable text
- GitHub format uses event_type + client_payload structure for Actions integration

### Change Log

- 2026-04-16: Story 11.4 implemented — Slack, GitHub, and generic webhook payload formatters

### File List

- internal/webhook/dispatcher.go (modified — formatPayload with Slack, GitHub, generic formatters)
- internal/webhook/dispatcher_test.go (modified — TestSlackFormat, TestGitHubFormat)
