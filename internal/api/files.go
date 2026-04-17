package api

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// maxListEntries caps the file tree walk so a misconfigured workdir
// (accidentally pointed at $HOME) can't blow the response up.
const maxListEntries = 5000

// maxFileBytes caps how much of a single file we ship back. Anything
// bigger is almost certainly binary, an artefact blob, or generated;
// the dashboard isn't the right tool to read it.
const maxFileBytes = 1 << 20 // 1 MiB

// skipDirs are directories we never walk into. These almost always
// contain more bytes than the user's source code, and opening them
// risks dereferencing huge trees (node_modules alone can blow through
// the entry cap).
var skipDirs = map[string]struct{}{
	".git":         {},
	"node_modules": {},
	".svelte-kit":  {},
	".next":        {},
	"dist":         {},
	"build":        {},
	".cache":       {},
	"__pycache__":  {},
	".venv":        {},
	"vendor":       {},
	".turbo":       {},
	"target":       {}, // Rust
}

// fileEntry is what the list endpoint returns per path. Paths are
// always relative to the project root so the frontend can use them
// verbatim when asking for content.
type fileEntry struct {
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	IsDir    bool      `json:"is_dir"`
	Modified time.Time `json:"modified"`
}

// projectRoot returns the directory we expose for a given project: the
// repo_path if the operator wired up an existing codebase, otherwise
// the workdir. Empty string means nothing to show (project hasn't been
// bound to a directory yet).
func (s *Server) projectRoot(ctx interface{ Value(any) any }, projectID string) (string, error) {
	// ctx is interface{} to sidestep importing context just for this
	// helper signature — callers pass r.Context() which satisfies it.
	_ = ctx
	var root string
	// Prefer repo_path when set (explicit override for "add BMAD to an
	// existing codebase"), fall back to workdir (the generated build).
	row := s.db().QueryRow(
		`SELECT COALESCE(repo_path, ''), COALESCE(workdir, '')
		 FROM projects WHERE id = ?`, projectID)
	var repoPath, workdir string
	if err := row.Scan(&repoPath, &workdir); err != nil {
		return "", err
	}
	if repoPath != "" {
		root = repoPath
	} else {
		root = workdir
	}
	return root, nil
}

// resolveInside validates that `rel` stays inside `root` after
// resolution. Rejects .. tricks and absolute paths. Returns the
// absolute path when the input is safe.
func resolveInside(root, rel string) (string, error) {
	if root == "" {
		return "", errors.New("project has no working directory")
	}
	if filepath.IsAbs(rel) {
		return "", errors.New("absolute paths are not allowed")
	}
	// Clean collapses any ../ segments; joining then cleaning lets us
	// detect escape attempts by checking the final prefix.
	abs := filepath.Join(root, rel)
	absClean := filepath.Clean(abs)
	rootClean := filepath.Clean(root)
	if absClean != rootClean && !strings.HasPrefix(absClean, rootClean+string(filepath.Separator)) {
		return "", errors.New("path escapes project root")
	}
	return absClean, nil
}

// handleListFiles walks the project root and returns every file (not
// directory) up to maxListEntries. Directory traversal skips known
// build/artefact folders and hidden directories.
func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	if s.projectStore == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_PROJECT_STORE",
			"project subsystem is not configured on this node")
		return
	}
	id := r.PathValue("id")
	root, err := s.projectRoot(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	if root == "" {
		writeJSON(w, map[string]any{
			"root":    "",
			"files":   []fileEntry{},
			"message": "project is not bound to a directory yet",
		})
		return
	}
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, map[string]any{
				"root":    root,
				"files":   []fileEntry{},
				"message": "working directory does not exist on disk yet",
			})
			return
		}
		writeError(w, http.StatusInternalServerError, "STAT_FAILED", err.Error())
		return
	}
	if !info.IsDir() {
		writeError(w, http.StatusInternalServerError, "NOT_A_DIR",
			"project root is a file, not a directory")
		return
	}

	entries := make([]fileEntry, 0, 256)
	truncated := false
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			// Skip unreadable entries rather than aborting the whole walk.
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			// Skip the root's hidden/build dirs but let the root itself pass.
			if path == root {
				return nil
			}
			if strings.HasPrefix(name, ".") {
				return fs.SkipDir
			}
			if _, skip := skipDirs[name]; skip {
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, ".") {
			return nil
		}
		if len(entries) >= maxListEntries {
			truncated = true
			return filepath.SkipAll
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		entries = append(entries, fileEntry{
			Path:     filepath.ToSlash(rel),
			Size:     info.Size(),
			IsDir:    false,
			Modified: info.ModTime(),
		})
		return nil
	})
	if walkErr != nil && !errors.Is(walkErr, filepath.SkipAll) {
		writeError(w, http.StatusInternalServerError, "WALK_FAILED", walkErr.Error())
		return
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })

	writeJSON(w, map[string]any{
		"root":      root,
		"files":     entries,
		"truncated": truncated,
	})
}

// handleFileContent returns a single file's content. Path traversal is
// blocked via resolveInside. Binary files are detected with a naive
// utf8 check and returned with a flag so the UI can show a placeholder
// instead of garbage.
func (s *Server) handleFileContent(w http.ResponseWriter, r *http.Request) {
	if s.projectStore == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_PROJECT_STORE",
			"project subsystem is not configured on this node")
		return
	}
	id := r.PathValue("id")
	rel := r.URL.Query().Get("path")
	if rel == "" {
		writeError(w, http.StatusBadRequest, "MISSING_PATH", "path query param is required")
		return
	}
	root, err := s.projectRoot(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	abs, err := resolveInside(root, rel)
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_PATH", err.Error())
		return
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "FILE_NOT_FOUND", "file does not exist")
			return
		}
		writeError(w, http.StatusInternalServerError, "STAT_FAILED", err.Error())
		return
	}
	if info.IsDir() {
		writeError(w, http.StatusBadRequest, "IS_DIR", "path points at a directory, not a file")
		return
	}
	f, err := os.Open(abs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "OPEN_FAILED", err.Error())
		return
	}
	defer f.Close()
	var buf bytes.Buffer
	n, err := io.Copy(&buf, io.LimitReader(f, maxFileBytes+1))
	if err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusInternalServerError, "READ_FAILED", err.Error())
		return
	}
	truncated := n > maxFileBytes
	data := buf.Bytes()
	if truncated {
		data = data[:maxFileBytes]
	}
	// utf8.Valid catches most binaries; a well-formed utf8 "string"
	// with a NUL early is still almost certainly binary, so check for
	// one in the first KB.
	peek := data
	if len(peek) > 1024 {
		peek = peek[:1024]
	}
	isBinary := !utf8.Valid(data) || bytes.IndexByte(peek, 0) >= 0

	resp := map[string]any{
		"path":      filepath.ToSlash(rel),
		"size":      info.Size(),
		"modified":  info.ModTime(),
		"is_binary": isBinary,
		"truncated": truncated,
	}
	if !isBinary {
		resp["content"] = string(data)
	}
	writeJSON(w, resp)
}

