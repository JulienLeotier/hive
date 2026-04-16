# Knowledge Layer

The knowledge layer persists "what worked" and "what didn't" across agent runs
so the colony accumulates operational wisdom. Entries decay after 90 days by
default so stale patterns don't override recent learnings.

## What gets stored

Every completed task can record a knowledge entry via
`knowledge.Store.Record(ctx, taskType, approach, outcome, contextJSON)`.

Fields:

| Field       | Meaning                                             |
|-------------|-----------------------------------------------------|
| `task_type` | capability label (e.g., `code-review`)              |
| `approach`  | human-readable description of what was tried        |
| `outcome`   | `success` or `failure`                              |
| `context`   | arbitrary JSON (lang, repo size, …)                 |
| `embedding` | float32 vector BLOB (populated when Embedder is set)|
| `created_at`| UTC timestamp                                       |

## Embedders

- **HashingEmbedder** (default, zero-dependency): deterministic, L2-normalised,
  signed-feature hashing. Good for in-repo tests and air-gapped installs.
- **OpenAIEmbedder**: hits `/v1/embeddings` with a configurable model
  (default `text-embedding-3-small`, 1536 dimensions). Falls back to the
  provided `Fallback` embedder on any error (missing key, rate limit, 5xx,
  timeout) so knowledge features never break because of a transient network
  blip.

```go
store := knowledge.NewStore(db).WithEmbedder(
    knowledge.NewOpenAIEmbedder(os.Getenv("OPENAI_API_KEY"),
        "text-embedding-3-small",
        knowledge.NewHashingEmbedder(1536)),
)
```

## CLI

```bash
hive knowledge list --type code-review
hive knowledge search "how to handle API timeouts"
```

`search` uses vector similarity when an embedder is configured, otherwise
falls back to keyword + recency ranking.

## Lifecycle

- Entries older than `maxAge` (default 90 days) are excluded from search.
- Results are tie-broken by recency: at equal similarity, newer wins.
- No write amplification: records without embeddings are still stored so the
  keyword search keeps working even before you configure an embedder.
