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

// TestInstallDoesNotOverwriteByDefault guards Story 14.3 AC:
// "existing files are not overwritten without confirmation".
func TestInstallDoesNotOverwriteByDefault(t *testing.T) {
	dest := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dest, "hive.yaml"), []byte("old content\n"), 0o644))

	manifest := []map[string]string{{"path": "hive.yaml", "content": "new content\n"}}
	manifestJSON, _ := json.Marshal(manifest)

	// Build a registry that returns our templated tarball.
	mux := http.NewServeMux()
	mux.HandleFunc("/template.json", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(manifestJSON) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	idxSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]Template{{Name: "demo", Version: "0.1.0", URL: srv.URL + "/template.json"}})
	}))
	defer idxSrv.Close()

	r := NewRegistry()
	r.IndexURL = idxSrv.URL

	// Default Install (Confirm=nil) must NOT overwrite.
	_, written, err := r.Install("demo", dest)
	require.NoError(t, err)
	assert.Empty(t, written, "no files should be written when an existing file would be overwritten")

	data, _ := os.ReadFile(filepath.Join(dest, "hive.yaml"))
	assert.Equal(t, "old content\n", string(data), "existing content must be preserved")
}

func TestInstallWithForceOverwrites(t *testing.T) {
	dest := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dest, "hive.yaml"), []byte("old\n"), 0o644))

	manifest := []map[string]string{{"path": "hive.yaml", "content": "new\n"}}
	manifestJSON, _ := json.Marshal(manifest)

	mux := http.NewServeMux()
	mux.HandleFunc("/template.json", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(manifestJSON) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	idxSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]Template{{Name: "demo", Version: "0.1.0", URL: srv.URL + "/template.json"}})
	}))
	defer idxSrv.Close()

	r := NewRegistry()
	r.IndexURL = idxSrv.URL

	_, written, err := r.InstallWith("demo", dest, InstallOptions{Force: true})
	require.NoError(t, err)
	assert.Equal(t, []string{"hive.yaml"}, written)
	data, _ := os.ReadFile(filepath.Join(dest, "hive.yaml"))
	assert.Equal(t, "new\n", string(data))
}
