package adapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockAgent(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /declare", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(AgentCapabilities{
			Name:      "mock-agent",
			TaskTypes: []string{"code-review", "summarize"},
		})
	})

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(HealthStatus{Status: "healthy"})
	})

	mux.HandleFunc("POST /invoke", func(w http.ResponseWriter, r *http.Request) {
		var task Task
		json.NewDecoder(r.Body).Decode(&task)
		json.NewEncoder(w).Encode(TaskResult{
			TaskID: task.ID,
			Status: "completed",
			Output: map[string]string{"result": "done"},
		})
	})

	mux.HandleFunc("GET /checkpoint", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(Checkpoint{Data: map[string]int{"step": 3}})
	})

	mux.HandleFunc("POST /resume", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return httptest.NewServer(mux)
}

func TestHTTPAdapterDeclare(t *testing.T) {
	srv := newMockAgent(t)
	defer srv.Close()

	a := NewHTTPAdapter(srv.URL)
	caps, err := a.Declare(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "mock-agent", caps.Name)
	assert.Equal(t, []string{"code-review", "summarize"}, caps.TaskTypes)
}

func TestHTTPAdapterHealth(t *testing.T) {
	srv := newMockAgent(t)
	defer srv.Close()

	a := NewHTTPAdapter(srv.URL)
	status, err := a.Health(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "healthy", status.Status)
}

func TestHTTPAdapterHealthUnreachable(t *testing.T) {
	a := NewHTTPAdapter("http://localhost:1") // unreachable
	status, err := a.Health(context.Background())
	require.NoError(t, err) // Health should not error, returns unavailable
	assert.Equal(t, "unavailable", status.Status)
}

func TestHTTPAdapterInvoke(t *testing.T) {
	srv := newMockAgent(t)
	defer srv.Close()

	a := NewHTTPAdapter(srv.URL)
	result, err := a.Invoke(context.Background(), Task{
		ID:    "task-001",
		Type:  "code-review",
		Input: map[string]string{"file": "main.go"},
	})
	require.NoError(t, err)
	assert.Equal(t, "task-001", result.TaskID)
	assert.Equal(t, "completed", result.Status)
}

func TestHTTPAdapterCheckpoint(t *testing.T) {
	srv := newMockAgent(t)
	defer srv.Close()

	a := NewHTTPAdapter(srv.URL)
	cp, err := a.Checkpoint(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, cp.Data)
}

func TestHTTPAdapterResume(t *testing.T) {
	srv := newMockAgent(t)
	defer srv.Close()

	a := NewHTTPAdapter(srv.URL)
	err := a.Resume(context.Background(), Checkpoint{Data: map[string]int{"step": 3}})
	require.NoError(t, err)
}

func TestHTTPAdapterHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer srv.Close()

	a := NewHTTPAdapter(srv.URL)
	_, err := a.Declare(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 500")
}

// Verify HTTPAdapter implements Adapter interface at compile time.
var _ Adapter = (*HTTPAdapter)(nil)
