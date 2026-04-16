package cluster

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"sort"
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

// Roster exposes the persisted cluster membership table. Story 22.2.
type Roster struct {
	db *sql.DB
}

// NewRoster builds a roster backed by the given database.
func NewRoster(db *sql.DB) *Roster { return &Roster{db: db} }

// Heartbeat upserts this node's entry with a fresh timestamp.
func (r *Roster) Heartbeat(ctx context.Context, n *Node) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO cluster_members (node_id, hostname, address, status, last_heartbeat)
		 VALUES (?, ?, ?, ?, datetime('now'))
		 ON CONFLICT(node_id) DO UPDATE SET
		    hostname = excluded.hostname,
		    address = excluded.address,
		    status = excluded.status,
		    last_heartbeat = excluded.last_heartbeat`,
		n.ID, n.Hostname, n.Address, n.Status)
	return err
}

// List returns all nodes known to the cluster, newest heartbeat first.
func (r *Roster) List(ctx context.Context) ([]*Node, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT node_id, hostname, address, status, last_heartbeat
		 FROM cluster_members ORDER BY last_heartbeat DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []*Node
	for rows.Next() {
		n := &Node{}
		var ts string
		if err := rows.Scan(&n.ID, &n.Hostname, &n.Address, &n.Status, &ts); err != nil {
			return nil, err
		}
		if parsed, err := time.Parse("2006-01-02 15:04:05", ts); err == nil {
			n.StartedAt = parsed
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

// MarkStale moves nodes to status=offline if their last heartbeat is older than maxAge.
func (r *Roster) MarkStale(ctx context.Context, maxAge time.Duration) (int, error) {
	cutoff := time.Now().Add(-maxAge).UTC().Format("2006-01-02 15:04:05")
	res, err := r.db.ExecContext(ctx,
		`UPDATE cluster_members SET status = 'offline'
		 WHERE status != 'offline' AND last_heartbeat < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// Remove drops a node from the roster.
func (r *Roster) Remove(ctx context.Context, nodeID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM cluster_members WHERE node_id = ?`, nodeID)
	return err
}

// ---------------- Node-aware routing (Story 22.3) ----------------

// PickAgent picks the preferred agent name for a task type given the per-node
// agent bindings. Local-first mode prefers this node; best-fit round-robins.
func (m *Manager) PickAgent(perNode map[string][]string, taskType string) string {
	if len(perNode) == 0 {
		return ""
	}

	self := m.Self().ID
	if m.ShouldPreferLocal() {
		if local, ok := perNode[self]; ok {
			for _, a := range local {
				return a
			}
		}
	}

	// Fallback / best-fit: deterministic round-robin across sorted nodes.
	nodes := make([]string, 0, len(perNode))
	for n := range perNode {
		nodes = append(nodes, n)
	}
	sort.Strings(nodes)
	for _, n := range nodes {
		for _, a := range perNode[n] {
			return a
		}
	}
	return ""
}
