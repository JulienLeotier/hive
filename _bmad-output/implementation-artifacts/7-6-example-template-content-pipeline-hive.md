# Story 7.6: Example Template -- Content Pipeline Hive

Status: done

## Story

As a new user,
I want a pre-built "content pipeline" hive template,
so that I can see multi-agent content production in action.

## Acceptance Criteria

1. **Given** the user runs `hive init --template content-pipeline`
   **When** initialization completes
   **Then** the project includes: workflow with writer -> editor -> SEO optimizer -> publisher stages

2. **Given** the generated project
   **When** the user examines the files
   **Then** it contains example agent configs for each stage
   **And** documentation explaining the pipeline

## Tasks / Subtasks

- [x] Task 1: Content pipeline workflow template (AC: #1)
  - [x] Define content-pipeline template in `getTemplate()` function
  - [x] Workflow has 4 tasks: `write` -> `edit` -> `optimize` -> `publish`
  - [x] Each task depends on the previous one — fully sequential pipeline
  - [x] Write task accepts topic as template variable `{{topic}}`
  - [x] Task types: `content-write`, `content-edit`, `seo-optimize`, `publish`
- [x] Task 2: Template integration with hive init (AC: #1)
  - [x] Template is selectable via `hive init --template content-pipeline`
  - [x] Generated `hive.yaml` is valid and parseable
  - [x] DAG validation passes (linear chain, no cycles)
- [x] Task 3: Documentation (AC: #2)
  - [x] README explains the content pipeline pattern
  - [x] Documents each stage's purpose and expected input/output

## Dev Notes

### Architecture Compliance

- Template is built into `internal/cli/init_cmd.go` as a Go string in the `getTemplate()` function
- Generated YAML follows the `workflow.Config` schema
- 4-stage linear pipeline demonstrates sequential task dependencies
- Template variable syntax `{{topic}}` shows how workflows accept dynamic input

### Template YAML

```yaml
name: my-project
tasks:
  - name: write
    type: content-write
    input:
      topic: "{{topic}}"
  - name: edit
    type: content-edit
    depends_on: [write]
  - name: optimize
    type: seo-optimize
    depends_on: [edit]
  - name: publish
    type: publish
    depends_on: [optimize]
```

### Key Design Decisions

- The content pipeline template demonstrates a longer sequential chain (4 tasks) compared to code review (2 tasks) — shows how dependencies scale
- Each stage has a distinct capability type, demonstrating that different agents can handle different parts of the pipeline
- The `{{topic}}` template variable demonstrates parameterized workflow input
- The pipeline pattern is common in content production, making it immediately relatable

### Integration Points

- `internal/cli/init_cmd.go` — `getTemplate("content-pipeline", projectName)` case
- `internal/workflow/parser.go` — validates generated YAML

### References

- [Source: _bmad-output/planning-artifacts/prd.md#FR34, FR35]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Content pipeline template generates 4-task sequential workflow: write -> edit -> optimize -> publish
- Demonstrates longer dependency chains and template variable syntax
- Each stage uses distinct capability type for multi-agent orchestration
- Integrated into hive init --template content-pipeline

### Change Log

- 2026-04-16: Story 7.6 implemented — content pipeline hive template

### File List

- internal/cli/init_cmd.go (modified — content-pipeline template case in getTemplate)
