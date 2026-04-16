package workflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"crypto/rand"

	"github.com/JulienLeotier/hive/internal/event"
	"github.com/oklog/ulid/v2"
)

// Status constants for workflows.
const (
	StatusIdle      = "idle"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

// Workflow represents a registered workflow.
type Workflow struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Config    string    `json:"config"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// Store manages workflow persistence.
type Store struct {
	db  *sql.DB
	bus *event.Bus
}

// NewStore creates a workflow store.
func NewStore(db *sql.DB, bus *event.Bus) *Store {
	return &Store{db: db, bus: bus}
}

// Create registers a new workflow from parsed config.
func (s *Store) Create(ctx context.Context, name string, config *Config) (*Workflow, error) {
	id := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)
	cfgJSON, _ := json.Marshal(config)

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO workflows (id, name, config, status) VALUES (?, ?, ?, ?)`,
		id.String(), name, string(cfgJSON), StatusIdle,
	)
	if err != nil {
		return nil, fmt.Errorf("creating workflow %s: %w", name, err)
	}

	slog.Info("workflow created", "id", id.String(), "name", name)
	return &Workflow{
		ID:     id.String(),
		Name:   name,
		Config: string(cfgJSON),
		Status: StatusIdle,
	}, nil
}

// UpdateStatus changes the workflow status and emits events.
func (s *Store) UpdateStatus(ctx context.Context, id, status string) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE workflows SET status = ? WHERE id = ?`, status, id,
	)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("workflow %s not found", id)
	}

	var evtType string
	switch status {
	case StatusRunning:
		evtType = event.WorkflowStarted
	case StatusCompleted:
		evtType = event.WorkflowCompleted
	case StatusFailed:
		evtType = event.WorkflowFailed
	}
	if evtType != "" {
		s.bus.Publish(ctx, evtType, "system", map[string]string{"workflow_id": id})
	}
	return nil
}

// GetByID retrieves a workflow by ID.
func (s *Store) GetByID(ctx context.Context, id string) (*Workflow, error) {
	var w Workflow
	var created string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, config, status, created_at FROM workflows WHERE id = ?`, id,
	).Scan(&w.ID, &w.Name, &w.Config, &w.Status, &created)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("workflow %s not found", id)
	}
	if err != nil {
		return nil, err
	}
	w.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
	return &w, nil
}

// List returns workflows with a default limit of 1000.
func (s *Store) List(ctx context.Context) ([]Workflow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, config, status, created_at FROM workflows ORDER BY created_at DESC LIMIT 1000`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workflows []Workflow
	for rows.Next() {
		var w Workflow
		var created string
		if err := rows.Scan(&w.ID, &w.Name, &w.Config, &w.Status, &created); err != nil {
			return nil, err
		}
		w.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		workflows = append(workflows, w)
	}
	return workflows, rows.Err()
}
