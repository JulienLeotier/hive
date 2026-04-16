# Story 14.1: Template Packaging & Publishing

Status: done

## Story

As a user,
I want to publish my hive configuration as a reusable template,
so that other users can benefit from my orchestration patterns.

## Acceptance Criteria

1. **Given** a working hive project with `hive.yaml` and agent configs
   **When** the user runs `hive publish --name my-template --description "..."`
   **Then** the system packages: `hive.yaml`, `agents/` directory, `README.md`, `metadata.json`

2. **Given** the packaging step succeeds
   **When** the publish command completes
   **Then** the package is pushed to the HiveHub Git registry

3. **Given** a project without a `hive.yaml`
   **When** the user runs `hive publish`
   **Then** the command fails with a clear error message indicating the missing configuration

4. **Given** a project with sensitive files (`.env`, credentials)
   **When** packaging runs
   **Then** sensitive files are excluded from the package

5. **Given** the HiveHub registry is unreachable
   **When** the user runs `hive publish`
   **Then** the command fails with a network error and suggested remediation

## Tasks / Subtasks

- [x] Task 1: Template metadata generation (AC: #1)
  - [x] Create `metadata.json` struct with name, description, author, version, category fields
  - [x] Auto-populate version from `hive.yaml` or default to `1.0.0`
  - [x] Auto-populate author from git config or environment
- [x] Task 2: Template packaging (AC: #1, #4)
  - [x] Collect files: `hive.yaml`, `agents/` directory contents, `README.md`
  - [x] Generate `metadata.json` from CLI flags and project metadata
  - [x] Exclude sensitive patterns: `.env*`, `*credentials*`, `*secret*`, `*.key`
  - [x] Create a tar.gz archive of the package contents
- [x] Task 3: Registry push (AC: #2, #5)
  - [x] Implement `Publish()` method on `Registry` struct in `internal/hivehub/registry.go`
  - [x] Push packaged template to HiveHub Git registry via HTTPS
  - [x] Handle network errors with clear messages and remediation hints
- [x] Task 4: CLI command (AC: #1, #3)
  - [x] Create `publishCmd` cobra command with `--name`, `--description`, `--category` flags
  - [x] Validate `hive.yaml` exists before packaging
  - [x] Display success message with template URL after publish

## Dev Notes

### Architecture Compliance

- `internal/hivehub/registry.go` -- `Registry` struct handles all HiveHub interactions (search, get, publish)
- CLI command registered in `internal/cli/` following existing cobra command patterns
- Uses `net/http` for registry API calls with 15s timeout (consistent with search endpoint)
- Sensitive file exclusion uses pattern matching, not a hardcoded list

### Key Design Decisions

- Templates are published as tar.gz archives to the HiveHub Git registry -- this keeps the registry lightweight and Git-friendly
- The exclusion list for sensitive files is applied during packaging, not at publish time -- prevents accidental inclusion even if the archive is inspected locally
- Author information is auto-detected from git config (`user.name`, `user.email`) with fallback to environment variables
- The publish command is intentionally simple (name + description) -- advanced metadata like categories and tags can be set in the registry UI

### Integration Points

- `internal/hivehub/registry.go` -- `Publish()` method added to existing `Registry` struct
- `internal/cli/` -- `publishCmd` cobra command
- `hive.yaml` -- source of template content and version metadata

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 14.1]
- [Source: _bmad-output/planning-artifacts/prd.md#FR89, FR92, FR93]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Template packaging collects hive.yaml, agents/ directory, README.md into tar.gz archive
- Metadata.json auto-generated with name, description, author, version, category
- Sensitive file exclusion via pattern matching (.env, credentials, secrets, keys)
- Registry.Publish() pushes package to HiveHub Git registry via HTTPS
- CLI command validates hive.yaml presence before packaging

### Change Log

- 2026-04-16: Story 14.1 implemented -- template packaging and publishing to HiveHub

### File List

- internal/hivehub/registry.go (modified -- added Publish method and packaging logic)
- internal/cli/init_cmd.go (modified -- added publishCmd cobra command)
