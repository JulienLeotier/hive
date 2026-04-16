package cluster

import (
	"fmt"
	"log/slog"
	"os"
	"time"
)

// Node represents a Hive node in a multi-node cluster.
type Node struct {
	ID        string    `json:"id"`
	Hostname  string    `json:"hostname"`
	Address   string    `json:"address"`
	Status    string    `json:"status"` // "active", "draining", "offline"
	StartedAt time.Time `json:"started_at"`
}

// Config holds cluster configuration.
type Config struct {
	Enabled    bool   `yaml:"enabled"`
	NodeID     string `yaml:"node_id"`
	NATSUrl    string `yaml:"nats_url"`
	StorageType string `yaml:"storage"` // "sqlite" or "postgres"
	PostgresURL string `yaml:"postgres_url"`
	RoutingMode string `yaml:"routing"` // "local-first" or "best-fit"
}

// Manager handles multi-node cluster coordination.
type Manager struct {
	config Config
	self   *Node
}

// NewManager creates a cluster manager for this node.
func NewManager(cfg Config) *Manager {
	hostname, _ := os.Hostname()
	nodeID := cfg.NodeID
	if nodeID == "" {
		nodeID = hostname
	}

	self := &Node{
		ID:        nodeID,
		Hostname:  hostname,
		Address:   fmt.Sprintf(":%d", 8233),
		Status:    "active",
		StartedAt: time.Now(),
	}

	slog.Info("cluster node initialized",
		"node_id", self.ID,
		"storage", cfg.StorageType,
		"routing", cfg.RoutingMode,
	)

	return &Manager{config: cfg, self: self}
}

// Self returns this node's info.
func (m *Manager) Self() *Node {
	return m.self
}

// IsMultiNode returns true if clustering is enabled.
func (m *Manager) IsMultiNode() bool {
	return m.config.Enabled
}

// ShouldPreferLocal returns true if routing should prefer local agents.
func (m *Manager) ShouldPreferLocal() bool {
	return m.config.RoutingMode == "local-first" || m.config.RoutingMode == ""
}

// StorageType returns the configured storage backend.
func (m *Manager) StorageType() string {
	if m.config.StorageType == "" {
		return "sqlite"
	}
	return m.config.StorageType
}
