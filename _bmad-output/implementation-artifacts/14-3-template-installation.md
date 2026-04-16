# Story 14.3: Template Installation

Status: done

## Story

As a user,
I want to install a HiveHub template into my project,
so that I can start with a proven orchestration pattern.

## Acceptance Criteria

1. **Given** a template exists in HiveHub
   **When** the user runs `hive install content-pipeline`
   **Then** the template files are downloaded and merged into the current project

2. **Given** existing files in the project directory
   **When** a template contains files that conflict with existing ones
   **Then** existing files are not overwritten without confirmation

3. **Given** a template name that does not exist
   **When** the user runs `hive install nonexistent-template`
   **Then** the command fails with a clear "template not found" error

4. **Given** the HiveHub registry is unreachable
   **When** the user runs `hive install`
   **Then** the command fails with a network error and suggested remediation

5. **Given** a template with agents/ directory
   **When** installation completes
   **Then** agent configuration files are placed in the project's agents/ directory

## Tasks / Subtasks

- [x] Task 1: Template lookup (AC: #1, #3)
  - [x] Use `Registry.Get(name)` to find the template by name
  - [x] Return "template not found" error with suggestion to run `hive search` if not found
- [x] Task 2: Template download (AC: #1, #4)
  - [x] Download template archive from the template's URL field
  - [x] Handle network errors with descriptive messages
  - [x] Extract tar.gz archive to temporary directory
- [x] Task 3: Conflict detection and merge (AC: #2, #5)
  - [x] Scan template files against current project directory
  - [x] Detect file conflicts (existing files that would be overwritten)
  - [x] Prompt user for confirmation on each conflict (skip/overwrite/abort)
  - [x] Copy non-conflicting files directly
  - [x] Create `agents/` subdirectory if it does not exist
- [x] Task 4: CLI command (AC: #1, #3)
  - [x] Create `installCmd` cobra command accepting template name as argument
  - [x] Display progress during download and extraction
  - [x] Show summary of installed files after completion

## Dev Notes

### Architecture Compliance

- `internal/hivehub/registry.go` -- `Registry.Get()` for template lookup, download logic
- CLI command registered in `internal/cli/` following existing cobra patterns
- File operations use `os.MkdirAll` for directory creation and `os.OpenFile` for safe writes
- Conflict detection checks file existence before write, never silently overwrites

### Key Design Decisions

- Template installation merges into the current directory rather than creating a new subdirectory -- this matches how `hive init --template` works and allows layering templates
- Conflict resolution is interactive (prompt per file) rather than all-or-nothing -- gives users fine-grained control
- Template archives are extracted to a temp directory first, then selectively copied -- prevents partial installation on conflict abort
- The `agents/` directory is always created during installation since templates typically include agent configs

### Integration Points

- `internal/hivehub/registry.go` -- `Registry.Get()` for template metadata, download URL
- `internal/cli/init_cmd.go` -- `installCmd` cobra command
- Local filesystem -- target project directory for file merge

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 14.3]
- [Source: _bmad-output/planning-artifacts/prd.md#FR91]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Template lookup via Registry.Get() with clear "not found" error
- Download and extract tar.gz archive to temp directory
- Conflict detection with interactive prompt (skip/overwrite/abort)
- Files merged into current project directory, agents/ auto-created
- Summary of installed files displayed on completion

### Change Log

- 2026-04-16: Story 14.3 implemented -- HiveHub template installation with conflict detection

### File List

- internal/hivehub/registry.go (modified -- added download and extraction logic)
- internal/cli/init_cmd.go (modified -- added installCmd cobra command)
