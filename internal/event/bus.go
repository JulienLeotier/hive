package event

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Bus is an in-process event bus backed by SQLite for persistence.
// Events are persisted before delivery to subscribers.
type Bus struct {
	db          *sql.DB
	mu          sync.RWMutex
	subscribers map[string][]Subscriber // prefix → callbacks
}

// NewBus creates an event bus backed by the given database.
func NewBus(db *sql.DB) *Bus {
	return &Bus{
		db:          db,
		subscribers: make(map[string][]Subscriber),
	}
}

// Publish persists an event to SQLite then delivers it to matching subscribers.
func (b *Bus) Publish(ctx context.Context, eventType, source string, payload any) (Event, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("marshaling event payload: %w", err)
	}

	// Persist first (guaranteed delivery)
	result, err := b.db.ExecContext(ctx,
		`INSERT INTO events (type, source, payload) VALUES (?, ?, ?)`,
		eventType, source, string(payloadJSON),
	)
	if err != nil {
		return Event{}, fmt.Errorf("persisting event: %w", err)
	}

	id, _ := result.LastInsertId()
	evt := Event{
		ID:        id,
		Type:      eventType,
		Source:    source,
		Payload:   string(payloadJSON),
		CreatedAt: time.Now(),
	}

	// Deliver to subscribers (async, non-blocking)
	b.deliver(evt)

	slog.Debug("event published", "id", id, "type", eventType, "source", source)
	return evt, nil
}

// Subscribe registers a callback for events matching the given type prefix.
// Example: "task.*" matches "task.created", "task.completed", etc.
func (b *Bus) Subscribe(prefix string, fn Subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers[prefix] = append(b.subscribers[prefix], fn)
}

// Query returns events matching the filter criteria, ordered by ID (chronological).
func (b *Bus) Query(ctx context.Context, opts QueryOpts) ([]Event, error) {
	query := `SELECT id, type, source, payload, created_at FROM events WHERE 1=1`
	var args []any

	if opts.Type != "" {
		query += ` AND type LIKE ?`
		args = append(args, opts.Type+"%")
	}
	if opts.Source != "" {
		query += ` AND source = ?`
		args = append(args, opts.Source)
	}
	if !opts.Since.IsZero() {
		query += ` AND created_at >= ?`
		args = append(args, opts.Since.Format("2006-01-02 15:04:05"))
	}

	query += ` ORDER BY id ASC`

	if opts.Limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, opts.Limit)
	}

	rows, err := b.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var created string
		if err := rows.Scan(&e.ID, &e.Type, &e.Source, &e.Payload, &created); err != nil {
			return nil, fmt.Errorf("scanning event: %w", err)
		}
		e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		events = append(events, e)
	}
	return events, rows.Err()
}

// QueryOpts filters for event queries.
type QueryOpts struct {
	Type   string    // prefix match (e.g., "task" matches "task.created")
	Source string    // exact match
	Since  time.Time // events after this time
	Limit  int       // max results (0 = unlimited)
}

func (b *Bus) deliver(evt Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for prefix, subs := range b.subscribers {
		if strings.HasPrefix(evt.Type, prefix) || prefix == "*" {
			for _, fn := range subs {
				fn(evt) // synchronous delivery for ordering guarantee
			}
		}
	}
}
