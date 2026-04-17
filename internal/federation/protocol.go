package federation

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Link represents a federation connection to another Hive deployment.
type Link struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	URL           string   `json:"url"`
	Status        string   `json:"status"` // "active", "degraded", "disconnected"
	SharedCaps    []string `json:"shared_capabilities"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
}

// SharedCapability represents a capability offered by a federated hive.
type SharedCapability struct {
	HiveName   string `json:"hive_name"`
	HiveURL    string `json:"hive_url"`
	Capability string `json:"capability"`
	AgentCount int    `json:"agent_count"`
}

// Manager handles federation links and capability discovery.
type Manager struct {
	mu     sync.Mutex
	links  map[string]*Link
	client *http.Client
}

// NewManager creates a federation manager.
func NewManager() *Manager {
	return &Manager{
		links:  make(map[string]*Link),
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Connect establishes a federation link with another Hive.
func (m *Manager) Connect(ctx context.Context, name, url string, sharedCaps []string) (*Link, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	link := &Link{
		ID:         name,
		Name:       name,
		URL:        url,
		Status:     "active",
		SharedCaps: sharedCaps,
		LastHeartbeat: time.Now(),
	}

	// Verify connectivity
	caps, err := m.discoverCapabilities(ctx, url)
	if err != nil {
		link.Status = "disconnected"
		slog.Warn("federation link degraded", "name", name, "error", err)
	} else {
		slog.Info("federation link established", "name", name, "remote_capabilities", len(caps))
	}

	m.links[name] = link
	return link, nil
}

// Disconnect removes a federation link.
func (m *Manager) Disconnect(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.links[name]; !ok {
		return fmt.Errorf("federation link %s not found", name)
	}
	delete(m.links, name)
	slog.Info("federation link removed", "name", name)
	return nil
}

// ListLinks returns all federation links.
func (m *Manager) ListLinks() []*Link {
	m.mu.Lock()
	defer m.mu.Unlock()
	links := make([]*Link, 0, len(m.links))
	for _, l := range m.links {
		links = append(links, l)
	}
	return links
}

// FindRemoteCapability searches federated hives for a specific capability.
func (m *Manager) FindRemoteCapability(ctx context.Context, capability string) ([]SharedCapability, error) {
	var results []SharedCapability
	for _, link := range m.links {
		if link.Status != "active" {
			continue
		}
		caps, err := m.discoverCapabilities(ctx, link.URL)
		if err != nil {
			continue
		}
		for _, cap := range caps {
			if cap == capability {
				results = append(results, SharedCapability{
					HiveName:   link.Name,
					HiveURL:    link.URL,
					Capability: capability,
				})
			}
		}
	}
	return results, nil
}

func (m *Manager) discoverCapabilities(ctx context.Context, baseURL string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/v1/capabilities", nil)
	if err != nil {
		return nil, err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("discovering capabilities: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var result struct {
		Data []string `json:"data"`
	}
	_ = json.Unmarshal(data, &result)
	return result.Data, nil
}
