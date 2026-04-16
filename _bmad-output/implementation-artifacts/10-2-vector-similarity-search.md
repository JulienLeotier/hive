# Story 10.2: Vector Similarity Search

Status: done

## Story

As an agent,
I want to search for similar prior approaches before starting a task,
so that I can learn from the colony's experience.

## Acceptance Criteria

1. **Given** knowledge entries exist
   **When** an agent queries for approaches similar to its current task
   **Then** the system returns the top-5 most similar entries ranked by relevance

2. **Given** search results are returned
   **When** the results are ranked
   **Then** they include both successful and failed approaches (not filtered by outcome)

3. **Given** a search query with no matching entries
   **When** the search runs
   **Then** it returns an empty result set (not an error)

4. **Given** more than the requested limit of matching entries
   **When** search runs with a limit
   **Then** only the top N entries are returned

## Tasks / Subtasks

- [x] Task 1: Search implementation — keyword similarity (AC: #1, #2, #3, #4)
  - [x] Implement `Search(ctx, query, limit)` on Store
  - [x] Fetch all non-expired entries (within maxAge) from database, limit 1000
  - [x] Tokenize query into lowercase words
  - [x] Score each entry: combine task_type + approach + context into searchable text
  - [x] Keyword similarity: count matching words / total query words (0-1 range)
  - [x] Recency boost: exponential decay with 30-day half-life (`exp(-0.023 * age_days)`)
  - [x] Combined score: 70% keyword similarity + 30% recency boost
  - [x] Sort by score descending, return top N entries
  - [x] Skip entries with zero keyword matches
  - [x] Default limit of 5 when limit <= 0
- [x] Task 2: Unit tests (AC: #1, #2, #3, #4)
  - [x] Test search finds entries by keyword match (Go error handling query matches Go-related entry)
  - [x] Test empty search returns empty result set
  - [x] Test search limit is respected (20 entries, limit 3 = 3 results)
  - [x] Test search includes both success and failure outcomes

## Dev Notes

### Architecture Compliance

- **Pure Go** — keyword matching uses `strings.Contains` and `strings.Fields`, no external NLP library
- **Math** — recency boost uses `math.Exp` for exponential decay calculation
- **Two-phase** — fetch from DB first (SQL filtering by date), then score in Go (keyword matching + recency)

### Key Design Decisions

- v0.2 uses keyword-based similarity (TF-IDF-like) rather than true vector embeddings — this avoids an external embedding API dependency. Vector embeddings planned for v0.3 (Story 16.1)
- Scoring formula: `0.7 * keyword_similarity + 0.3 * recency_boost` — keyword match is weighted higher than recency because relevance matters more than freshness
- Recency boost uses exponential decay with half-life of 30 days: `exp(-ln(2)/30 * age)` where `ln(2)/30 ≈ 0.023`
- Entries older than `maxAge` (90 days) are excluded at the SQL level before scoring
- Fetch limit of 1000 prevents loading the entire knowledge table into memory — sufficient for early usage patterns
- Bubble sort used for ranking (simple, correct, and the result set is small after keyword filtering)

### Integration Points

- `internal/knowledge/store.go` — `Search()` method
- `internal/knowledge/store_test.go` — search unit tests

### References

- [Source: _bmad-output/planning-artifacts/prd.md#FR73]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 10.2]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Keyword-based similarity search with recency weighting
- Scoring: 70% keyword match + 30% exponential recency decay (30-day half-life)
- Entries older than 90 days excluded at SQL level
- Returns top-N results sorted by combined score
- Both success and failure outcomes included in results
- 4 unit tests covering keyword matching, empty results, limits, and mixed outcomes

### Change Log

- 2026-04-16: Story 10.2 implemented — keyword similarity search with recency-weighted scoring

### File List

- internal/knowledge/store.go (modified — added Search method)
- internal/knowledge/store_test.go (modified — added search tests)
