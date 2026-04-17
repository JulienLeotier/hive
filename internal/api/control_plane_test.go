package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JulienLeotier/hive/internal/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFireWorkflow503WithoutTriggerManager(t *testing.T) {
	srv := setupServer(t)

	req := httptest.NewRequest("POST", "/api/v1/workflows/deploy/runs", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestFireWorkflowDispatchesRegistered(t *testing.T) {
	srv := setupServer(t)

	var fires int32
	tm := workflow.NewTriggerManager(func(ctx context.Context, cfg *workflow.Config, p workflow.TriggerPayload) error {
		atomic.AddInt32(&fires, 1)
		return nil
	})
	require.NoError(t, tm.Register(context.Background(), &workflow.Config{
		Name:  "deploy",
		Tasks: []workflow.TaskDef{{Name: "t", Type: "x"}},
	}))
	srv.WithTriggerManager(tm)

	req := httptest.NewRequest("POST", "/api/v1/workflows/deploy/runs", strings.NewReader(`{"ref":"main"}`))
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	// FireManual runs in a goroutine — poll briefly for the fire count.
	assert.Eventually(t, func() bool {
		return atomic.LoadInt32(&fires) == 1
	}, 2*time.Second, 20*time.Millisecond, "workflow should fire once")
}

func TestRetryTaskCreatesNewPendingRow(t *testing.T) {
	srv := setupServer(t)

	// Seed a failed task
	_, err := srv.db().Exec(
		`INSERT INTO tasks (id, workflow_id, type, input, status, agent_id) VALUES (?, ?, ?, ?, 'failed', 'agent-x')`,
		"tsk-1", "wf-1", "code-review", `{"foo":1}`,
	)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/v1/tasks/tsk-1/retry", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp Response
	json.NewDecoder(w.Body).Decode(&resp)
	require.Nil(t, resp.Error)

	data := resp.Data.(map[string]any)
	newID, _ := data["new_task_id"].(string)
	assert.NotEmpty(t, newID)

	// Verify new row is pending and shares the original type/input/workflow
	var typ, input, workflowID, status string
	require.NoError(t, srv.db().QueryRow(
		`SELECT type, input, workflow_id, status FROM tasks WHERE id = ?`, newID,
	).Scan(&typ, &input, &workflowID, &status))
	assert.Equal(t, "code-review", typ)
	assert.Equal(t, `{"foo":1}`, input)
	assert.Equal(t, "wf-1", workflowID)
	assert.Equal(t, "pending", status)
}

func TestRetryTaskRejectsInFlight(t *testing.T) {
	srv := setupServer(t)

	_, err := srv.db().Exec(
		`INSERT INTO tasks (id, workflow_id, type, input, status, agent_id) VALUES (?, ?, ?, ?, 'running', 'agent-x')`,
		"tsk-2", "wf-1", "x", `{}`,
	)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/v1/tasks/tsk-2/retry", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestRetryTask404WhenMissing(t *testing.T) {
	srv := setupServer(t)
	req := httptest.NewRequest("POST", "/api/v1/tasks/ghost/retry", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
