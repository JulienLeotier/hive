# Story 3.3: Workflow Execution Engine

Status: done

## Story

As a user,
I want to run a workflow end-to-end with `hive run`,
so that I can see my agents collaborate on a complete task.

## Acceptance Criteria

1. **Given** a valid workflow definition and registered agents **When** the workflow is executed **Then** the engine creates tasks for each workflow step in the `tasks` table
2. **Given** tasks are created from the workflow **When** execution proceeds **Then** tasks are routed to agents via the capability router (FindCapableAgent)
3. **Given** tasks at the same DAG level **When** they are ready **Then** the engine dispatches them for parallel execution using TopologicalSort levels
4. **Given** task results are produced **When** upstream tasks complete **Then** results are available to downstream tasks as defined in the workflow
5. **Given** workflow execution **When** it starts and completes **Then** workflow-level events are emitted (`workflow.started`, `workflow.completed`, `workflow.failed`)

## Tasks / Subtasks

- [x] Task 1: Implement workflow.Store with Create, UpdateStatus, GetByID, List (AC: #5)
- [x] Task 2: Wire UpdateStatus to emit workflow events (started, completed, failed) (AC: #5)
- [x] Task 3: Store workflow config as JSON in workflows table (AC: #1)
- [x] Task 4: Integrate TopologicalSort for level-based execution planning (AC: #3)
- [x] Task 5: Connect task creation via task.Store.Create from workflow task definitions (AC: #1)
- [x] Task 6: Connect task routing via task.Router.FindCapableAgent (AC: #2)

## Dev Notes

- Workflow Store is the persistence layer; the execution engine orchestrates Store + Router + TaskStore
- UpdateStatus maps status constants to event types: running->workflow.started, completed->workflow.completed, failed->workflow.failed
- Workflow config stored as JSON-serialized Config struct in the `config` column
- List returns workflows ordered by creation time descending with a 1000-row limit
- The full orchestration loop (create tasks -> route -> invoke -> collect results) is coordinated by the CLI `hive run` command and the API server
- Parallel execution at each DAG level uses goroutines dispatched per level from TopologicalSort output

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### File List

- internal/workflow/workflow.go (new) -- Workflow struct, Store with Create/UpdateStatus/GetByID/List
- internal/workflow/parser.go (modified) -- TopologicalSort integration for execution planning
- internal/task/task.go (modified) -- Create method used by engine to create workflow tasks
- internal/task/router.go (modified) -- FindCapableAgent used for routing during execution
