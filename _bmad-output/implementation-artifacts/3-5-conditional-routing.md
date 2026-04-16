# Story 3.5: Conditional Routing

Status: done

## Story

As a user,
I want workflow paths to branch based on task results,
so that my workflows can handle different outcomes intelligently.

## Acceptance Criteria

1. **Given** a workflow with conditional branches defined in YAML **When** a task definition includes a `condition` field **Then** the condition expression is stored in the TaskDef struct
2. **Given** a task completes with a result **When** the engine evaluates conditions **Then** it routes to the matching branch based on the condition expression (e.g., `result.score > 0.8`)
3. **Given** conditional branches in the DAG **When** conditions are defined **Then** they are validated as part of the workflow parsing (non-empty string check)
4. **Given** unmatched conditions with no default branch **When** evaluation fails to match **Then** a clear error is produced (FR11)

## Tasks / Subtasks

- [x] Task 1: Add Condition field to TaskDef struct (AC: #1)
- [x] Task 2: Include condition in YAML parsing (AC: #1)
- [x] Task 3: Support condition expressions in workflow validation (AC: #3)
- [x] Task 4: Enable condition-based routing in workflow engine execution flow (AC: #2, #4)

## Dev Notes

- TaskDef.Condition is a string field with `yaml:"condition,omitempty"` tag
- Condition expressions follow a simple format: `result.score > 0.8`, `result.status == "pass"`
- Conditions are optional -- tasks without conditions execute unconditionally based on dependency completion
- The condition evaluation engine operates at the workflow execution layer, checking task output against condition expressions
- Missing default branch handling produces descriptive errors identifying which condition was unmatched
- This is a foundational feature -- advanced condition syntax (CEL, jsonpath) can be added later

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### File List

- internal/workflow/parser.go (modified) -- TaskDef.Condition field added
- internal/workflow/workflow.go (modified) -- Condition-aware execution in workflow engine
