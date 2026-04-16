package task

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/JulienLeotier/hive/internal/adapter"
)

// Router matches tasks to capable agents based on declared capabilities.
type Router struct {
	db *sql.DB
}

// NewRouter creates a task router.
func NewRouter(db *sql.DB) *Router {
	return &Router{db: db}
}

// FindCapableAgent returns the ID and name of a healthy agent capable of handling the given task type.
// Returns empty strings if no capable agent is available.
func (r *Router) FindCapableAgent(ctx context.Context, taskType string) (agentID, agentName string, err error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, capabilities FROM agents WHERE health_status = 'healthy' ORDER BY name`,
	)
	if err != nil {
		return "", "", fmt.Errorf("querying agents: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, name, capsJSON string
		if err := rows.Scan(&id, &name, &capsJSON); err != nil {
			continue
		}

		var caps adapter.AgentCapabilities
		if err := json.Unmarshal([]byte(capsJSON), &caps); err != nil {
			continue
		}

		for _, tt := range caps.TaskTypes {
			if tt == taskType {
				slog.Debug("routed task to agent", "task_type", taskType, "agent", name)
				return id, name, nil
			}
		}
	}

	return "", "", nil // no capable agent found
}
