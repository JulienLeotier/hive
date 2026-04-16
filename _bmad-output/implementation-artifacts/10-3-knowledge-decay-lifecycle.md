# Story 10.3: Knowledge Decay & Lifecycle

Status: done

## Story

As the system,
I want knowledge entries to decay over time,
so that stale patterns don't override recent learnings.

## Acceptance Criteria

1. **Given** knowledge entries with varying ages
   **When** similarity search runs
   **Then** results are weighted by recency (newer entries rank higher at equal similarity)

2. **Given** entries older than configurable threshold (default 90 days)
   **When** search runs
   **Then** they are excluded from results entirely

3. **Given** two entries with identical keyword similarity
   **When** one is 1 day old and the other is 60 days old
   **Then** the newer entry ranks higher due to the recency boost

## Tasks / Subtasks

- [x] Task 1: Configurable maxAge (AC: #2)
  - [x] `maxAge` field on `Store` struct, default 90 days (`90 * 24 * time.Hour`)
  - [x] Set in `NewStore(db)` constructor
  - [x] Search query filters by `created_at >= cutoff` where cutoff = `now - maxAge`
- [x] Task 2: Recency weighting in search scoring (AC: #1, #3)
  - [x] Calculate entry age in days: `time.Since(entry.CreatedAt).Hours() / 24`
  - [x] Exponential decay: `exp(-0.023 * age_days)` — 30-day half-life
  - [x] Combined score: `similarity * 0.7 + recencyBoost * 0.3`
  - [x] Newer entries with equal keyword matches always score higher
- [x] Task 3: Date-based SQL filtering (AC: #2)
  - [x] Cutoff time formatted as `2006-01-02 15:04:05` for SQLite comparison
  - [x] SQL WHERE clause: `created_at >= ?` with cutoff parameter
  - [x] Entries beyond maxAge never loaded into memory

## Dev Notes

### Architecture Compliance

- **Configurable** — `maxAge` is set at construction time, future stories can expose it via `hive.yaml`
- **SQL-level filtering** — date cutoff applied in SQL WHERE clause, not in Go post-processing, for efficiency
- **math.Exp** — standard library function for exponential decay calculation

### Key Design Decisions

- Knowledge decay is implemented through two complementary mechanisms:
  1. **Hard cutoff** — entries older than `maxAge` (90 days) are excluded entirely via SQL WHERE clause
  2. **Soft decay** — entries within the window are penalized by age via exponential decay in the scoring formula
- The 30-day half-life means an entry's recency score drops to 50% at 30 days, 25% at 60 days, 12.5% at 90 days
- Combined with the 70/30 weighting (keyword vs recency), an entry at 60 days old with a perfect keyword match still scores 0.7 + 0.25*0.3 = 0.775 — still highly relevant
- The `maxAge` is not yet exposed as a CLI flag or YAML config — future configuration story can add this

### Integration Points

- `internal/knowledge/store.go` — `maxAge` field, Search method with date filtering and recency scoring
- `internal/knowledge/store_test.go` — recency-weighted search tests

### References

- [Source: _bmad-output/planning-artifacts/prd.md#FR74]
- [Source: _bmad-output/planning-artifacts/epics.md#Story 10.3]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Knowledge decay via dual mechanism: hard 90-day cutoff + soft exponential recency decay
- 30-day half-life exponential decay in search scoring formula
- SQL-level date filtering prevents stale entries from being loaded into memory
- Recency boost weighted at 30% of combined score (70% keyword similarity)
- Configurable maxAge field on Store struct

### Change Log

- 2026-04-16: Story 10.3 implemented — knowledge decay with hard cutoff and soft exponential recency weighting

### File List

- internal/knowledge/store.go (modified — maxAge field, date filtering in Search)
- internal/knowledge/store_test.go (reference — search tests cover recency behavior)
