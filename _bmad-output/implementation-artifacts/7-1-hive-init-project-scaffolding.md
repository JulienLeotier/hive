# Story 7.1: hive init -- Project Scaffolding

Status: done

## Story

As a new user,
I want to scaffold a hive project with one command,
so that I can start orchestrating agents immediately.

## Acceptance Criteria

1. **Given** the user runs `hive init my-project`
   **When** scaffolding completes
   **Then** a directory `my-project/` is created with: `hive.yaml` (workflow config), `agents/` directory with example agent configs, `README.md` with quickstart instructions

2. **Given** a template is specified
   **When** the user runs `hive init --template code-review`
   **Then** the project uses the code review template for `hive.yaml`

3. **Given** no template is specified
   **When** the user runs `hive init` with no arguments
   **Then** the project name defaults to `my-hive` with a generic starter template

## Tasks / Subtasks

- [x] Task 1: Init command implementation (AC: #1, #3)
  - [x] Create `initCmd` cobra command in `internal/cli/init_cmd.go`
  - [x] Accept optional project name argument (default: `my-hive`)
  - [x] Create project directory with `0755` permissions
  - [x] Generate `hive.yaml` from template
  - [x] Create `agents/` directory with `.gitkeep`
  - [x] Generate `README.md` with quickstart instructions
- [x] Task 2: Template system (AC: #2)
  - [x] Implement `--template` flag supporting: `code-review`, `content-pipeline`, `research`
  - [x] Create `getTemplate()` function with switch on template name
  - [x] Default template generates a simple example workflow
  - [x] Each template produces valid `hive.yaml` with appropriate tasks and dependencies
- [x] Task 3: Output and guidance (AC: #1, #3)
  - [x] Display created files after scaffolding
  - [x] Show template name if used
  - [x] Print next steps: `cd <project> && hive add-agent ...`

## Dev Notes

### Architecture Compliance

- CLI command registered via cobra in `internal/cli/init_cmd.go`
- Templates are hardcoded Go strings — no external template engine dependency (NFR14: zero external dependencies)
- Project structure follows the convention: `hive.yaml` at root, `agents/` for agent configs
- Generated YAML is valid and parseable by `workflow.ParseFile()`

### Key Design Decisions

- Templates are built into the binary rather than fetched from a registry — keeps the tool self-contained and offline-capable
- Default project name is `my-hive` when no argument is provided
- `.gitkeep` in `agents/` directory ensures the directory is tracked by git
- README includes the 3-step quickstart: add-agent, run, status
- Template YAML uses the same schema as `workflow.Config` for consistency

### Template Summary

- **default**: Single example task — minimal starting point
- **code-review**: review -> summarize pipeline (2 tasks, 1 dependency)
- **content-pipeline**: write -> edit -> optimize -> publish (4 tasks, 3 dependencies)
- **research**: parallel search-a + search-b -> aggregate -> report (4 tasks, DAG with parallelism)

### Integration Points

- `internal/cli/init_cmd.go` — init command and template generation
- `internal/cli/root.go` — command registration
- `internal/workflow/parser.go` — generated YAML compatible with parser

### References

- [Source: _bmad-output/planning-artifacts/prd.md#FR34, FR35]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Init command scaffolds project directory with hive.yaml, agents/, and README.md
- Three built-in templates: code-review, content-pipeline, research
- Default template provides minimal starting point for new users
- Next-steps guidance printed after scaffolding

### Change Log

- 2026-04-16: Story 7.1 implemented — hive init with template-based project scaffolding

### File List

- internal/cli/init_cmd.go (new)
- internal/cli/root.go (reference — command registration)
