package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockHive stands in for the real server. Each test registers a route
// handler keyed on METHOD + PATH. Keeps tests readable — one struct per
// test, response shapes match the real envelope.
type mockHive struct {
	t        *testing.T
	routes   map[string]http.HandlerFunc
	requests []*http.Request
}

func newMock(t *testing.T) (*mockHive, *httptest.Server) {
	m := &mockHive{t: t, routes: map[string]http.HandlerFunc{}}
	srv := httptest.NewServer(m)
	t.Cleanup(srv.Close)
	return m, srv
}

func (m *mockHive) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.requests = append(m.requests, r)
	key := r.Method + " " + r.URL.Path
	h, ok := m.routes[key]
	if !ok {
		http.Error(w, "no route for "+key, http.StatusNotFound)
		return
	}
	h(w, r)
}

func (m *mockHive) on(method, path string, h http.HandlerFunc) {
	m.routes[method+" "+path] = h
}

// writeData is a tiny helper that produces the "{data:..., error:null}"
// envelope the server uses.
func writeData(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": v, "error": nil})
}

// writeErr returns the server's error envelope shape.
func writeErr(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data":  nil,
		"error": map[string]string{"code": code, "message": msg},
	})
}

func TestAgentsListHappyPath(t *testing.T) {
	mock, srv := newMock(t)
	mock.on("GET", "/api/v1/agents", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer hive_test", r.Header.Get("Authorization"))
		writeData(w, []Agent{
			{ID: "a1", Name: "reviewer", Type: "http", HealthStatus: "healthy"},
		})
	})

	c := NewClient(srv.URL, "hive_test")
	got, err := c.Agents().List(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "reviewer", got[0].Name)
	assert.Equal(t, "healthy", got[0].HealthStatus)
}

func TestAgentsRegister(t *testing.T) {
	mock, srv := newMock(t)
	mock.on("POST", "/api/v1/agents", func(w http.ResponseWriter, r *http.Request) {
		var body RegisterOpts
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "worker", body.Name)
		assert.Equal(t, "http", body.Type)
		w.WriteHeader(http.StatusCreated)
		writeDataNoHeader(w, Agent{ID: "a2", Name: body.Name, Type: body.Type, HealthStatus: "healthy"})
	})

	c := NewClient(srv.URL, "k")
	a, err := c.Agents().Register(context.Background(), RegisterOpts{
		Name: "worker", Type: "http", URL: "http://worker.example.com",
	})
	require.NoError(t, err)
	assert.Equal(t, "a2", a.ID)
}

// writeDataNoHeader is a variant that doesn't send Content-Type / 200; use
// after WriteHeader has already been called with a custom status.
func writeDataNoHeader(w http.ResponseWriter, v any) {
	_ = json.NewEncoder(w).Encode(map[string]any{"data": v, "error": nil})
}

func TestAgentsDelete(t *testing.T) {
	mock, srv := newMock(t)
	mock.on("DELETE", "/api/v1/agents/worker", func(w http.ResponseWriter, r *http.Request) {
		writeData(w, map[string]string{"status": "removed", "name": "worker"})
	})

	c := NewClient(srv.URL, "k")
	err := c.Agents().Delete(context.Background(), "worker")
	require.NoError(t, err)
}

func TestAgentsInvokeForwardsTypeAndInput(t *testing.T) {
	mock, srv := newMock(t)
	mock.on("POST", "/api/v1/agents/worker/invoke", func(w http.ResponseWriter, r *http.Request) {
		var got InvokeOpts
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		assert.Equal(t, "code-review", got.Type)
		writeData(w, TaskResult{TaskID: "t-x", Status: "completed", Output: map[string]any{"score": 0.9}})
	})

	c := NewClient(srv.URL, "k")
	res, err := c.Agents().Invoke(context.Background(), "worker", InvokeOpts{
		Type: "code-review", Input: map[string]string{"diff": "..."},
	})
	require.NoError(t, err)
	assert.Equal(t, "t-x", res.TaskID)
}

func TestTasksListWithOpts(t *testing.T) {
	mock, srv := newMock(t)
	mock.on("GET", "/api/v1/tasks", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		assert.Equal(t, "failed", q.Get("status"))
		assert.Equal(t, "wf-1", q.Get("workflow_id"))
		assert.Equal(t, "50", q.Get("limit"))
		writeData(w, []Task{{ID: "t1", Type: "x", Status: "failed"}})
	})

	c := NewClient(srv.URL, "k")
	got, err := c.Tasks().List(context.Background(), ListOpts{
		Status: "failed", WorkflowID: "wf-1", Limit: 50,
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
}

func TestTasksRetry(t *testing.T) {
	mock, srv := newMock(t)
	mock.on("POST", "/api/v1/tasks/t1/retry", func(w http.ResponseWriter, r *http.Request) {
		writeData(w, RetryResult{NewTaskID: "retry_t1", OriginalTaskID: "t1"})
	})

	c := NewClient(srv.URL, "k")
	res, err := c.Tasks().Retry(context.Background(), "t1")
	require.NoError(t, err)
	assert.Equal(t, "retry_t1", res.NewTaskID)
}

func TestWorkflowsFireWithPayload(t *testing.T) {
	mock, srv := newMock(t)
	mock.on("POST", "/api/v1/workflows/deploy/runs", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		assert.Contains(t, string(body), `"ref":"main"`)
		w.WriteHeader(http.StatusAccepted)
		writeDataNoHeader(w, FireResult{Status: "accepted", Workflow: "deploy"})
	})

	c := NewClient(srv.URL, "k")
	res, err := c.Workflows().Fire(context.Background(), "deploy", map[string]string{"ref": "main"})
	require.NoError(t, err)
	assert.Equal(t, "accepted", res.Status)
}

func TestWebhooksAdd(t *testing.T) {
	mock, srv := newMock(t)
	mock.on("POST", "/api/v1/webhooks", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		writeDataNoHeader(w, Webhook{ID: "wh1", Name: "slack", URL: "https://hooks.example/x", Type: "slack", Enabled: true})
	})
	c := NewClient(srv.URL, "k")
	wh, err := c.Webhooks().Add(context.Background(), AddOpts{
		Name: "slack", URL: "https://hooks.example/x", Type: "slack",
	})
	require.NoError(t, err)
	assert.Equal(t, "wh1", wh.ID)
}

func TestEventsEmit(t *testing.T) {
	mock, srv := newMock(t)
	mock.on("POST", "/api/v1/events", func(w http.ResponseWriter, r *http.Request) {
		var body EmitOpts
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "custom.ping", body.Type)
		writeData(w, map[string]any{"id": 1})
	})
	c := NewClient(srv.URL, "k")
	err := c.Events().Emit(context.Background(), EmitOpts{Type: "custom.ping"})
	require.NoError(t, err)
}

func TestAPIErrorIsTyped(t *testing.T) {
	mock, srv := newMock(t)
	mock.on("GET", "/api/v1/agents", func(w http.ResponseWriter, r *http.Request) {
		writeErr(w, http.StatusNotFound, "NOT_FOUND", "no such thing")
	})
	c := NewClient(srv.URL, "k")
	_, err := c.Agents().List(context.Background())
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr), "error must unwrap to *APIError")
	assert.Equal(t, "NOT_FOUND", apiErr.Code)
	assert.Equal(t, http.StatusNotFound, apiErr.HTTPStatus)
}

func TestNonJSONErrorSurfacesAsAPIError(t *testing.T) {
	// Simulate an ingress returning HTML 502 — the SDK should still produce
	// an APIError so callers can branch on type instead of parsing.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("<html>504 cloudflare</html>"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "k")
	_, err := c.Agents().List(context.Background())
	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusBadGateway, apiErr.HTTPStatus)
	assert.True(t, strings.Contains(apiErr.Message, "cloudflare"))
}

func TestNoAuthHeaderWhenAPIKeyIsEmpty(t *testing.T) {
	mock, srv := newMock(t)
	mock.on("GET", "/api/v1/agents", func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"), "dev mode: no Authorization header")
		writeData(w, []Agent{})
	})
	c := NewClient(srv.URL, "")
	_, err := c.Agents().List(context.Background())
	require.NoError(t, err)
}
