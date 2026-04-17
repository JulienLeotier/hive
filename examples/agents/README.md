# Starter agent templates

Three drop-in HTTP agents that implement the Hive adapter protocol
(`GET /health`, `GET /declare`, `POST /invoke`). Each is a tiny Go
server with its own `go.mod`; build and run with `go run .` or the
provided Dockerfile.

| Directory | Purpose | External deps |
|---|---|---|
| [`echo/`](./echo/) | Returns whatever input you give it, tagged. Useful for smoke-testing Hive without any API keys. | none |
| [`summarizer/`](./summarizer/) | Summarises text via OpenAI's chat completions. | `OPENAI_API_KEY` env var |
| [`translator/`](./translator/) | Translates text between languages via OpenAI. | `OPENAI_API_KEY` env var |

## Register one

```bash
# In another terminal, the agent's HTTP server listens on :9100
cd examples/agents/echo && go run .

# Back in the hive shell:
hive add-agent --name echo --type http --url http://localhost:9100

# Fire the demo workflow:
hive run examples/agents/echo/workflow.yaml
```

Every template ships with a minimal `workflow.yaml` demonstrating one
canonical usage so you can go from zero to "workflow completed" in
under a minute.
