package hivehub

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Template represents a published hive template in the registry.
type Template struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Author      string `json:"author"`
	Version     string `json:"version"`
	Category    string `json:"category"`
	URL         string `json:"url"`
	Downloads   int    `json:"downloads"`
}

// Registry manages HiveHub template discovery and installation.
type Registry struct {
	IndexURL string // URL to the HiveHub index.json
	client   *http.Client
}

// DefaultRegistryURL is the default HiveHub registry index.
const DefaultRegistryURL = "https://raw.githubusercontent.com/JulienLeotier/hivehub/main/index.json"

// NewRegistry creates a HiveHub registry client.
func NewRegistry() *Registry {
	return &Registry{
		IndexURL: DefaultRegistryURL,
		client:   &http.Client{Timeout: 15 * time.Second},
	}
}

// Search finds templates matching the query.
func (r *Registry) Search(query string) ([]Template, error) {
	templates, err := r.fetchIndex()
	if err != nil {
		return nil, err
	}

	if query == "" {
		return templates, nil
	}

	q := strings.ToLower(query)
	var results []Template
	for _, t := range templates {
		text := strings.ToLower(t.Name + " " + t.Description + " " + t.Category)
		if strings.Contains(text, q) {
			results = append(results, t)
		}
	}
	return results, nil
}

// Get retrieves a specific template by name.
func (r *Registry) Get(name string) (*Template, error) {
	templates, err := r.fetchIndex()
	if err != nil {
		return nil, err
	}
	for _, t := range templates {
		if t.Name == name {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("template %q not found in HiveHub", name)
}

// InstallOptions customises an Install call.
type InstallOptions struct {
	// Force overwrites existing files; otherwise they're skipped and reported.
	Force bool
	// Confirm is called when a file already exists and Force is false.
	// Returning true overwrites, false skips. Nil Confirm treats existing
	// files as "skip" (safe default; wire it to a prompt in the CLI).
	Confirm func(path string) bool
}

// Install downloads a template tarball into dest/. dest is created if missing.
// Returns the list of files written. Only http(s) template URLs are accepted.
func (r *Registry) Install(name, dest string) (*Template, []string, error) {
	return r.InstallWith(name, dest, InstallOptions{})
}

// InstallWith is the configurable variant of Install.
func (r *Registry) InstallWith(name, dest string, opts InstallOptions) (*Template, []string, error) {
	tmpl, err := r.Get(name)
	if err != nil {
		return nil, nil, err
	}
	if !strings.HasPrefix(tmpl.URL, "http://") && !strings.HasPrefix(tmpl.URL, "https://") {
		return nil, nil, fmt.Errorf("template URL must be http(s), got %q", tmpl.URL)
	}

	resp, err := r.client.Get(tmpl.URL)
	if err != nil {
		return nil, nil, fmt.Errorf("fetching template: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("template download returned %d", resp.StatusCode)
	}

	if err := ensureDir(dest); err != nil {
		return nil, nil, err
	}

	// For v0.3 we support a JSON manifest of files; tarball support lands in
	// a later story. A manifest is a list of {path, content} pairs.
	data, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20))
	if err != nil {
		return nil, nil, fmt.Errorf("reading template body: %w", err)
	}
	var manifest []struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, nil, fmt.Errorf("parsing template manifest: %w", err)
	}

	var written []string
	for _, f := range manifest {
		if strings.Contains(f.Path, "..") {
			return nil, nil, fmt.Errorf("unsafe path in manifest: %s", f.Path)
		}
		// Story 14.3: don't silently overwrite existing files.
		if existing := joinPath(dest, f.Path); fileExists(existing) && !opts.Force {
			if opts.Confirm == nil || !opts.Confirm(f.Path) {
				continue
			}
		}
		if err := writeTemplateFile(dest, f.Path, f.Content); err != nil {
			return nil, nil, err
		}
		written = append(written, f.Path)
	}
	return tmpl, written, nil
}

// PublishDir packages a directory as a JSON manifest ready to POST to a registry
// endpoint. Returns the manifest bytes. The registry upload itself is out of
// scope for v0.3 — callers pipe the manifest into a PR against the index.
func (r *Registry) PublishDir(dir string, meta Template) ([]byte, error) {
	var manifest []map[string]string
	if err := walkTemplateDir(dir, func(relPath, content string) {
		manifest = append(manifest, map[string]string{"path": relPath, "content": content})
	}); err != nil {
		return nil, err
	}
	if len(manifest) == 0 {
		return nil, fmt.Errorf("no files to publish in %s", dir)
	}
	return json.MarshalIndent(map[string]any{
		"template": meta,
		"files":    manifest,
	}, "", "  ")
}

func (r *Registry) fetchIndex() ([]Template, error) {
	resp, err := r.client.Get(r.IndexURL)
	if err != nil {
		return nil, fmt.Errorf("fetching HiveHub index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HiveHub returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("reading HiveHub index: %w", err)
	}

	var templates []Template
	if err := json.Unmarshal(data, &templates); err != nil {
		return nil, fmt.Errorf("parsing HiveHub index: %w", err)
	}
	return templates, nil
}
