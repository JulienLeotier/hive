# Story 3.6: Workflow Validation

Status: done

## Story

As a user,
I want to validate my workflow config before running it,
so that I catch errors early without wasting agent compute.

## Acceptance Criteria

1. **Given** a `hive.yaml` workflow definition **When** the user runs `hive validate` **Then** the system checks YAML syntax, task names, types, dependency DAG validity, and trigger configuration
2. **Given** validation succeeds **When** all checks pass **Then** the CLI displays workflow name, task count, parallel level count, and trigger info **And** exits with code 0
3. **Given** validation fails **When** any check fails **Then** the CLI displays a `FAIL:` message with the specific error **And** exits with non-zero code
4. **Given** the validate command **When** called with an optional argument **Then** it validates the specified file (default: `hive.yaml` in current directory) (FR12)

## Tasks / Subtasks

- [x] Task 1: Implement `hive validate` CLI command with Cobra (AC: #1, #4)
- [x] Task 2: Wire ParseFile for YAML syntax and structural validation (AC: #1)
- [x] Task 3: Wire TopologicalSort for DAG validation and level counting (AC: #1, #2)
- [x] Task 4: Display success output with workflow stats (AC: #2)
- [x] Task 5: Display failure output with specific error message (AC: #3)
- [x] Task 6: Support optional file path argument (default hive.yaml) (AC: #4)

## Dev Notes

- The validate command reuses `workflow.ParseFile` and `workflow.TopologicalSort` -- no duplicate validation logic
- Validation is comprehensive: YAML syntax -> struct validation -> dependency DAG check -> cycle detection
- Success output shows: workflow name, task count, parallel levels count, trigger type (if defined)
- Error output prefixed with `FAIL:` for clear pass/fail distinction in CI pipelines
- Command registered as `validateCmd` in cli package with `cobra.MaximumNArgs(1)`
- The `--json` flag for CI integration (mentioned in FR12) can be added as an enhancement

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### File List

- internal/cli/validate.go (new) -- hive validate command implementation
- internal/workflow/parser.go (dependency) -- ParseFile, TopologicalSort used by validate
