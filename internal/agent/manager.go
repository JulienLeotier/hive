package agent

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/JulienLeotier/hive/internal/adapter"
	"github.com/oklog/ulid/v2"
)

// Publisher is the minimal surface the manager needs to broadcast lifecycle
// events. Usually backed by event.Bus (SQLite) or event.NATSBus (clustered).
type Publisher func(ctx context.Context, eventType, source string, payload any) error

// Manager handles agent registration, listing, and removal.
type Manager struct {
	db  *sql.DB
	bus Publisher
}

// NewManager creates an agent manager backed by the given database.
func NewManager(db *sql.DB) *Manager {
	return &Manager{db: db}
}

// WithPublisher installs a publisher so Register/Remove emit events; Story 22.2
// uses this to replicate registrations across a NATS cluster.
func (m *Manager) WithPublisher(p Publisher) *Manager {
	m.bus = p
	return m
}

// Register adds a new agent to the hive after validating connectivity.
func (m *Manager) Register(ctx context.Context, name, agentType, baseURL string) (*Agent, error) {
	a := adapter.NewHTTPAdapter(baseURL)

	// Validate connectivity via health check
	health, err := a.Health(ctx)
	if err != nil {
		return nil, fmt.Errorf("health check failed for %s: %w", name, err)
	}

	// Get capabilities via declare
	caps, err := a.Declare(ctx)
	if err != nil {
		return nil, fmt.Errorf("declare failed for %s: %w", name, err)
	}

	capsJSON, err := json.Marshal(caps)
	if err != nil {
		return nil, fmt.Errorf("marshaling capabilities: %w", err)
	}

	configJSON, err := json.Marshal(map[string]string{"base_url": baseURL})
	if err != nil {
		return nil, fmt.Errorf("marshaling config: %w", err)
	}

	id := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)

	_, err = m.db.ExecContext(ctx,
		`INSERT INTO agents (id, name, type, config, capabilities, health_status, trust_level)
		 VALUES (?, ?, ?, ?, ?, ?, 'scripted')`,
		id.String(), name, agentType, string(configJSON), string(capsJSON), health.Status,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting agent %s: %w", name, err)
	}

	slog.Info("agent registered", "name", name, "type", agentType, "health", health.Status)

	if m.bus != nil {
		_ = m.bus(ctx, "agent.registered", name, map[string]string{
			"id": id.String(), "type": agentType, "url": baseURL,
		})
	}

	return &Agent{
		ID:           id.String(),
		Name:         name,
		Type:         agentType,
		Config:       string(configJSON),
		Capabilities: string(capsJSON),
		HealthStatus: health.Status,
		TrustLevel:   "scripted",
	}, nil
}

// RegisterLocal registers a local-only agent that cannot be HTTP-health-checked.
// The capabilities map is stored as-is; the caller is responsible for populating it.
func (m *Manager) RegisterLocal(ctx context.Context, name, agentType, path string, caps map[string]any) (*Agent, error) {
	if caps == nil {
		caps = map[string]any{"name": name, "task_types": []string{}}
	}
	capsJSON, err := json.Marshal(caps)
	if err != nil {
		return nil, fmt.Errorf("marshaling capabilities: %w", err)
	}
	configJSON, err := json.Marshal(map[string]string{"path": path})
	if err != nil {
		return nil, fmt.Errorf("marshaling config: %w", err)
	}

	id := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)

	if _, err := m.db.ExecContext(ctx,
		`INSERT INTO agents (id, name, type, config, capabilities, health_status, trust_level)
		 VALUES (?, ?, ?, ?, ?, 'healthy', 'scripted')`,
		id.String(), name, agentType, string(configJSON), string(capsJSON),
	); err != nil {
		return nil, fmt.Errorf("inserting local agent %s: %w", name, err)
	}

	slog.Info("local agent registered", "name", name, "type", agentType, "path", path)
	return &Agent{
		ID:           id.String(),
		Name:         name,
		Type:         agentType,
		Config:       string(configJSON),
		Capabilities: string(capsJSON),
		HealthStatus: "healthy",
		TrustLevel:   "scripted",
	}, nil
}

// List returns registered agents with a default limit of 1000.
func (m *Manager) List(ctx context.Context) ([]Agent, error) {
	return m.ListWithLimit(ctx, 1000)
}

// ListWithLimit returns registered agents up to the given limit.
func (m *Manager) ListWithLimit(ctx context.Context, limit int) ([]Agent, error) {
	rows, err := m.db.QueryContext(ctx,
		`SELECT id, name, type, config, capabilities, health_status, trust_level, created_at, updated_at
		 FROM agents ORDER BY name LIMIT ?`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("listing agents: %w", err)
	}
	defer rows.Close()

	var agents []Agent
	for rows.Next() {
		var a Agent
		var createdAt, updatedAt string
		if err := rows.Scan(&a.ID, &a.Name, &a.Type, &a.Config, &a.Capabilities,
			&a.HealthStatus, &a.TrustLevel, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scanning agent row: %w", err)
		}
		a.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		a.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

// Remove deletes an agent by name.
// Story 1.3 AC: "any queued tasks for that agent are returned to the unassigned pool".
func (m *Manager) Remove(ctx context.Context, name string) error {
	// Find the agent ID so we can requeue its tasks before deleting.
	var agentID string
	_ = m.db.QueryRowContext(ctx, `SELECT id FROM agents WHERE name = ?`, name).Scan(&agentID)

	requeued := int64(0)
	if agentID != "" {
		res, err := m.db.ExecContext(ctx,
			`UPDATE tasks SET status = 'pending', agent_id = ''
			 WHERE agent_id = ? AND status IN ('assigned','running')`, agentID)
		if err != nil {
			return fmt.Errorf("requeueing tasks for agent %s: %w", name, err)
		}
		requeued, _ = res.RowsAffected()
	}

	result, err := m.db.ExecContext(ctx, `DELETE FROM agents WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("removing agent %s: %w", name, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("agent %s not found", name)
	}
	slog.Info("agent removed", "name", name, "requeued_tasks", requeued)
	if m.bus != nil {
		_ = m.bus(ctx, "agent.removed", name, map[string]any{
			"name":           name,
			"requeued_tasks": requeued,
		})
	}
	return nil
}

// GetByName retrieves an agent by name.
func (m *Manager) GetByName(ctx context.Context, name string) (*Agent, error) {
	var a Agent
	var createdAt, updatedAt string
	err := m.db.QueryRowContext(ctx,
		`SELECT id, name, type, config, capabilities, health_status, trust_level, created_at, updated_at
		 FROM agents WHERE name = ?`, name,
	).Scan(&a.ID, &a.Name, &a.Type, &a.Config, &a.Capabilities,
		&a.HealthStatus, &a.TrustLevel, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("agent %s not found", name)
	}
	if err != nil {
		return nil, fmt.Errorf("getting agent %s: %w", name, err)
	}
	a.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	a.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &a, nil
}

// UpdateHealth refreshes the health status of an agent.
func (m *Manager) UpdateHealth(ctx context.Context, name string, status string) error {
	_, err := m.db.ExecContext(ctx,
		`UPDATE agents SET health_status = ?, updated_at = datetime('now') WHERE name = ?`,
		status, name,
	)
	return err
}
