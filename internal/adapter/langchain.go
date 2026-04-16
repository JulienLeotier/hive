package adapter

import "context"

// LangChainAdapter connects to a LangChain agent exposed via LangServe HTTP API.
type LangChainAdapter struct {
	http *HTTPAdapter
	Name string
}

// NewLangChainAdapter creates an adapter for a LangServe endpoint.
func NewLangChainAdapter(baseURL, name string) *LangChainAdapter {
	return &LangChainAdapter{http: NewHTTPAdapter(baseURL), Name: name}
}

func (a *LangChainAdapter) Declare(ctx context.Context) (AgentCapabilities, error) {
	caps, err := a.http.Declare(ctx)
	if err != nil {
		return AgentCapabilities{Name: a.Name, TaskTypes: []string{"langchain-chain"}}, nil
	}
	caps.Name = a.Name
	return caps, nil
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
