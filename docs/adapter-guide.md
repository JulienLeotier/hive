# Adapter Guide -- Writing a Hive Adapter

An adapter is the bridge between Hive and an AI agent. Every adapter implements the `Adapter` interface defined in `internal/adapter/adapter.go`.

## The Adapter Interface

```go
type Adapter interface {
    Declare(ctx context.Context) (AgentCapabilities, error)
    Invoke(ctx context.Context, task Task) (TaskResult, error)
    Health(ctx context.Context) (HealthStatus, error)
    Checkpoint(ctx context.Context) (Checkpoint, error)
    Resume(ctx context.Context, cp Checkpoint) error
}
```

These 5 methods are the complete Agent Adapter Protocol.

## Protocol Types

```go
type AgentCapabilities struct {
    Name       string   `json:"name"`
    TaskTypes  []string `json:"task_types"`
    CostPerRun float64  `json:"cost_per_run,omitempty"`
}

type Task struct {
    ID    string `json:"id"`
    Type  string `json:"type"`
    Input any    `json:"input"`
}

type TaskResult struct {
    TaskID string `json:"task_id"`
    Status string `json:"status"` // "completed" or "failed"
    Output any    `json:"output,omitempty"`
    Error  string `json:"error,omitempty"`
}

type HealthStatus struct {
    Status  string `json:"status"` // "healthy", "degraded", "unavailable"
    Message string `json:"message,omitempty"`
}

type Checkpoint struct {
    Data any `json:"data"`
}
```

## Method Reference

| Method | Purpose | When Called |
|---|---|---|
| `Declare` | Return agent name, supported task types, cost | On registration and capability refresh |
| `Invoke` | Execute a task, return result | When the task router assigns work |
| `Health` | Report current health status | Periodic health checks |
| `Checkpoint` | Serialize agent state for persistence | Before agent swap or at intervals |
| `Resume` | Restore agent state from checkpoint | After restart or failover |

## HTTP Adapter (Reference Implementation)

The HTTP adapter in `internal/adapter/http.go` maps the 5 methods to HTTP endpoints:

| Method | HTTP Call |
|---|---|
| `Declare` | `GET /declare` |
| `Invoke` | `POST /invoke` |
| `Health` | `GET /health` |
| `Checkpoint` | `GET /checkpoint` |
| `Resume` | `POST /resume` |

To expose your agent to Hive over HTTP, implement these 5 endpoints on your server. The HTTP adapter uses a 30-second timeout and limits response bodies to 10 MB.

### Minimal HTTP Agent (Python Example)

```python
from flask import Flask, jsonify, request

app = Flask(__name__)

@app.get("/declare")
def declare():
    return jsonify(name="my-agent", task_types=["summarize"], cost_per_run=0.01)

@app.post("/invoke")
def invoke():
    task = request.json
    result = do_work(task["input"])
    return jsonify(task_id=task["id"], status="completed", output=result)

@app.get("/health")
def health():
    return jsonify(status="healthy")

@app.get("/checkpoint")
def checkpoint():
    return jsonify(data={})

@app.post("/resume")
def resume():
    return "", 204
```

Register it: `hive add-agent --name my-agent --type http --url http://localhost:5000`

## CLI Adapter (Claude Code)

The `ClaudeCodeAdapter` in `internal/adapter/claude_code.go` invokes a Claude Code skill via subprocess. It passes task input as JSON on stdin and captures stdout as the result.

```go
cmd := exec.CommandContext(ctx, "claude", "--skill", a.SkillPath)
cmd.Stdin = strings.NewReader(string(inputJSON))
output, err := cmd.CombinedOutput()
```

Health is checked by verifying `claude` exists in PATH via `exec.LookPath("claude")`.

## MCP Adapter

The `MCPAdapter` in `internal/adapter/mcp.go` wraps an MCP server endpoint. It delegates to the HTTP adapter internally. If the MCP server does not implement `/declare`, it falls back to generic `mcp-tool` capabilities.

## Framework Adapters

| Adapter | File | Transport | Notes |
|---|---|---|---|
| HTTP | `http.go` | HTTP/JSON | Reference implementation |
| Claude Code | `claude_code.go` | stdio subprocess | Invokes `claude --skill` |
| MCP | `mcp.go` | HTTP (delegated) | Wraps MCP server |
| OpenAI | `openai.go` | OpenAI Assistants API | Creates thread, adds message, polls run |
| LangChain | `langchain.go` | HTTP (LangServe) | Delegates to HTTP adapter |
| CrewAI | `crewai.go` | Python subprocess | Invokes `python -m crewai run` |
| AutoGen | `autogen.go` | HTTP | Delegates to HTTP adapter |

## Writing Your Own Adapter

1. Create a new file in `internal/adapter/` (e.g., `myframework.go`)
2. Define a struct and implement all 5 `Adapter` methods
3. Add a compile-time interface check: `var _ Adapter = (*MyAdapter)(nil)`
4. Write tests in `myframework_test.go`

At minimum, `Declare`, `Invoke`, and `Health` must be functional. `Checkpoint` and `Resume` can return empty values if your agent is stateless.

## Task Routing

When a task needs execution, `internal/task/router.go` calls `FindCapableAgent()` which queries all healthy agents and matches their declared `TaskTypes` against the task's type. The first match wins.
