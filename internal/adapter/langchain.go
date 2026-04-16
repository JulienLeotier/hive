package adapter

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// LangChainAdapter connects to a LangChain agent exposed via LangServe HTTP API.
type LangChainAdapter struct {
	http *HTTPAdapter
	Name string
}

// NewLangChainAdapter creates an adapter for a LangServe endpoint.
func NewLangChainAdapter(baseURL, name string) *LangChainAdapter {
	return &LangChainAdapter{http: NewHTTPAdapter(baseURL), Name: name}
}

// Declare maps LangServe routes to Hive capabilities. Story 13.2 AC:
// "connects to the LangServe endpoint and maps available chains to Hive
// capabilities". Reads the OpenAPI spec and lists each /<chain>/invoke route.
func (a *LangChainAdapter) Declare(ctx context.Context) (AgentCapabilities, error) {
	if caps, err := a.http.Declare(ctx); err == nil {
		caps.Name = a.Name
		return caps, nil
	}

	// LangServe exposes /openapi.json. Parse it to discover routes.
	req, err := http.NewRequestWithContext(ctx, "GET", a.http.BaseURL+"/openapi.json", nil)
	if err == nil {
		if resp, err := a.http.HTTPClient.Do(req); err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			var spec struct {
				Paths map[string]any `json:"paths"`
			}
			if json.NewDecoder(resp.Body).Decode(&spec) == nil {
				var routes []string
				for p := range spec.Paths {
					if strings.HasSuffix(p, "/invoke") {
						name := strings.TrimSuffix(strings.TrimPrefix(p, "/"), "/invoke")
						if name == "" {
							name = "langchain-chain"
						}
						routes = append(routes, "langchain-"+name)
					}
				}
				if len(routes) > 0 {
					return AgentCapabilities{Name: a.Name, TaskTypes: routes}, nil
				}
			}
		}
	}

	return AgentCapabilities{Name: a.Name, TaskTypes: []string{"langchain-chain"}}, nil
}

func (a *LangChainAdapter) Invoke(ctx context.Context, task Task) (TaskResult, error) {
	return a.http.Invoke(ctx, task)
}

func (a *LangChainAdapter) Health(ctx context.Context) (HealthStatus, error) {
	return a.http.Health(ctx)
}

func (a *LangChainAdapter) Checkpoint(ctx context.Context) (Checkpoint, error) {
	return a.http.Checkpoint(ctx)
}

func (a *LangChainAdapter) Resume(ctx context.Context, cp Checkpoint) error {
	return a.http.Resume(ctx, cp)
}

var _ Adapter = (*LangChainAdapter)(nil)
