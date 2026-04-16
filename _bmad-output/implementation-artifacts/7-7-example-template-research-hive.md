# Story 7.7: Example Template -- Research Hive

Status: done

## Story

As a new user,
I want a pre-built "research" hive template,
so that I can see parallel research agent orchestration.

## Acceptance Criteria

1. **Given** the user runs `hive init --template research`
   **When** initialization completes
   **Then** the project includes: workflow with parallel research agents -> aggregator -> report generator

2. **Given** the generated project
   **When** the user examines the files
   **Then** it contains example agent configs for research and synthesis
   **And** documentation explaining the research pattern

## Tasks / Subtasks

- [x] Task 1: Research workflow template (AC: #1)
  - [x] Define research template in `getTemplate()` function
  - [x] Workflow has 4 tasks: `search-a`, `search-b` (parallel), `aggregate`, `report`
  - [x] `search-a` and `search-b` have no mutual dependencies — execute in parallel
  - [x] `aggregate` depends on both `search-a` and `search-b` — fan-in pattern
  - [x] `report` depends on `aggregate` — final sequential step
  - [x] Search tasks accept query as template variable `{{query}}` with different sources
- [x] Task 2: Template integration with hive init (AC: #1)
  - [x] Template is selectable via `hive init --template research`
  - [x] Generated `hive.yaml` is valid and parseable
  - [x] DAG validation passes with parallel branches correctly detected
  - [x] TopologicalSort produces correct levels: [search-a, search-b], [aggregate], [report]
- [x] Task 3: Documentation (AC: #2)
  - [x] README explains the research pattern with parallel fan-out and fan-in
  - [x] Documents the DAG structure and parallel execution benefits

## Dev Notes

### Architecture Compliance

- Template is built into `internal/cli/init_cmd.go` as a Go string in the `getTemplate()` function
- Generated YAML follows the `workflow.Config` schema with DAG dependencies
- Demonstrates the parallel execution capability: two search tasks run concurrently
- Fan-in pattern: aggregate task waits for both parallel search tasks

### Template YAML

```yaml
name: my-project
tasks:
  - name: search-a
    type: research
    input:
      query: "{{query}}"
      source: academic
  - name: search-b
    type: research
    input:
      query: "{{query}}"
      source: web
  - name: aggregate
    type: summarize
    depends_on: [search-a, search-b]
  - name: report
    type: report-generate
    depends_on: [aggregate]
```

### Key Design Decisions

- The research template is the only template that demonstrates parallel execution — `search-a` and `search-b` have no dependency on each other
- This is the key differentiation from the sequential templates (code-review, content-pipeline)
- Both search tasks use the same capability type (`research`) but different input sources — shows how the same agent type can handle varied inputs
- The fan-out/fan-in pattern is a fundamental DAG pattern in workflow orchestration

### DAG Visualization

```
search-a ──┐
            ├── aggregate ── report
search-b ──┘
```

### Integration Points

- `internal/cli/init_cmd.go` — `getTemplate("research", projectName)` case
- `internal/workflow/parser.go` — validates generated YAML, detects parallel branches
- `internal/workflow/parser.go` — `TopologicalSort()` groups search-a and search-b at the same level

### References

- [Source: _bmad-output/planning-artifacts/prd.md#FR34, FR35]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Research template generates 4-task DAG with parallel fan-out (search-a, search-b) and fan-in (aggregate)
- Demonstrates parallel execution — the key DAG feature missing from sequential templates
- Both search tasks use same capability with different sources
- Integrated into hive init --template research

### Change Log

- 2026-04-16: Story 7.7 implemented — research hive template with parallel execution pattern

### File List

- internal/cli/init_cmd.go (modified — research template case in getTemplate)
