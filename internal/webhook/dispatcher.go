package webhook

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"crypto/rand"

	"github.com/JulienLeotier/hive/internal/event"
	"github.com/oklog/ulid/v2"
)

// Config represents a webhook configuration stored in the database.
type Config struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Type        string `json:"type"` // "slack", "github", "generic"
	EventFilter string `json:"event_filter"`
	Enabled     bool   `json:"enabled"`
}

// Dispatcher manages webhook configurations and delivers notifications.
type Dispatcher struct {
	db     *sql.DB
	client *http.Client
}

// NewDispatcher creates a webhook dispatcher.
func NewDispatcher(db *sql.DB) *Dispatcher {
	return &Dispatcher{
		db:     db,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Add registers a new webhook configuration.
func (d *Dispatcher) Add(ctx context.Context, name, url, whType, eventFilter string) (*Config, error) {
	id := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)

	_, err := d.db.ExecContext(ctx,
		`INSERT INTO webhooks (id, name, url, type, event_filter, enabled) VALUES (?, ?, ?, ?, ?, 1)`,
		id.String(), name, url, whType, eventFilter,
	)
	if err != nil {
		return nil, fmt.Errorf("adding webhook %s: %w", name, err)
	}

	return &Config{ID: id.String(), Name: name, URL: url, Type: whType, EventFilter: eventFilter, Enabled: true}, nil
}

// List returns all webhook configurations.
func (d *Dispatcher) List(ctx context.Context) ([]Config, error) {
	rows, err := d.db.QueryContext(ctx, `SELECT id, name, url, type, COALESCE(event_filter,''), enabled FROM webhooks ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []Config
	for rows.Next() {
		var c Config
		var enabled int
		if err := rows.Scan(&c.ID, &c.Name, &c.URL, &c.Type, &c.EventFilter, &enabled); err != nil {
			continue
		}
		c.Enabled = enabled == 1
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

// Dispatch sends notifications for an event to all matching webhooks.
func (d *Dispatcher) Dispatch(ctx context.Context, evt event.Event) {
	configs, err := d.List(ctx)
	if err != nil {
		slog.Error("loading webhooks for dispatch", "error", err)
		return
	}

	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}
		if !matchesFilter(evt.Type, cfg.EventFilter) {
			continue
		}
		go d.deliver(cfg, evt)
	}
}

// deliver sends the webhook with retry (3 attempts, exponential backoff).
func (d *Dispatcher) deliver(cfg Config, evt event.Event) {
	payload := formatPayload(cfg.Type, evt)

	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<attempt) * time.Second) // 2s, 4s
		}

		req, err := http.NewRequest("POST", cfg.URL, bytes.NewReader(payload))
		if err != nil {
			slog.Error("webhook request creation failed", "webhook", cfg.Name, "error", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := d.client.Do(req)
		if err != nil {
			slog.Warn("webhook delivery failed", "webhook", cfg.Name, "attempt", attempt+1, "error", err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			slog.Debug("webhook delivered", "webhook", cfg.Name, "event", evt.Type)
			return
		}
		slog.Warn("webhook non-2xx response", "webhook", cfg.Name, "status", resp.StatusCode, "attempt", attempt+1)
	}
	slog.Error("webhook delivery exhausted retries", "webhook", cfg.Name, "event", evt.Type)
}

func matchesFilter(eventType, filter string) bool {
	if filter == "" {
		return true // no filter = match all
	}
	var types []string
	json.Unmarshal([]byte(filter), &types)
	if len(types) == 0 {
		// Try comma-separated fallback
		for _, t := range splitComma(filter) {
			if eventType == t || (len(t) > 0 && t[len(t)-1] == '*' && len(eventType) >= len(t)-1 && eventType[:len(t)-1] == t[:len(t)-1]) {
				return true
			}
		}
		return false
	}
	for _, t := range types {
		if eventType == t {
			return true
		}
	}
	return false
}

func splitComma(s string) []string {
	var result []string
	for _, part := range bytes.Split([]byte(s), []byte(",")) {
		trimmed := bytes.TrimSpace(part)
		if len(trimmed) > 0 {
			result = append(result, string(trimmed))
		}
	}
	return result
}

func formatPayload(whType string, evt event.Event) []byte {
	switch whType {
	case "slack":
		msg := map[string]string{
			"text": fmt.Sprintf("[Hive] %s from %s: %s", evt.Type, evt.Source, evt.Payload),
		}
		data, _ := json.Marshal(msg)
		return data
	case "github":
		msg := map[string]any{
			"event_type": evt.Type,
			"client_payload": map[string]any{
				"source":  evt.Source,
				"payload": evt.Payload,
			},
		}
		data, _ := json.Marshal(msg)
		return data
	default: // generic
		data, _ := json.Marshal(map[string]any{
			"id":         evt.ID,
			"type":       evt.Type,
			"source":     evt.Source,
			"payload":    evt.Payload,
			"created_at": evt.CreatedAt,
		})
		return data
	}
}
