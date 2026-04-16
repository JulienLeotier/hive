# Story 3.1: YAML Workflow Parser

Status: done

## Story

As a user,
I want to define workflows in a `hive.yaml` file,
so that I can declaratively describe how agents collaborate.

## Acceptance Criteria

1. **Given** a `hive.yaml` file with a workflow definition **When** the parser reads the file **Then** it produces a validated `Config` struct with name, tasks, dependencies, and triggers
2. **Given** tasks in the workflow **When** they reference capabilities **Then** tasks use `type` field for capability matching (e.g., `code-review`) not agent names, enabling loose coupling
3. **Given** invalid YAML **When** the parser encounters errors **Then** it returns clear error messages including what validation failed (missing name, missing type, etc.)
4. **Given** a valid workflow **When** it is parsed and stored **Then** the workflow is persisted in the `workflows` table with config as JSON (FR8)
5. **Given** a workflow with a trigger definition **When** parsed **Then** the trigger type (manual, webhook, schedule) and its configuration are available

## Tasks / Subtasks

- [x] Task 1: Define Config, TaskDef, TriggerDef structs with YAML tags (AC: #1, #2, #5)
- [x] Task 2: Implement ParseFile to read and parse YAML from disk (AC: #1)
- [x] Task 3: Implement Parse for raw YAML bytes (AC: #1)
- [x] Task 4: Implement validate with checks for name, tasks, types, duplicates (AC: #3)
- [x] Task 5: Validate dependency references (unknown deps, self-deps) (AC: #3)
- [x] Task 6: Implement workflow.Store.Create to persist to SQLite (AC: #4)
- [x] Task 7: Write tests for valid workflow, missing name, no tasks, duplicate names, unknown deps, self-deps, triggers (AC: #1-#5)

## Dev Notes

- Config struct uses `yaml.v3` struct tags for direct YAML-to-Go mapping
- TaskDef.Type is the capability requirement, not an agent name -- this enables loose coupling per architecture
- TaskDef.Condition field supports conditional routing expressions (used in Story 3.5)
- Validation is exhaustive: name required, at least one task, task names unique, types required, deps valid
- Parser errors are user-facing and descriptive -- no raw stack traces
- workflow.Store uses ULID for workflow IDs and JSON-serialized config in the `workflows` table

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### File List

- internal/workflow/parser.go (new) -- Config/TaskDef/TriggerDef types, ParseFile, Parse, validate
- internal/workflow/parser_test.go (new) -- 8 tests covering valid parse, missing name, no tasks, duplicates, unknown deps, self-deps, triggers
- internal/workflow/workflow.go (new) -- Store.Create persisting workflow to SQLite
