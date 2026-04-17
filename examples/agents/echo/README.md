# echo agent

Zero-dependency HTTP agent that echoes every task input back as output.
Handy for:

- Verifying Hive is routing tasks correctly before plugging in a real LLM
- Walking through the visual workflow builder with live task completions
- CI smoke tests

## Run

```bash
go run .                # listens on :9100
PORT=8099 go run .      # custom port
```

## Register

```bash
hive add-agent --name echo --type http --url http://localhost:9100
```

## Fire a demo workflow

```bash
hive run examples/agents/echo/workflow.yaml
```

The two-step workflow (`ping` → `ping-again`) verifies DAG execution.
Open `/workflows/<id>` in the dashboard to see both tasks completed
with the inputs reflected back.

## Capabilities

- Task types: `echo`, `debug`
- Cost per run: 0.0 (free — it's an echo)
- Version: 1.0.0
