# Adapters Guide (CrewAI · LangChain · AutoGen · OpenAI Assistants)

Hive ships adapters for the four most-asked-for frameworks. Each maps the
framework's native invocation to the Hive Agent Adapter Protocol
(`Declare / Invoke / Health / Checkpoint / Resume`), so you can mix them in
the same workflow.

## CrewAI

```bash
hive add-agent --type crewai --path ./my-crew --name reviewer
```

- Uses local subprocess execution (`python -m crewai run`).
- Declare returns `task_types: ["crewai-crew"]` unless the crew's `AGENT.yaml`
  overrides the capabilities list.
- Input is piped as JSON on stdin; output is collected from stdout.

## LangChain / LangServe

```bash
hive add-agent --type langchain --url http://localhost:8000 --name rag
```

- Expects a LangServe endpoint at the given base URL.
- Declare delegates to the LangServe OpenAPI spec; if unreachable, falls back
  to `task_types: ["langchain-chain"]`.
- Invoke forwards the task as a JSON POST body compatible with LangServe's
  invocation schema.

## AutoGen

```bash
hive add-agent --type autogen --url http://localhost:8001 --name chat
```

- AutoGen agents are expected to be exposed via an HTTP wrapper.
- Same capability-detection behaviour as LangChain.
- Invoke sends the task JSON; expects a `{task_id, status, output}` JSON reply.

## OpenAI Assistants

```bash
hive add-agent --type openai \
  --name assistant-1 \
  --assistant-id asst_abc123 \
  --api-key $OPENAI_API_KEY
```

- Creates a thread per task, runs the assistant, polls for completion, returns
  the last assistant message as output.
- `--api-key` is optional — falls back to `$OPENAI_API_KEY`.
- Costs can be tracked via the `cost_per_run` capability declaration.

## Writing your own

```bash
hive adapter-template my-framework
cd my-framework
go run .
```

- Scaffolds `main.go` (HTTP listener with `/declare`, `/task`, `/health`,
  `/checkpoint`), `AGENT.yaml`, `README.md`, and a compliance-test stub.
- The generated compliance test is wired for
  `adapter.RunCompliance(&MyAdapter{}, opts)` once you vendor Hive into the
  project.

## Compliance

```go
res := adapter.RunCompliance(myAdapter, adapter.ComplianceOptions{})
if !res.OK() {
    // res.Failed lists the specific checks that didn't pass.
}
```

The harness exercises every protocol method (plus optional checkpoint
round-trip) with configurable timeouts and skips.

## Retry policy

HTTP-backed adapters can be wrapped with an exponential-backoff retry:

```go
adapter.NewHTTPAdapter(url).WithRetry(&adapter.RetryPolicy{
    MaxAttempts: 3,
    InitialWait: 200 * time.Millisecond,
    MaxWait:     2 * time.Second,
    Multiplier:  2.0,
    Jitter:      0.2,
})
```

The workflow engine installs a policy from `hive.yaml`'s `retry:` block and
wires an `OnAttempt` hook that publishes `task.retry` events for every
attempt.

## Trust levels

Every adapter plays the same game — start at `supervised`, earn promotions
after clean task histories. See [trust-configuration.md](trust-configuration.md).
