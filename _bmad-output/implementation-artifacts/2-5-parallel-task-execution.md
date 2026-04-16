# Story 2.5: Parallel Task Execution

Status: done

## Story

As a user,
I want independent tasks to execute in parallel,
so that workflows complete as fast as possible.

## Acceptance Criteria

1. **Given** a workflow DAG where tasks A and B have no dependency on each other **When** both tasks are ready to execute **Then** the system dispatches both tasks concurrently
2. **Given** task C depends on both A and B **When** A and B are both running **Then** task C starts only after both A and B complete
3. **Given** the workflow DAG is processed **When** TopologicalSort is called **Then** tasks are grouped into parallel levels where tasks at the same level can execute simultaneously
4. **Given** a flat workflow with no dependencies **When** TopologicalSort is called **Then** all tasks are placed in a single parallel level (FR14)

## Tasks / Subtasks

- [x] Task 1: Implement TopologicalSort returning level-grouped task lists (AC: #1, #3)
- [x] Task 2: Group tasks by dependency level for parallel execution (AC: #1, #2)
- [x] Task 3: Handle flat DAGs (all tasks in one level) (AC: #4)
- [x] Task 4: Integrate with workflow parser DAG validation (AC: #3)
- [x] Task 5: Write tests for diamond DAG, flat DAG, linear chain (AC: #1-#4)

## Dev Notes

- TopologicalSort in `workflow/parser.go` uses Kahn's algorithm with level grouping
- Tasks at the same level have all dependencies satisfied and can run in parallel
- The sort returns `[][]TaskDef` where each inner slice is a parallel execution group
- Diamond pattern (A -> B,C -> D) produces 3 levels: [A], [B,C], [D]
- Flat pattern (A, B, C with no deps) produces 1 level: [A, B, C]
- Actual goroutine dispatch happens in the workflow execution engine (Story 3.3)
- Circular dependency detection reuses the same Kahn's algorithm (visited != total = cycle)

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### File List

- internal/workflow/parser.go (modified) -- TopologicalSort function with level-based parallel grouping
- internal/workflow/parser_test.go (modified) -- TestTopologicalSort (diamond), TestTopologicalSortFlat
