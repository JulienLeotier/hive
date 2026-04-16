# Story 7.5: Example Template -- Code Review Hive

Status: done

## Story

As a new user,
I want a pre-built "code review" hive template,
so that I can see a real orchestration example immediately.

## Acceptance Criteria

1. **Given** the user runs `hive init --template code-review`
   **When** initialization completes
   **Then** the project includes: workflow that takes a PR URL, routes to a code-review agent, then a summary agent

2. **Given** the generated project
   **When** the user examines the files
   **Then** it contains example agent configs for HTTP-based review and summary agents
   **And** documentation explaining the workflow

## Tasks / Subtasks

- [x] Task 1: Code review workflow template (AC: #1)
  - [x] Define code-review template in `getTemplate()` function
  - [x] Workflow has 2 tasks: `review` (type: code-review) and `summarize` (type: summarize)
  - [x] `summarize` depends on `review` — sequential pipeline
  - [x] Review task input accepts PR source
  - [x] Summarize task specifies markdown output format
- [x] Task 2: Template integration with hive init (AC: #1)
  - [x] Template is selectable via `hive init --template code-review`
  - [x] Generated `hive.yaml` is valid and parseable by `workflow.ParseFile()`
  - [x] DAG validation passes (no cycles, valid dependencies)
- [x] Task 3: Example agent configs and documentation (AC: #2)
  - [x] Generated agents directory includes example config files
  - [x] README explains the code review workflow pattern
  - [x] Next steps guide users to register actual agents

## Dev Notes

### Architecture Compliance

- Template is built into `internal/cli/init_cmd.go` as a Go string in the `getTemplate()` function
- Generated YAML follows the `workflow.Config` schema with `tasks` and `depends_on`
- Task types use capability names (`code-review`, `summarize`) for loose coupling with agents
- Template is one of three built-in templates alongside `content-pipeline` and `research`

### Template YAML

```yaml
name: my-project
tasks:
  - name: review
    type: code-review
    input:
      source: pr
  - name: summarize
    type: summarize
    depends_on: [review]
    input:
      format: markdown
```

### Key Design Decisions

- The code review template demonstrates a simple 2-task sequential pipeline — ideal for showing dependencies without overwhelming new users
- Task types are generic capabilities, not tied to specific agents — users can plug in any agent that declares the `code-review` capability
- Input uses template variables where applicable for configurability

### Integration Points

- `internal/cli/init_cmd.go` — `getTemplate("code-review", projectName)` case
- `internal/workflow/parser.go` — validates generated YAML

### References

- [Source: _bmad-output/planning-artifacts/prd.md#FR34, FR35]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Code review template generates 2-task sequential workflow: review -> summarize
- Template produces valid hive.yaml compatible with workflow parser
- Review task accepts PR input, summarize task outputs markdown
- Integrated into hive init --template code-review

### Change Log

- 2026-04-16: Story 7.5 implemented — code review hive template

### File List

- internal/cli/init_cmd.go (modified — code-review template case in getTemplate)
