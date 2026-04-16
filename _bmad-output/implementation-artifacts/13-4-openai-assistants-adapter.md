# Story 13.4: OpenAI Assistants Adapter

Status: done

## Story

As a user,
I want to register OpenAI Assistants with Hive,
so that I can orchestrate GPT-based assistants alongside local agents.

## Acceptance Criteria

1. **Given** an OpenAI API key and Assistant ID
   **When** the user runs `hive add-agent --type openai --assistant-id asst_xxx --api-key $OPENAI_API_KEY`
   **Then** the adapter is created with the provided credentials

2. **Given** a registered OpenAI Assistant
   **When** `Declare()` is called
   **Then** it returns capabilities with `openai-assistant` task type

3. **Given** a registered OpenAI Assistant
   **When** a task is routed to it
   **Then** the adapter creates a thread, adds a message with the task input, creates a run, polls for completion, and returns the assistant's response

4. **Given** an OpenAI adapter with no API key configured
   **When** `Health()` is called
   **Then** it returns `unavailable` with message "no API key configured"

5. **Given** a valid API key
   **When** `Health()` is called
   **Then** it returns `healthy`

6. **Given** an OpenAI run
   **When** the run fails or is cancelled
   **Then** the adapter returns a `failed` result with the run status

7. **Given** an OpenAI run
   **When** polling exceeds 60 attempts (120 seconds)
   **Then** the adapter returns a `failed` result with a timeout error

## Tasks / Subtasks

- [x] Task 1: OpenAI adapter struct (AC: #1, #2)
  - [x] Create `OpenAIAdapter` struct with `AssistantID`, `APIKey`, `Name`, and `http.Client` fields
  - [x] Implement `NewOpenAIAdapter(assistantID, apiKey, name)` constructor with 120s timeout
  - [x] Implement `Declare()` returning `openai-assistant` task type
  - [x] Verify compile-time interface satisfaction with `var _ Adapter = (*OpenAIAdapter)(nil)`
- [x] Task 2: OpenAI Assistants API integration (AC: #3, #6, #7)
  - [x] Implement `apiCall()` helper for authenticated OpenAI API requests
  - [x] Set `Authorization: Bearer`, `Content-Type: application/json`, `OpenAI-Beta: assistants=v2` headers
  - [x] Implement `createThread()` -- POST `/v1/threads`
  - [x] Implement `addMessage()` -- POST `/v1/threads/{id}/messages` with user role
  - [x] Implement `createRunAndPoll()` -- POST `/v1/threads/{id}/runs` then poll GET with 2s intervals
  - [x] Implement `getLastMessage()` -- GET `/v1/threads/{id}/messages?limit=1&order=desc`
  - [x] Handle `completed`, `failed`, `cancelled` run statuses
  - [x] Timeout after 60 polling attempts (120s total)
- [x] Task 3: Invoke orchestration (AC: #3)
  - [x] Implement `Invoke()` chaining: createThread -> addMessage -> createRunAndPoll -> return result
  - [x] Return `failed` TaskResult (not Go error) for API failures to allow graceful handling
- [x] Task 4: Health check (AC: #4, #5)
  - [x] Implement `Health()` checking API key presence
  - [x] Return `unavailable` if API key is empty, `healthy` otherwise
- [x] Task 5: Checkpoint/Resume stubs (AC: #1)
  - [x] Implement `Checkpoint()` and `Resume()` as no-ops
- [x] Task 6: Response size safety (AC: #3)
  - [x] Use `io.LimitReader` with 10MB cap on API response bodies

## Dev Notes

### Architecture Compliance

- Implements the `Adapter` interface from `internal/adapter/adapter.go`
- Uses `net/http` client directly rather than an OpenAI SDK to keep dependencies minimal
- API key is never logged -- passed only via Authorization header
- Response bodies capped at 10MB via `io.LimitReader` to prevent memory exhaustion
- Uses Assistants API v2 (`OpenAI-Beta: assistants=v2` header)

### Key Design Decisions

- Unlike the LangChain and AutoGen adapters, this adapter does NOT wrap HTTPAdapter -- the OpenAI Assistants API has a unique thread/run/message model that requires custom logic
- Each task invocation creates a fresh thread to avoid state leakage between tasks
- Polling uses a fixed 2-second interval with 60 max attempts (120s total) -- this matches the adapter's HTTP client timeout
- API errors are returned as `failed` TaskResults rather than Go errors, allowing the circuit breaker and retry system to handle them at the orchestration level
- Checkpoint/Resume are no-ops because OpenAI manages thread state server-side

### Integration Points

- `internal/adapter/adapter.go` -- implements `Adapter` interface
- `internal/cli/agent.go` -- `hive add-agent --type openai --assistant-id --api-key` creates this adapter
- `internal/agent/manager.go` -- stores agent record after `Declare()` call
- `internal/resilience/circuit_breaker.go` -- circuit breaker wraps Invoke calls

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 13.4]
- [Source: _bmad-output/planning-artifacts/prd.md#FR87, FR88]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- OpenAIAdapter implements full Assistants API v2 lifecycle: thread creation, message posting, run creation, polling, result extraction
- Custom apiCall helper handles authentication, headers, response limits
- Polling with 2s intervals, 60 max attempts, handles completed/failed/cancelled statuses
- Health check validates API key presence without making API call
- Checkpoint/Resume as no-ops -- OpenAI manages thread state server-side
- 10MB response body limit via io.LimitReader

### Change Log

- 2026-04-16: Story 13.4 implemented -- OpenAI Assistants adapter with full API v2 support

### File List

- internal/adapter/openai.go (new)
- internal/adapter/adapter.go (reference -- Adapter interface)
- internal/cli/agent.go (modified -- added openai type with assistant-id and api-key flags)
- internal/agent/manager.go (reference -- agent registration flow)
