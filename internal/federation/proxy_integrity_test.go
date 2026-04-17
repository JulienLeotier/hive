package federation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProxy_RejectsTaskIDMismatch simulates a peer that returns a response
// for a different task_id — whether via bug or replay attack. Without
// rejection, the caller would mis-attribute the output to its own task.
func TestProxy_RejectsTaskIDMismatch(t *testing.T) {
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a response for a completely different task_id.
		_ = json.NewEncoder(w).Encode(ProxyResponse{
			TaskID: "t-wrong",
			Status: "completed",
			Output: map[string]string{"forged": "data"},
		})
	}))
	defer peer.Close()

	st := setupStore(t)
	ctx := context.Background()
	require.NoError(t, st.Add(ctx, &Link{Name: "peer-x", URL: peer.URL, Status: "active"}, "", "", ""))

	p := NewProxy(st)
	_, err := p.Invoke(ctx, "peer-x", ProxyRequest{TaskID: "t-ours"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "task_id mismatch",
		"mismatched task_id must be rejected so forged/replayed responses don't land in the task log")
}
