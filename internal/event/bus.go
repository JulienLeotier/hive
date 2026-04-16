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

// EventBus is the abstract contract every backend (SQLite, NATS, …) implements.
// Extracted in Story 15.1 so a NATS backend can replace the SQLite one without
// rippling changes through the rest of the codebase.
type EventBus interface {
	Publish(ctx context.Context, eventType, source string, payload any) (Event, error)
	Subscribe(prefix string, fn Subscriber)
	Query(ctx context.Context, opts QueryOpts) ([]Event, error)
}

// Bus is an in-process event bus backed by SQLite for persistence.
// Events are persisted before delivery to subscribers.
type Bus struct {
	db          *sql.DB
	mu          sync.RWMutex
	subscribers map[string][]Subscriber // prefix → callbacks
}

// Compile-time check that Bus satisfies the interface.
var _ EventBus = (*Bus)(nil)

// NewBus creates an event bus backed by the given database.
func NewBus(db *sql.DB) *Bus {
	return &Bus{
		db:          db,
		subscribers: make(map[string][]Subscriber),
	}
}

// DB returns the underlying database handle so callers that already share this
// bus don't need to plumb the *sql.DB separately.
func (b *Bus) DB() *sql.DB { return b.db }

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
		// Escape LIKE wildcards in user input to prevent unintended broadening
		escaped := strings.NewReplacer("%", "\\%", "_", "\\_").Replace(opts.Type)
		query += ` AND type LIKE ? ESCAPE '\'`
		args = append(args, escaped+"%")
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
				b.safeCall(fn, evt)
			}
		}
	}
}

func (b *Bus) safeCall(fn Subscriber, evt Event) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("subscriber panic recovered", "event", evt.Type, "panic", r)
		}
	}()
	fn(evt)
}
