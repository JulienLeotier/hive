package adapter

import "context"

// AutoGenAdapter connects to a Microsoft AutoGen agent exposed via HTTP.
type AutoGenAdapter struct {
	http *HTTPAdapter
	Name string
}

// NewAutoGenAdapter creates an adapter for an AutoGen HTTP endpoint.
func NewAutoGenAdapter(baseURL, name string) *AutoGenAdapter {
	return &AutoGenAdapter{http: NewHTTPAdapter(baseURL), Name: name}
}

func (a *AutoGenAdapter) Declare(ctx context.Context) (AgentCapabilities, error) {
	caps, err := a.http.Declare(ctx)
	if err != nil {
		return AgentCapabilities{Name: a.Name, TaskTypes: []string{"autogen-agent"}}, nil
	}
	caps.Name = a.Name
	return caps, nil
}

func (a *AutoGenAdapter) Invoke(ctx context.Context, task Task) (TaskResult, error) {
	return a.http.Invoke(ctx, task)
}

func (a *AutoGenAdapter) Health(ctx context.Context) (HealthStatus, error) {
	return a.http.Health(ctx)
}

func (a *AutoGenAdapter) Checkpoint(ctx context.Context) (Checkpoint, error) {
	return a.http.Checkpoint(ctx)
}

func (a *AutoGenAdapter) Resume(ctx context.Context, cp Checkpoint) error {
	return a.http.Resume(ctx, cp)
}

var _ Adapter = (*AutoGenAdapter)(nil)
