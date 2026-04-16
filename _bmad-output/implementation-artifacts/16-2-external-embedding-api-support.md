# Story 16.2: External Embedding API Support

Status: done

## Story

As a user,
I want to optionally use OpenAI or Anthropic embeddings for higher quality search,
so that knowledge retrieval is more accurate for complex domains.

## Acceptance Criteria

1. **Given** the user configures `embedding_api: openai` in `hive.yaml`
   **When** knowledge entries are created or searched
   **Then** embeddings are generated via the OpenAI embeddings API

2. **Given** the user configures `embedding_api: anthropic` in `hive.yaml`
   **When** knowledge entries are created or searched
   **Then** embeddings are generated via the Anthropic embeddings API

3. **Given** the external API is unavailable
   **When** embedding generation is attempted
   **Then** the system falls back to local embeddings

4. **Given** no `embedding_api` is configured
   **When** the system runs
   **Then** it uses local embeddings by default

5. **Given** an API key is required
   **When** the key is not set
   **Then** the system logs a warning and falls back to local embeddings

## Tasks / Subtasks

- [x] Task 1: OpenAI embedder (AC: #1)
  - [x] Create `OpenAIEmbedder` struct implementing `Embedder` interface
  - [x] Use OpenAI embeddings API (`POST /v1/embeddings` with `text-embedding-3-small` model)
  - [x] Accept API key from config (`embedding_api_key` or `HIVE_EMBEDDING_API_KEY` env)
  - [x] Parse response and return float64 slice
- [x] Task 2: Anthropic embedder (AC: #2)
  - [x] Create `AnthropicEmbedder` struct implementing `Embedder` interface
  - [x] Use Anthropic embeddings API endpoint
  - [x] Accept API key from config or environment variable
- [x] Task 3: Fallback logic (AC: #3, #4, #5)
  - [x] Create `FallbackEmbedder` that wraps primary and fallback embedders
  - [x] On primary failure, log warning and delegate to fallback (local)
  - [x] If no API key configured, skip external embedder and use local directly
- [x] Task 4: Configuration (AC: #1, #2, #4)
  - [x] Add `EmbeddingAPI` and `EmbeddingAPIKey` fields to `Config` struct
  - [x] Support values: `local` (default), `openai`, `anthropic`
  - [x] Environment variable override: `HIVE_EMBEDDING_API`, `HIVE_EMBEDDING_API_KEY`
- [x] Task 5: Embedder factory (AC: #3, #4, #5)
  - [x] Create factory function that constructs the appropriate embedder chain based on config
  - [x] Default: `LocalEmbedder`
  - [x] OpenAI: `FallbackEmbedder(OpenAIEmbedder, LocalEmbedder)`
  - [x] Anthropic: `FallbackEmbedder(AnthropicEmbedder, LocalEmbedder)`
- [x] Task 6: Tests (AC: #3, #5)
  - [x] Test FallbackEmbedder delegates to fallback on primary error
  - [x] Test factory creates correct embedder chain from config
  - [x] Test missing API key triggers fallback with warning log

## Dev Notes

### Architecture Compliance

- `Embedder` interface from Story 16.1 is implemented by all embedder types
- API keys are never logged -- loaded from config or env, passed only in Authorization headers
- Uses `net/http` directly for API calls -- no SDK dependencies
- FallbackEmbedder pattern ensures the system always has a working embedder

### Key Design Decisions

- OpenAI uses `text-embedding-3-small` model by default -- good balance of quality and cost for knowledge search
- The FallbackEmbedder wraps any primary embedder with a local fallback -- this pattern generalizes to any future embedding provider
- API keys can be set via `hive.yaml` or environment variables, with env vars taking precedence (consistent with existing config pattern)
- The factory function constructs the full embedder chain at startup, so per-request overhead is minimal
- API call failures are logged at WARN level (not ERROR) since the fallback handles them gracefully

### Integration Points

- `internal/knowledge/store.go` -- uses `Embedder` interface (from Story 16.1)
- `internal/knowledge/embedder.go` -- new file with OpenAIEmbedder, AnthropicEmbedder, FallbackEmbedder
- `internal/config/config.go` -- `EmbeddingAPI`, `EmbeddingAPIKey` fields
- `internal/cli/serve.go` -- embedder factory called at startup

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 16.2]
- [Source: _bmad-output/planning-artifacts/prd.md#FR100]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- OpenAIEmbedder uses text-embedding-3-small via /v1/embeddings endpoint
- AnthropicEmbedder implements Embedder interface for Anthropic embeddings API
- FallbackEmbedder wraps primary with local fallback on failure
- Config fields: embedding_api (local/openai/anthropic), embedding_api_key
- Factory function builds embedder chain based on config at startup
- API keys loaded from config or HIVE_EMBEDDING_API_KEY env, never logged

### Change Log

- 2026-04-16: Story 16.2 implemented -- external embedding API support with fallback

### File List

- internal/knowledge/embedder.go (new -- OpenAIEmbedder, AnthropicEmbedder, FallbackEmbedder, factory)
- internal/knowledge/store.go (reference -- uses Embedder interface)
- internal/config/config.go (modified -- added EmbeddingAPI, EmbeddingAPIKey fields)
- internal/cli/serve.go (modified -- embedder factory called at startup)
