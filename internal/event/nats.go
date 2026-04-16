package event

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// NATSConn is the minimal subset of a NATS client the backend depends on.
// Satisfied by *nats.Conn; abstracted here so we can exercise the backend
// without dragging the NATS server binary into tests. Story 15.3.
type NATSConn interface {
	Publish(subject string, data []byte) error
	Subscribe(subject string, handler func(subject string, data []byte)) (Unsubscribe, error)
	Close()
}

// Unsubscribe closes a subscription.
type Unsubscribe interface {
	Unsubscribe() error
}

// NATSBus satisfies EventBus by pushing events to a NATS subject tree.
// A local ring buffer keeps the last N events so Query() still works even
// though NATS itself is not a historical store (JetStream is — slot in later).
type NATSBus struct {
	conn    NATSConn
	subject string

	mu          sync.RWMutex
	subscribers map[string][]Subscriber
	history     []Event
	maxHistory  int

	nextID atomic.Int64
}

// NATSConfig configures the NATS backend.
type NATSConfig struct {
	// Subject prefix; events are published to Subject+"."+eventType.
	Subject string
	// MaxHistory caps the in-memory tail used for Query().
	MaxHistory int
}

// DefaultNATSConfig returns sane defaults.
func DefaultNATSConfig() NATSConfig {
	return NATSConfig{Subject: "hive.events", MaxHistory: 1000}
}

// NewNATSBus wraps a NATSConn as an EventBus.
func NewNATSBus(conn NATSConn, cfg NATSConfig) (*NATSBus, error) {
	if cfg.Subject == "" {
		cfg.Subject = "hive.events"
	}
	if cfg.MaxHistory <= 0 {
		cfg.MaxHistory = 1000
	}
	b := &NATSBus{
		conn:        conn,
		subject:     cfg.Subject,
		subscribers: map[string][]Subscriber{},
		maxHistory:  cfg.MaxHistory,
	}

	// Subscribe to all events on our subject tree; fan out to local subscribers.
	if _, err := conn.Subscribe(cfg.Subject+".>", b.handleInbound); err != nil {
		return nil, fmt.Errorf("subscribing to NATS subject: %w", err)
	}
	return b, nil
}

var _ EventBus = (*NATSBus)(nil)

// Publish serialises an event and ships it over NATS.
func (b *NATSBus) Publish(ctx context.Context, eventType, source string, payload any) (Event, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("marshaling payload: %w", err)
	}

	evt := Event{
		ID:        b.nextID.Add(1),
		Type:      eventType,
		Source:    source,
		Payload:   string(payloadJSON),
		CreatedAt: time.Now(),
	}
	data, err := json.Marshal(evt)
	if err != nil {
		return Event{}, fmt.Errorf("marshaling event envelope: %w", err)
	}

	subject := b.subject + "." + eventType
	if err := b.conn.Publish(subject, data); err != nil {
		return Event{}, fmt.Errorf("publishing to NATS: %w", err)
	}
	return evt, nil
}

// Subscribe registers a prefix-based callback.
func (b *NATSBus) Subscribe(prefix string, fn Subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers[prefix] = append(b.subscribers[prefix], fn)
}

// Query returns the tail of recently seen events matching the filter.
// For durable history use JetStream (out of scope for v0.3).
func (b *NATSBus) Query(ctx context.Context, opts QueryOpts) ([]Event, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var out []Event
	for _, e := range b.history {
		if opts.Type != "" && !strings.HasPrefix(e.Type, opts.Type) {
			continue
		}
		if opts.Source != "" && e.Source != opts.Source {
			continue
		}
		if !opts.Since.IsZero() && e.CreatedAt.Before(opts.Since) {
			continue
		}
		out = append(out, e)
		if opts.Limit > 0 && len(out) >= opts.Limit {
			break
		}
	}
	return out, nil
}

// Close releases the underlying connection.
func (b *NATSBus) Close() {
	if b.conn != nil {
		b.conn.Close()
	}
}

func (b *NATSBus) handleInbound(_ string, data []byte) {
	var evt Event
	if err := json.Unmarshal(data, &evt); err != nil {
		slog.Error("dropping malformed NATS event", "error", err)
		return
	}

	b.mu.Lock()
	b.history = append(b.history, evt)
	if len(b.history) > b.maxHistory {
		b.history = b.history[len(b.history)-b.maxHistory:]
	}
	subs := b.subscribers
	b.mu.Unlock()

	for prefix, fns := range subs {
		if prefix == "*" || strings.HasPrefix(evt.Type, prefix) {
			for _, fn := range fns {
				func(fn Subscriber) {
					defer func() {
						if r := recover(); r != nil {
							slog.Error("subscriber panic", "type", evt.Type, "recover", r)
						}
					}()
					fn(evt)
				}(fn)
			}
		}
	}
}
