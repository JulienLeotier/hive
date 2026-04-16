package task

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/JulienLeotier/hive/internal/event"
	"github.com/oklog/ulid/v2"
	"crypto/rand"
)

// Status constants for the task state machine.
const (
	StatusPending   = "pending"
	StatusAssigned  = "assigned"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

// Task represents a unit of work in the system.
type Task struct {
	ID          string    `json:"id"`
	WorkflowID  string    `json:"workflow_id"`
	Type        string    `json:"type"`
	Status      string    `json:"status"`
	AgentID     string    `json:"agent_id,omitempty"`
	Input       string    `json:"input"`
	Output      string    `json:"output,omitempty"`
	Checkpoint  string    `json:"checkpoint,omitempty"`
	DependsOn   string    `json:"depends_on,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// Store manages task persistence and state transitions.
type Store struct {
	db  *sql.DB
	bus *event.Bus
}

// NewStore creates a task store.
func NewStore(db *sql.DB, bus *event.Bus) *Store {
	return &Store{db: db, bus: bus}
}

// Create creates a new task in pending state.
func (s *Store) Create(ctx context.Context, workflowID, taskType, input string, dependsOn []string) (*Task, error) {
	id := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)

	depsJSON, _ := json.Marshal(dependsOn)

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO tasks (id, workflow_id, type, status, input, depends_on) VALUES (?, ?, ?, ?, ?, ?)`,
		id.String(), workflowID, taskType, StatusPending, input, string(depsJSON),
	)
	if err != nil {
		return nil, fmt.Errorf("creating task: %w", err)
	}

	t := &Task{
		ID:         id.String(),
		WorkflowID: workflowID,
		Type:       taskType,
		Status:     StatusPending,
		Input:      input,
		DependsOn:  string(depsJSON),
	}

	s.bus.Publish(ctx, event.TaskCreated, "system", map[string]string{
		"task_id": t.ID, "type": t.Type, "workflow_id": workflowID,
	})

	slog.Info("task created", "id", t.ID, "type", taskType, "workflow", workflowID)
	return t, nil
}

// Assign transitions a task from pending to assigned.
func (s *Store) Assign(ctx context.Context, taskID, agentID string) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE tasks SET status = ?, agent_id = ?
		 WHERE id = ? AND status = ?`,
		StatusAssigned, agentID, taskID, StatusPending,
	)
	if err != nil {
		return fmt.Errorf("assigning task %s: %w", taskID, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("task %s not in pending state", taskID)
	}

	s.bus.Publish(ctx, event.TaskAssigned, "system", map[string]string{
		"task_id": taskID, "agent_id": agentID,
	})
	return nil
}

// Start transitions a task from assigned to running.
func (s *Store) Start(ctx context.Context, taskID string) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE tasks SET status = ?, started_at = datetime('now') WHERE id = ? AND status = ?`,
		StatusRunning, taskID, StatusAssigned,
	)
	if err != nil {
		return fmt.Errorf("starting task %s: %w", taskID, err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("task %s not in assigned state", taskID)
	}

	s.bus.Publish(ctx, event.TaskStarted, "system", map[string]string{"task_id": taskID})
	return nil
}

// Complete transitions a task to completed with output.
func (s *Store) Complete(ctx context.Context, taskID, output string) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE tasks SET status = ?, output = ?, completed_at = datetime('now') WHERE id = ? AND status = ?`,
		StatusCompleted, output, taskID, StatusRunning,
	)
	if err != nil {
		return fmt.Errorf("completing task %s: %w", taskID, err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("task %s not in running state", taskID)
	}

	s.bus.Publish(ctx, event.TaskCompleted, "system", map[string]string{"task_id": taskID})
	return nil
}

// Fail transitions a running task to failed with an error message.
func (s *Store) Fail(ctx context.Context, taskID, errMsg string) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE tasks SET status = ?, output = ?, completed_at = datetime('now') WHERE id = ? AND status = ?`,
		StatusFailed, errMsg, taskID, StatusRunning,
	)
	if err != nil {
		return fmt.Errorf("failing task %s: %w", taskID, err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("task %s not in running state", taskID)
	}

	s.bus.Publish(ctx, event.TaskFailed, "system", map[string]string{
		"task_id": taskID, "error": errMsg,
	})
	return nil
}

// SaveCheckpoint stores a checkpoint for a running task and stamps its timestamp.
func (s *Store) SaveCheckpoint(ctx context.Context, taskID, checkpoint string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE tasks SET checkpoint = ?, checkpoint_at = datetime('now') WHERE id = ?`,
		checkpoint, taskID,
	)
	return err
}

// StaleRunningTasks returns running tasks whose checkpoint is older than maxAge
// (or whose task was started but never checkpointed).
func (s *Store) StaleRunningTasks(ctx context.Context, maxAge time.Duration) ([]Task, error) {
	cutoff := time.Now().Add(-maxAge).UTC().Format("2006-01-02 15:04:05")
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, workflow_id, type, status, COALESCE(agent_id,''), input, COALESCE(checkpoint,''),
		 COALESCE(started_at, created_at), COALESCE(checkpoint_at, started_at, created_at)
		 FROM tasks
		 WHERE status = 'running'
		   AND COALESCE(checkpoint_at, started_at, created_at) < ?`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("querying stale tasks: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		var startedStr, cpStr string
		if err := rows.Scan(&t.ID, &t.WorkflowID, &t.Type, &t.Status, &t.AgentID,
			&t.Input, &t.Checkpoint, &startedStr, &cpStr); err != nil {
			return nil, err
		}
		if ts, err := time.Parse("2006-01-02 15:04:05", startedStr); err == nil {
			t.StartedAt = &ts
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// GetByID retrieves a task by ID.
func (s *Store) GetByID(ctx context.Context, taskID string) (*Task, error) {
	var t Task
	var startedAt, completedAt sql.NullString
	var createdAt string

	err := s.db.QueryRowContext(ctx,
		`SELECT id, workflow_id, type, status, COALESCE(agent_id,''), input, COALESCE(output,''),
		 COALESCE(checkpoint,''), COALESCE(depends_on,'[]'), created_at, started_at, completed_at
		 FROM tasks WHERE id = ?`, taskID,
	).Scan(&t.ID, &t.WorkflowID, &t.Type, &t.Status, &t.AgentID, &t.Input, &t.Output,
		&t.Checkpoint, &t.DependsOn, &createdAt, &startedAt, &completedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("task %s not found", taskID)
	}
	if err != nil {
		return nil, fmt.Errorf("getting task %s: %w", taskID, err)
	}
	t.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return &t, nil
}

// ListByWorkflow returns all tasks for a workflow, ordered by creation.
func (s *Store) ListByWorkflow(ctx context.Context, workflowID string) ([]Task, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, workflow_id, type, status, COALESCE(agent_id,''), input, COALESCE(output,''),
		 COALESCE(checkpoint,''), COALESCE(depends_on,'[]'), created_at
		 FROM tasks WHERE workflow_id = ? ORDER BY created_at`,
		workflowID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		var created string
		if err := rows.Scan(&t.ID, &t.WorkflowID, &t.Type, &t.Status, &t.AgentID,
			&t.Input, &t.Output, &t.Checkpoint, &t.DependsOn, &created); err != nil {
			return nil, err
		}
		t.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// ListPending returns all tasks in pending state matching optional type filter.
func (s *Store) ListPending(ctx context.Context, taskType string) ([]Task, error) {
	query := `SELECT id, workflow_id, type, status, input, COALESCE(depends_on,'[]'), created_at
		 FROM tasks WHERE status = ?`
	args := []any{StatusPending}

	if taskType != "" {
		query += ` AND type = ?`
		args = append(args, taskType)
	}
	query += ` ORDER BY created_at`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		var created string
		if err := rows.Scan(&t.ID, &t.WorkflowID, &t.Type, &t.Status, &t.Input, &t.DependsOn, &created); err != nil {
			return nil, err
		}
		t.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}
