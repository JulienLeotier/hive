# Story 7.8: Shell Completion

Status: done

## Story

As a power user,
I want shell completion for all hive commands,
so that I can work faster in the terminal.

## Acceptance Criteria

1. **Given** the user runs `hive completion bash` (or zsh/fish)
   **When** the output is sourced in the shell
   **Then** tab completion works for: all commands and subcommands, `--flags` for each command, agent names for agent-specific commands, workflow names for workflow commands

2. **Given** the user needs setup instructions
   **When** they run `hive completion --help`
   **Then** installation instructions are displayed for each supported shell

## Tasks / Subtasks

- [x] Task 1: Shell completion command (AC: #1, #2)
  - [x] Leverage cobra's built-in shell completion generation
  - [x] Register `hive completion` command with subcommands: `bash`, `zsh`, `fish`
  - [x] Each subcommand outputs the completion script to stdout
  - [x] Help text includes installation instructions per shell
- [x] Task 2: Bash completion (AC: #1)
  - [x] `hive completion bash` outputs bash completion script
  - [x] Instructions: `source <(hive completion bash)` or append to `.bashrc`
- [x] Task 3: Zsh completion (AC: #1)
  - [x] `hive completion zsh` outputs zsh completion script
  - [x] Instructions: `hive completion zsh > "${fpath[1]}/_hive"` and reload
- [x] Task 4: Fish completion (AC: #1)
  - [x] `hive completion fish` outputs fish completion script
  - [x] Instructions: `hive completion fish | source` or write to completions dir
- [x] Task 5: Dynamic completions (AC: #1)
  - [x] Register dynamic completion for agent names (used by remove-agent, agent swap, etc.)
  - [x] Register dynamic completion for workflow names
  - [x] Cobra's `ValidArgsFunction` or `RegisterFlagCompletionFunc` for dynamic values

## Dev Notes

### Architecture Compliance

- Uses cobra's built-in completion generation — no custom completion engine needed
- Cobra automatically completes all registered commands, subcommands, and flags
- Dynamic completions query the database for agent and workflow names
- NFR20: Shell completion for bash, zsh, and fish

### Key Design Decisions

- Leverages cobra's `GenBashCompletion()`, `GenZshCompletion()`, `GenFishCompletion()` methods — these are maintained by the cobra community and handle edge cases
- Dynamic completions for agent names query the database lazily — only when tab is pressed, not on every command invocation
- Help text for each shell includes the exact installation command to minimize setup friction
- Completion scripts are output to stdout for maximum flexibility (user can pipe, redirect, or source)

### Shell Setup Examples

```bash
# Bash
echo 'source <(hive completion bash)' >> ~/.bashrc

# Zsh
hive completion zsh > "${fpath[1]}/_hive"

# Fish
hive completion fish | source
```

### Integration Points

- `internal/cli/root.go` — completion command registration (cobra built-in)
- `internal/cli/agent.go` — dynamic agent name completion for agent-specific commands
- `internal/agent/manager.go` — `List()` for agent name completion
- `internal/workflow/workflow.go` — `List()` for workflow name completion

### References

- [Source: _bmad-output/planning-artifacts/prd.md#NFR20]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- Shell completion for bash, zsh, and fish using cobra's built-in generation
- Dynamic completions for agent and workflow names from database
- Help text includes per-shell installation instructions
- Completion scripts output to stdout for flexible installation

### Change Log

- 2026-04-16: Story 7.8 implemented — shell completion for bash, zsh, and fish

### File List

- internal/cli/root.go (modified — completion command registration)
- internal/cli/agent.go (modified — dynamic agent name completion)
- internal/agent/manager.go (reference — List for agent names)
- internal/workflow/workflow.go (reference — List for workflow names)
