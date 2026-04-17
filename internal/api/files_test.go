package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/JulienLeotier/hive/internal/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedProjectWithWorkdir creates a project bound to a temp directory
// populated with a handful of files (including a binary one and a
// skipped .git subdir) so the list + content endpoints can be
// exercised under realistic conditions.
func seedProjectWithWorkdir(t *testing.T, srv *Server) (string, string) {
	t.Helper()
	workdir := t.TempDir()

	// Plain text file.
	require.NoError(t, os.WriteFile(filepath.Join(workdir, "README.md"),
		[]byte("# hello\n\nsome text\n"), 0o644))

	// Nested source file.
	require.NoError(t, os.MkdirAll(filepath.Join(workdir, "src"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(workdir, "src", "main.go"),
		[]byte("package main\n"), 0o644))

	// Binary file — must be reported with is_binary=true.
	require.NoError(t, os.WriteFile(filepath.Join(workdir, "blob.bin"),
		[]byte{0x00, 0x01, 0xff, 0xfe}, 0o644))

	// .git dir that must be skipped.
	require.NoError(t, os.MkdirAll(filepath.Join(workdir, ".git", "objects"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(workdir, ".git", "HEAD"),
		[]byte("ref: refs/heads/main\n"), 0o644))

	store := project.NewStore(srv.db())
	srv.WithProjectStore(store)
	p, err := store.Create(context.Background(), "default", "idea",
		project.CreateOpts{Name: "files-demo", Workdir: workdir})
	require.NoError(t, err)
	return p.ID, workdir
}

func TestListFilesWalksWorkdir(t *testing.T) {
	srv := setupServer(t)
	pid, _ := seedProjectWithWorkdir(t, srv)

	req := httptest.NewRequest("GET", "/api/v1/projects/"+pid+"/files", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())

	var resp struct {
		Data struct {
			Files []fileEntry `json:"files"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))

	paths := make([]string, 0, len(resp.Data.Files))
	for _, e := range resp.Data.Files {
		paths = append(paths, e.Path)
	}
	assert.Contains(t, paths, "README.md")
	assert.Contains(t, paths, "src/main.go")
	assert.Contains(t, paths, "blob.bin")
	for _, p := range paths {
		assert.NotContains(t, p, ".git/", ".git dir must be skipped")
	}
}

func TestFileContentReturnsText(t *testing.T) {
	srv := setupServer(t)
	pid, _ := seedProjectWithWorkdir(t, srv)

	u := "/api/v1/projects/" + pid + "/files/content?path=" + url.QueryEscape("src/main.go")
	req := httptest.NewRequest("GET", u, nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var resp struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))

	assert.Equal(t, false, resp.Data["is_binary"])
	assert.Equal(t, "package main\n", resp.Data["content"])
}

func TestFileContentFlagsBinary(t *testing.T) {
	srv := setupServer(t)
	pid, _ := seedProjectWithWorkdir(t, srv)

	u := "/api/v1/projects/" + pid + "/files/content?path=blob.bin"
	req := httptest.NewRequest("GET", u, nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var resp struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))

	assert.Equal(t, true, resp.Data["is_binary"])
	_, hasContent := resp.Data["content"]
	assert.False(t, hasContent, "binary files must not ship content")
}

func TestFileContentRefusesPathTraversal(t *testing.T) {
	srv := setupServer(t)
	pid, _ := seedProjectWithWorkdir(t, srv)

	// `..` attempt.
	u := "/api/v1/projects/" + pid + "/files/content?path=" + url.QueryEscape("../etc/passwd")
	req := httptest.NewRequest("GET", u, nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code, "path traversal must be rejected")

	// Absolute path attempt.
	u = "/api/v1/projects/" + pid + "/files/content?path=" + url.QueryEscape("/etc/passwd")
	req = httptest.NewRequest("GET", u, nil)
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code, "absolute paths must be rejected")
}
