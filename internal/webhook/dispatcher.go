package webhook

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/secretstore"
	"github.com/oklog/ulid/v2"
)

// validateWebhookURL rejects URLs targeting private/internal IPs (SSRF prevention).
func validateWebhookURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only http/https URLs allowed, got %s", u.Scheme)
	}
	host := strings.ToLower(u.Hostname())
	blocked := []string{"localhost", "127.0.0.1", "0.0.0.0", "169.254.169.254", "[::1]", "metadata.google.internal"}
	for _, b := range blocked {
		if host == b {
			return fmt.Errorf("URL targeting %s is not allowed (SSRF prevention)", host)
		}
	}
	// Block 10.x.x.x, 172.16-31.x.x, 192.168.x.x
	if strings.HasPrefix(host, "10.") || strings.HasPrefix(host, "192.168.") || strings.HasPrefix(host, "172.") {
		return fmt.Errorf("URL targeting private IP %s is not allowed", host)
	}
	return nil
}

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
// Rejects URLs targeting private/internal IPs to prevent SSRF. When
// HIVE_MASTER_KEY is set, the URL is stored encrypted — users sometimes embed
// bearer tokens in webhook URLs (Slack, Discord, custom auth) and we
// shouldn't leak those to anyone with DB read access.
func (d *Dispatcher) Add(ctx context.Context, name, url, whType, eventFilter string) (*Config, error) {
	if err := validateWebhookURL(url); err != nil {
		return nil, fmt.Errorf("invalid webhook URL: %w", err)
	}

	stored, err := secretstore.Encrypt(url)
	if err != nil {
		return nil, fmt.Errorf("encrypting webhook URL: %w", err)
	}

	id := ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)

	_, err = d.db.ExecContext(ctx,
		`INSERT INTO webhooks (id, name, url, type, event_filter, enabled) VALUES (?, ?, ?, ?, ?, 1)`,
		id.String(), name, stored, whType, eventFilter,
	)
	if err != nil {
		return nil, fmt.Errorf("adding webhook %s: %w", name, err)
	}

	return &Config{ID: id.String(), Name: name, URL: url, Type: whType, EventFilter: eventFilter, Enabled: true}, nil
}

// List returns all webhook configurations with URLs decrypted.
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
		var storedURL string
		if err := rows.Scan(&c.ID, &c.Name, &storedURL, &c.Type, &c.EventFilter, &enabled); err != nil {
			continue
		}
		plain, err := secretstore.Decrypt(storedURL)
		if err != nil {
			slog.Error("webhook URL decrypt failed — skipping", "webhook", c.Name, "error", err)
			continue
		}
		c.URL = plain
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
// Every attempt (success, non-2xx, or transport failure) is written to the
// webhook_deliveries table so operators can audit the history.
func (d *Dispatcher) deliver(cfg Config, evt event.Event) {
	payload := formatPayload(cfg.Type, evt)

	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<attempt) * time.Second) // 2s, 4s
		}

		req, err := http.NewRequest("POST", cfg.URL, bytes.NewReader(payload))
		if err != nil {
			slog.Error("webhook request creation failed", "webhook", cfg.Name, "error", err)
			d.recordDelivery(cfg.Name, evt.Type, attempt+1, 0, err.Error())
			return
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := d.client.Do(req)
		if err != nil {
			slog.Warn("webhook delivery failed", "webhook", cfg.Name, "attempt", attempt+1, "error", err)
			d.recordDelivery(cfg.Name, evt.Type, attempt+1, 0, err.Error())
			continue
		}
		resp.Body.Close()

		d.recordDelivery(cfg.Name, evt.Type, attempt+1, resp.StatusCode, "")

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			slog.Debug("webhook delivered", "webhook", cfg.Name, "event", evt.Type)
			return
		}
		slog.Warn("webhook non-2xx response", "webhook", cfg.Name, "status", resp.StatusCode, "attempt", attempt+1)
	}
	slog.Error("webhook delivery exhausted retries", "webhook", cfg.Name, "event", evt.Type)
}

// Delivery is one row of the webhook_deliveries table — surfaced to the
// dashboard so operators can see why an integration went silent.
type Delivery struct {
	ID         int64  `json:"id"`
	Webhook    string `json:"webhook_name"`
	EventType  string `json:"event_type"`
	Attempt    int    `json:"attempt"`
	StatusCode int    `json:"status_code"`
	Error      string `json:"error,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// recordDelivery writes one attempt to the history table. Non-blocking
// failure logging: if the write fails we just slog.Warn and move on — the
// audit row is nice-to-have, not worth breaking a live delivery for.
func (d *Dispatcher) recordDelivery(name, eventType string, attempt, statusCode int, errMsg string) {
	_, err := d.db.ExecContext(context.Background(),
		`INSERT INTO webhook_deliveries (webhook_name, event_type, attempt, status_code, error_message)
		 VALUES (?, ?, ?, ?, ?)`,
		name, eventType, attempt, statusCode, errMsg,
	)
	if err != nil {
		slog.Warn("webhook delivery history insert failed", "webhook", name, "error", err)
	}
}

// Deliveries returns the last `limit` delivery attempts for the named
// webhook, newest first. Used by the /api/v1/webhooks/{name}/deliveries
// endpoint to populate the dashboard history panel.
func (d *Dispatcher) Deliveries(ctx context.Context, webhookName string, limit int) ([]Delivery, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := d.db.QueryContext(ctx,
		`SELECT id, webhook_name, event_type, attempt, status_code,
		        COALESCE(error_message,''), created_at
		 FROM webhook_deliveries
		 WHERE webhook_name = ?
		 ORDER BY created_at DESC LIMIT ?`, webhookName, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Delivery
	for rows.Next() {
		var dd Delivery
		if err := rows.Scan(&dd.ID, &dd.Webhook, &dd.EventType, &dd.Attempt,
			&dd.StatusCode, &dd.Error, &dd.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, dd)
	}
	return out, rows.Err()
}

func matchesFilter(eventType, filter string) bool {
	if filter == "" {
		return true // no filter = match all
	}
	var types []string
	_ = json.Unmarshal([]byte(filter), &types)
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
		clientPayload := map[string]any{
			"source":  evt.Source,
			"payload": evt.Payload,
		}
		// Story 11.4: surface PR/issue context when the event payload mentions it.
		// GitHub's repository_dispatch forwards client_payload to any workflow
		// listening on the matching event_type.
		for k, v := range extractGitHubContext(evt.Payload) {
			clientPayload[k] = v
		}
		msg := map[string]any{
			"event_type":     evt.Type,
			"client_payload": clientPayload,
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
