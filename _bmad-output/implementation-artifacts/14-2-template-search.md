# Story 14.2: Template Search

Status: done

## Story

As a user,
I want to search for templates by keyword or category,
so that I can find relevant orchestration patterns quickly.

## Acceptance Criteria

1. **Given** the HiveHub registry contains templates
   **When** the user runs `hive search code-review`
   **Then** matching templates are displayed with: name, description, author, download count

2. **Given** a search query
   **When** the search executes
   **Then** results are fetched from the HiveHub Git registry index

3. **Given** an empty search query
   **When** the user runs `hive search`
   **Then** all available templates are listed

4. **Given** a search query with no matches
   **When** the search completes
   **Then** a helpful message is shown: "No templates found matching '<query>'"

5. **Given** the HiveHub registry is unreachable
   **When** the user runs `hive search`
   **Then** the command fails with a network error and suggested remediation

## Tasks / Subtasks

- [x] Task 1: Registry search implementation (AC: #1, #2, #3)
  - [x] Implement `Search(query)` on `Registry` struct in `internal/hivehub/registry.go`
  - [x] Fetch `index.json` from HiveHub Git registry via HTTPS
  - [x] Parse index into `[]Template` structs
  - [x] Filter templates by case-insensitive keyword matching across name, description, and category
  - [x] Return all templates when query is empty
- [x] Task 2: Registry index fetching (AC: #2, #5)
  - [x] Implement `fetchIndex()` helper to GET the registry index URL
  - [x] Use `io.LimitReader` with 10MB cap on response body
  - [x] Handle HTTP errors and network failures with descriptive messages
  - [x] Use 15s HTTP client timeout
- [x] Task 3: Template struct (AC: #1)
  - [x] Define `Template` struct with Name, Description, Author, Version, Category, URL, Downloads fields
  - [x] JSON struct tags for index.json deserialization
- [x] Task 4: CLI command (AC: #1, #4)
  - [x] Create `searchCmd` cobra command accepting optional query argument
  - [x] Display results in table format: NAME, DESCRIPTION, AUTHOR, DOWNLOADS
  - [x] Show "No templates found" message for empty results

## Dev Notes

### Architecture Compliance

- `internal/hivehub/registry.go` -- `Registry` struct handles all HiveHub interactions
- Default registry URL points to `https://raw.githubusercontent.com/JulienLeotier/hivehub/main/index.json`
- Uses `net/http` with 15s timeout for registry API calls
- Response body capped at 10MB via `io.LimitReader` to prevent memory exhaustion
- Search is case-insensitive, matching against concatenated name + description + category text

### Key Design Decisions

- The registry index is a single `index.json` file hosted on GitHub -- this keeps the infrastructure simple and avoids running a dedicated API server
- Search uses simple string containment rather than fuzzy matching -- straightforward and predictable for users
- The `Template` struct includes `Downloads` count for social proof in search results, even though the count is managed server-side
- `fetchIndex()` is called on every search to get fresh results -- caching is deferred as a future optimization

### Integration Points

- `internal/hivehub/registry.go` -- `Registry.Search()` and `Registry.Get()` methods
- `internal/cli/init_cmd.go` -- `searchCmd` cobra command
- HiveHub Git registry -- external `index.json` data source

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 14.2]
- [Source: _bmad-output/planning-artifacts/prd.md#FR90]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Registry.Search() fetches index.json and filters by case-insensitive keyword matching
- Template struct with full metadata fields (name, description, author, version, category, URL, downloads)
- fetchIndex() with 10MB response limit and 15s timeout
- CLI searchCmd displays results in table format with helpful empty-result message
- Empty query returns all templates for browsing

### Change Log

- 2026-04-16: Story 14.2 implemented -- HiveHub template search via registry index

### File List

- internal/hivehub/registry.go (new -- Registry struct with Search, Get, fetchIndex)
- internal/cli/init_cmd.go (modified -- added searchCmd cobra command)
