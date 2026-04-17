package adapter

import (
	"encoding/json"
	"fmt"
	"os"
)

// AgentSpec is the minimum an adapter factory needs to reconstruct an
// Adapter instance from a row in the agents table. It intentionally uses
// the raw Config JSON blob rather than a typed struct per adapter type
// because the stored shape varies: HTTP stores {"base_url"}, local
// subprocess adapters store {"path"}, OpenAI stores {"path": <assistantID>}
// plus the API key in Capabilities or the environment.
type AgentSpec struct {
	Name         string
	Type         string
	Config       string // raw JSON written at registration
	Capabilities string // raw JSON — some types need it at Invoke time (openai)
}

// BuildAdapter picks the right Adapter implementation for an agent based on
// its stored Type. Returns an error for unknown types so the caller can
// surface the problem instead of falling back to HTTP and hitting an empty
// URL (the v0 behaviour that made claude-code/crewai/autogen/langchain
// registrations look live but unusable).
func BuildAdapter(spec AgentSpec) (Adapter, error) {
	cfg := parseConfig(spec.Config)

	switch spec.Type {
	case TypeHTTP, "": // "" is the legacy default, treat as HTTP
		if cfg.BaseURL == "" {
			return nil, fmt.Errorf("http agent %q: base_url missing from stored config", spec.Name)
		}
		return NewHTTPAdapter(cfg.BaseURL), nil

	case TypeClaude:
		if cfg.Path == "" {
			return nil, fmt.Errorf("claude-code agent %q: path missing from stored config", spec.Name)
		}
		return NewClaudeCodeAdapter(cfg.Path, spec.Name), nil

	case TypeMCP:
		if cfg.Path != "" {
			return NewMCPAdapter(cfg.Path, spec.Name), nil
		}
		if cfg.BaseURL != "" {
			return NewMCPAdapter(cfg.BaseURL, spec.Name), nil
		}
		return nil, fmt.Errorf("mcp agent %q: path or base_url missing from stored config", spec.Name)

	case TypeCrewAI:
		if cfg.Path == "" {
			return nil, fmt.Errorf("crewai agent %q: path missing from stored config", spec.Name)
		}
		return NewCrewAIAdapter(cfg.Path, spec.Name), nil

	case TypeAutoGen:
		url := cfg.BaseURL
		if url == "" && cfg.Path != "" {
			url = "file://" + cfg.Path
		}
		if url == "" {
			return nil, fmt.Errorf("autogen agent %q: path or base_url missing from stored config", spec.Name)
		}
		return NewAutoGenAdapter(url, spec.Name), nil

	case TypeLangChain:
		url := cfg.BaseURL
		if url == "" && cfg.Path != "" {
			url = "file://" + cfg.Path
		}
		if url == "" {
			return nil, fmt.Errorf("langchain agent %q: path or base_url missing from stored config", spec.Name)
		}
		return NewLangChainAdapter(url, spec.Name), nil

	case TypeOpenAI:
		// OpenAI stores the assistant ID in Config.path and expects the API
		// key via env var. We resolve the key here so the engine stays
		// stateless; operators who need per-agent keys should set a bespoke
		// var or switch to vault integration once available.
		assistantID := cfg.Path
		if assistantID == "" {
			return nil, fmt.Errorf("openai agent %q: assistant id missing from stored config", spec.Name)
		}
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("openai agent %q: OPENAI_API_KEY env var not set", spec.Name)
		}
		return NewOpenAIAdapter(assistantID, apiKey, spec.Name), nil

	case TypeA2A:
		// A2A transport not wired yet; surface explicitly.
		return nil, fmt.Errorf("a2a adapter not implemented at runtime")

	default:
		return nil, fmt.Errorf("unknown agent type %q", spec.Type)
	}
}

// configSchema is the superset of fields written by any RegisterLocal /
// Register callsite. Unknown keys are ignored; missing keys come out zero.
type configSchema struct {
	BaseURL string `json:"base_url"`
	Path    string `json:"path"`
}

func parseConfig(raw string) configSchema {
	var c configSchema
	if raw == "" {
		return c
	}
	_ = json.Unmarshal([]byte(raw), &c)
	return c
}
