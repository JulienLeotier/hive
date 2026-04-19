package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/project"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleSearch_ShortQueryReturnsEmpty(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()
	srv := NewServer(event.NewBus(st.DB))

	for _, q := range []string{"", "a"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q="+q, nil)
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		var env struct {
			Data []any `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
		assert.Empty(t, env.Data, "q=%q doit renvoyer []", q)
	}
}

func TestHandleSearch_MatchesProjectNameAndIdea(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()
	srv := NewServer(event.NewBus(st.DB)).
		WithProjectStore(project.NewStore(st.DB))
	store := project.NewStore(st.DB)

	ctx := context.Background()
	_, err = store.Create(ctx, "default", "mon idée sur la blockchain", project.CreateOpts{Name: "blockchain-app"})
	require.NoError(t, err)
	_, err = store.Create(ctx, "default", "todo list classique", project.CreateOpts{Name: "todolist"})
	require.NoError(t, err)

	// Search "block" → match blockchain-app (name ET idea).
	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=block", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var env struct {
		Data []struct {
			Type  string `json:"type"`
			Title string `json:"title"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	assert.NotEmpty(t, env.Data)
	for _, h := range env.Data {
		assert.Equal(t, "project", h.Type)
		assert.Contains(t, h.Title, "blockchain")
	}
}

func TestHandleSearch_CaseInsensitive(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()
	srv := NewServer(event.NewBus(st.DB)).
		WithProjectStore(project.NewStore(st.DB))
	store := project.NewStore(st.DB)
	_, _ = store.Create(context.Background(), "default", "CAPS IDEA", project.CreateOpts{Name: "UPPER-NAME"})

	for _, q := range []string{"caps", "CAPS", "CaPs"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q="+q, nil)
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		var env struct {
			Data []any `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
		assert.NotEmpty(t, env.Data, "q=%q doit trouver le projet", q)
	}
}
