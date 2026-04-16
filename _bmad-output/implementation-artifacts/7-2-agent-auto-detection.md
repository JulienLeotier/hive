# Story 7.2: Agent Auto-Detection

Status: done

## Story

As a user,
I want `hive add-agent` to auto-detect agent type from project structure,
so that registration is as simple as pointing to a directory.

## Acceptance Criteria

1. **Given** a directory containing a Claude Code skill, MCP server config, or HTTP agent
   **When** the user runs `hive add-agent --path ./my-agent`
   **Then** the system detects the agent type from project files (e.g., `skill.md` -> Claude Code, `mcp.json` -> MCP)

2. **Given** a detected agent type
   **When** the detection completes
   **Then** the adapter is auto-configured based on detected project structure

3. **Given** detection succeeds
   **When** the type is determined
   **Then** the detected type is confirmed with the user before registering

## Tasks / Subtasks

- [x] Task 1: File-based agent type detection (AC: #1)
  - [x] Implement detection logic scanning directory for marker files:
    - `skill.md` or `SKILL.md` -> Claude Code agent
    - `mcp.json` or `mcp.yaml` -> MCP agent
    - `Dockerfile` or `docker-compose.yml` with HTTP endpoints -> HTTP agent
    - `package.json` with MCP dependencies -> MCP agent
  - [x] Return detected type and confidence level
- [x] Task 2: Auto-configuration from detected type (AC: #2)
  - [x] Claude Code: extract capabilities from skill definition file
  - [x] MCP: parse mcp.json for server configuration and tool list
  - [x] HTTP: look for configuration files with endpoint definitions
- [x] Task 3: CLI integration with --path flag (AC: #1, #3)
  - [x] Add `--path` flag to `hive add-agent` command
  - [x] When `--path` is provided without `--type`, trigger auto-detection
  - [x] Display detected type and prompt for confirmation
  - [x] Proceed with registration using auto-configured adapter
- [x] Task 4: Graceful fallback (AC: #1)
  - [x] If no agent type can be detected, display helpful error with supported types
  - [x] Suggest using `--type` flag for manual specification

## Dev Notes

### Architecture Compliance

- Detection is file-system based — no network calls needed for type detection
- Uses existing adapter types: `claude-code`, `mcp`, `http` matching the adapter package implementations
- Auto-detection is a convenience layer on top of the existing registration flow
- Marker file patterns are based on real-world conventions for each agent framework

### Key Design Decisions

- Detection uses a priority list: Claude Code skill files are checked first (most specific), then MCP configs, then HTTP (most generic)
- The user is asked to confirm the detected type before registration proceeds — prevents miscategorization
- Auto-detection only works with `--path`; using `--type` explicitly skips detection
- Unknown directory structures produce a clear error listing all supported detection patterns

### Detection Patterns

| File Pattern | Detected Type | Confidence |
|---|---|---|
| `skill.md`, `SKILL.md` | claude-code | high |
| `mcp.json`, `mcp.yaml` | mcp | high |
| `package.json` with MCP deps | mcp | medium |
| `Dockerfile` + HTTP config | http | medium |
| `agent.yaml` | http | medium |

### Integration Points

- `internal/cli/agent.go` — `--path` flag and auto-detection logic in `add-agent` command
- `internal/adapter/claude_code.go` — Claude Code agent detection and capability extraction
- `internal/adapter/mcp.go` — MCP server config parsing
- `internal/adapter/http.go` — HTTP agent configuration detection

### References

- [Source: _bmad-output/planning-artifacts/prd.md#FR2]

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Completion Notes List

- File-based auto-detection for Claude Code (skill.md), MCP (mcp.json), and HTTP (Dockerfile) agents
- Priority-based detection with confidence levels
- User confirmation before registration prevents miscategorization
- Graceful fallback with helpful error messages listing supported patterns

### Change Log

- 2026-04-16: Story 7.2 implemented — agent auto-detection from project structure

### File List

- internal/cli/agent.go (modified — added --path flag and auto-detection logic)
- internal/adapter/claude_code.go (reference — Claude Code detection patterns)
- internal/adapter/mcp.go (reference — MCP config parsing)
- internal/adapter/http.go (reference — HTTP adapter)
