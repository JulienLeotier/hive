package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JulienLeotier/hive/internal/auth"
	eventPkg "github.com/JulienLeotier/hive/internal/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAdapterEventPush exercises Story 2.1 AC:
// "agents can emit custom events via the adapter protocol".
func TestAdapterEventPush(t *testing.T) {
	srv := setupServer(t)

	// Key the adapter authenticates with, mapped to an operator role so the
	// write-permissioned endpoint lets it through.
	keyMgr := srv.keyMgr
	users := auth.NewUserStore(srv.eventBus.DB())
	operator, err := keyMgr.Generate(context.Background(), "operator-adapter")
	require.NoError(t, err)
	require.NoError(t, users.Upsert(context.Background(), auth.UserRecord{
		Subject: "operator-adapter", Role: auth.RoleOperator,
	}))
	srv.WithUsers(users)

	body, _ := json.Marshal(map[string]any{
		"type":    "custom.signal",
		"payload": map[string]string{"hello": "adapter"},
	})
	req := httptest.NewRequest("POST", "/api/v1/events", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+operator)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "operator must be able to push custom events")

	// And the event must be queryable afterwards.
	evts, err := srv.eventBus.Query(context.Background(), eventPkg.QueryOpts{Type: "custom.signal"})
	require.NoError(t, err)
	assert.Len(t, evts, 1)
}

