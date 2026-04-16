package hivehub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newIndexServer(t *testing.T, templates []Template, files map[string]string) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/index.json", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(templates)
	})
	for path, body := range files {
		p, b := path, body
		mux.HandleFunc(p, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(b))
		})
	}
	return httptest.NewServer(mux)
}

func TestSearchFindsByKeyword(t *testing.T) {
	srv := newIndexServer(t, []Template{
		{Name: "code-review", Description: "PR quality gate", Category: "review"},
		{Name: "research", Description: "Market research pipeline", Category: "research"},
	}, nil)
	defer srv.Close()

	r := NewRegistry()
	r.IndexURL = srv.URL + "/index.json"
	results, err := r.Search("review")
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "code-review", results[0].Name)
}

func TestInstallWritesFilesFromManifest(t *testing.T) {
	dest := t.TempDir()
	manifest := []map[string]string{
		{"path": "hive.yaml", "content": "name: demo\n"},
		{"path": "adapters/agent.yaml", "content": "name: worker\n"},
	}
	manifestJSON, _ := json.Marshal(manifest)

	srv := newIndexServer(t, []Template{
		{Name: "demo", Version: "0.1.0", URL: "%MANIFEST%"},
	}, map[string]string{"/demo.json": string(manifestJSON)})
	defer srv.Close()

	// Rewrite template URL to the running server.
	r := NewRegistry()
	r.IndexURL = srv.URL + "/index.json"
	tmpl, err := r.Get("demo")
	require.NoError(t, err)
	tmpl.URL = srv.URL + "/demo.json"
	// Patch registry by hand: we install via a fresh search+get flow, so
	// substitute via a short custom path: write-through the URL.
	idx := []Template{*tmpl}
	srv2 := newIndexServer(t, idx, map[string]string{"/demo.json": string(manifestJSON)})
	defer srv2.Close()
	r.IndexURL = srv2.URL + "/index.json"
	tmplFixed, err := r.Get("demo")
	require.NoError(t, err)
	tmplFixed.URL = srv2.URL + "/demo.json"
	// Replace the index response so Install's internal Get sees the patched URL.
	r.IndexURL = func() string {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode([]Template{*tmplFixed})
		}))
		t.Cleanup(func() { s.Close() })
		return s.URL
	}()

	got, files, err := r.Install("demo", dest)
	require.NoError(t, err)
	assert.Equal(t, "demo", got.Name)
	assert.ElementsMatch(t, []string{"hive.yaml", "adapters/agent.yaml"}, files)

	data, err := os.ReadFile(filepath.Join(dest, "hive.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "name: demo\n", string(data))
}

func TestPublishDirProducesManifest(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hive.yaml"), []byte("name: demo\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "agents"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "agents", "worker.yaml"), []byte("role: worker\n"), 0o644))
	// Junk that must be skipped
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("bad"), 0o644))

	r := NewRegistry()
	data, err := r.PublishDir(dir, Template{Name: "demo", Version: "0.1.0"})
	require.NoError(t, err)

	var payload struct {
		Template Template            `json:"template"`
		Files    []map[string]string `json:"files"`
	}
	require.NoError(t, json.Unmarshal(data, &payload))
	assert.Equal(t, "demo", payload.Template.Name)
	// Expect the two real files, not the .git junk.
	paths := []string{}
	for _, f := range payload.Files {
		paths = append(paths, f["path"])
	}
	assert.ElementsMatch(t, []string{"hive.yaml", "agents/worker.yaml"}, paths)
}
