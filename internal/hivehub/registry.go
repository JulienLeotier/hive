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
