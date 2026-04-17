package adapter

// Adapter type identifiers. Used by the CLI (`hive add-agent --type ...`)
// and by downstream packages that need to switch on the kind of adapter.
// Kept as string constants so they can round-trip through YAML config and
// event payloads without a parse step.
const (
	TypeHTTP      = "http"
	TypeMCP       = "mcp"
	TypeOpenAI    = "openai"
	TypeLangChain = "langchain"
	TypeAutoGen   = "autogen"
	TypeCrewAI    = "crewai"
	TypeClaude    = "claude-code"
	TypeA2A       = "a2a"
)
