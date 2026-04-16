# Story 3.2: DAG Dependency Resolution

Status: done

## Story

As the system,
I want workflow task dependencies represented as a DAG,
so that execution order is optimized and deadlocks are impossible.

## Acceptance Criteria

1. **Given** a workflow with task dependencies **When** the DAG resolver processes the workflow **Then** it produces a topologically sorted execution plan grouped by parallel levels
2. **Given** tasks that can run in parallel (no mutual dependencies) **When** TopologicalSort is called **Then** they are grouped in the same level
3. **Given** a circular dependency (A -> B -> C -> A) **When** the DAG resolver processes the workflow **Then** it detects and rejects the cycle with a clear error message
4. **Given** a linear chain (A -> B -> C) **When** sorted **Then** each task is in its own level, producing 3 sequential levels (FR9)

## Tasks / Subtasks

- [x] Task 1: Implement detectCycles using Kahn's algorithm (AC: #3)
- [x] Task 2: Implement TopologicalSort returning [][]TaskDef grouped by levels (AC: #1, #2)
- [x] Task 3: Build adjacency list and in-degree map from task dependencies (AC: #1)
- [x] Task 4: Integrate cycle detection into workflow validation (AC: #3)
- [x] Task 5: Write tests for diamond pattern, flat graph, circular detection (AC: #1-#4)

## Dev Notes

- Kahn's algorithm chosen for both cycle detection and topological sorting
- detectCycles: if `visited != len(tasks)` after BFS, a cycle exists
- TopologicalSort: BFS with level tracking -- nodes dequeued in the same iteration form a parallel group
- Adjacency list built from `DependsOn` field: each dep edge goes from dependency to dependent
- Both functions share the same graph-building logic but serve different purposes
- detectCycles is called during `validate()` in the parser (catches cycles at parse time)
- TopologicalSort is called by the workflow engine and CLI validate command for execution planning

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### File List

- internal/workflow/parser.go (modified) -- detectCycles (Kahn's algorithm), TopologicalSort with level grouping
- internal/workflow/parser_test.go (modified) -- TestParseCircularDependency, TestTopologicalSort, TestTopologicalSortFlat
