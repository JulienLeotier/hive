# Story 16.1: Vector Embedding for Knowledge Search

Status: done

## Story

As a user,
I want knowledge search to understand meaning, not just keywords,
so that I find relevant approaches even when using different terminology.

## Acceptance Criteria

1. **Given** knowledge entries exist
   **When** the user searches "how to handle API timeouts"
   **Then** entries about "retry on connection failure" or "backoff strategy" are returned

2. **Given** a new knowledge entry is recorded
   **When** it is stored
   **Then** a vector embedding is generated and stored alongside the entry

3. **Given** a search query
   **When** the search executes
   **Then** the query is embedded and compared against stored embeddings via cosine similarity

4. **Given** embeddings are not available (model loading fails)
   **When** a search is performed
   **Then** the system falls back to keyword-based search (existing behavior)

5. **Given** entries with embeddings and entries without
   **When** a search is performed
   **Then** both are considered -- embedded entries use vector similarity, others use keyword matching

## Tasks / Subtasks

- [x] Task 1: Embedding interface (AC: #2, #4)
  - [x] Define `Embedder` interface with `Embed(text string) ([]float64, error)` method
  - [x] Create `LocalEmbedder` struct implementing lightweight local embedding
  - [x] Use TF-IDF or bag-of-words approach for local embeddings (no external dependencies)
  - [x] Fallback to nil embedder if initialization fails
- [x] Task 2: Embedding generation on record (AC: #2)
  - [x] Modify `Store.Record()` to generate embedding for approach + context text
  - [x] Store embedding as BLOB in knowledge table's existing `embedding` column
  - [x] Handle embedding failures gracefully -- store entry without embedding
- [x] Task 3: Vector similarity search (AC: #1, #3)
  - [x] Modify `Store.Search()` to embed the query text
  - [x] Compute cosine similarity between query embedding and stored embeddings
  - [x] Rank results by combined score: vector similarity * 0.7 + recency boost * 0.3
  - [x] Return top-N results
- [x] Task 4: Fallback to keyword search (AC: #4, #5)
  - [x] If embedder is nil or embedding fails, fall back to existing keyword-based search
  - [x] Entries without embeddings use keyword matching score in combined ranking
  - [x] Ensure zero regression from existing search behavior
- [x] Task 5: Tests (AC: #1, #3, #4)
  - [x] Test embedding generation stores non-nil BLOB
  - [x] Test vector similarity search returns semantically related results
  - [x] Test fallback to keyword search when embedder is unavailable
  - [x] Verify existing search tests still pass

## Dev Notes

### Architecture Compliance

- `internal/knowledge/store.go` -- `Store` struct extended with optional `Embedder` dependency
- Embedding stored in existing `embedding BLOB` column in knowledge table (already defined in v0.2 schema)
- Uses pure Go implementation for local embeddings -- no CGO, no external model files
- Graceful degradation: keyword search remains available when embeddings fail

### Key Design Decisions

- Local embeddings use a TF-IDF bag-of-words approach rather than a neural model -- this keeps the binary small and avoids model file distribution complexity
- The `Embedder` interface allows swapping to external API embeddings (Story 16.2) without changing the knowledge store
- Cosine similarity is computed in Go using a simple dot product / magnitude calculation -- no linear algebra library needed
- Combined scoring (70% similarity + 30% recency) matches the existing keyword search weighting, ensuring consistent result ranking
- Entries without embeddings are scored by keyword matching alone, allowing mixed-mode operation during migration from keyword to vector search

### Integration Points

- `internal/knowledge/store.go` -- `Store.Search()` and `Store.Record()` modified
- `internal/knowledge/store.go` -- `Embedder` interface defined
- `internal/knowledge/store_test.go` -- extended tests for vector search
- `internal/config/config.go` -- embedding configuration (local vs API)

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 16.1]
- [Source: _bmad-output/planning-artifacts/prd.md#FR98, FR99]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Embedder interface defined with Embed(text) method for pluggable embedding backends
- LocalEmbedder uses TF-IDF bag-of-words approach -- pure Go, no external dependencies
- Store.Record() generates and stores embedding as BLOB alongside knowledge entry
- Store.Search() uses cosine similarity for vector search with recency-weighted ranking
- Graceful fallback to keyword search when embedder unavailable or entry lacks embedding
- All existing knowledge store tests continue to pass

### Change Log

- 2026-04-16: Story 16.1 implemented -- vector embedding for semantic knowledge search

### File List

- internal/knowledge/store.go (modified -- Embedder interface, vector search in Search/Record)
- internal/knowledge/store_test.go (modified -- added vector search and fallback tests)
- internal/config/config.go (modified -- embedding configuration field)
