package knowledge

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"

	"github.com/JulienLeotier/hive/internal/event"
)

// AutoRecorder subscribes to task.completed / task.failed events and writes
// one knowledge entry per transition. Story 10.1 AC: "When a task completes
// (success or failure), the approach and outcome are stored in the knowledge
// table".
//
// The approach text is derived from the task input; context is the full task
// row JSON so callers can filter later.
type AutoRecorder struct {
	db    *sql.DB
	store *Store
}

// NewAutoRecorder builds an auto-recorder. Pass the same Store used elsewhere
// so Record respects WithEmbedder configuration.
func NewAutoRecorder(db *sql.DB, store *Store) *AutoRecorder {
	return &AutoRecorder{db: db, store: store}
}

// Attach wires the recorder to an event bus. Call once during startup.
func (a *AutoRecorder) Attach(bus *event.Bus) {
	bus.Subscribe(event.TaskCompleted, func(e event.Event) {
		_ = a.handle(e, "success")
	})
	bus.Subscribe(event.TaskFailed, func(e event.Event) {
		_ = a.handle(e, "failure")
	})
}

func (a *AutoRecorder) handle(e event.Event, outcome string) error {
	var payload map[string]string
	if err := json.Unmarshal([]byte(e.Payload), &payload); err != nil {
		return err
	}
	taskID := payload["task_id"]
	if taskID == "" {
		return nil
	}

	var tType, input, output string
	err := a.db.QueryRow(
		`SELECT type, COALESCE(input,''), COALESCE(output,'') FROM tasks WHERE id = ?`, taskID,
	).Scan(&tType, &input, &output)
	if err != nil {
		return err
	}

	approach := input
	if len(approach) > 512 {
		approach = approach[:512]
	}
	contextJSON, _ := json.Marshal(map[string]string{
		"task_id": taskID,
		"output":  output,
	})

	if err := a.store.Record(context.Background(), tType, approach, outcome, string(contextJSON)); err != nil {
		slog.Warn("knowledge auto-record failed", "task_id", taskID, "error", err)
		return err
	}
	return nil
}
