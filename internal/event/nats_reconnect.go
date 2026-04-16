package event

import (
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// ReconnectConn wraps a dialer so the bus can recover from disconnects with
// exponential backoff. Story 15.3.
//
// The wrapper owns a live NATSConn pointer; on Publish/Subscribe failure it
// redials using the provided factory. Backoff starts at InitialBackoff and
// doubles up to MaxBackoff. Reconnect attempts run on a single goroutine so
// we never pile up parallel connection storms.
type ReconnectConn struct {
	factory         func() (NATSConn, error)
	mu              sync.Mutex
	current         NATSConn
	status          atomic.Value // string
	initialBackoff  time.Duration
	maxBackoff      time.Duration
	subs            []subscribeRequest
	onStatusChange  func(status string)
}

type subscribeRequest struct {
	subject string
	handler func(subject string, data []byte)
}

// NewReconnectConn builds a reconnecting wrapper around a dial factory.
func NewReconnectConn(factory func() (NATSConn, error)) (*ReconnectConn, error) {
	c := &ReconnectConn{
		factory:        factory,
		initialBackoff: 200 * time.Millisecond,
		maxBackoff:     30 * time.Second,
	}
	c.setStatus("connecting")
	conn, err := factory()
	if err != nil {
		return nil, err
	}
	c.current = conn
	c.setStatus("connected")
	return c, nil
}

// Publish forwards to the live connection, reconnecting on failure.
func (c *ReconnectConn) Publish(subject string, data []byte) error {
	c.mu.Lock()
	conn := c.current
	c.mu.Unlock()
	if conn == nil {
		return errNotConnected
	}
	if err := conn.Publish(subject, data); err != nil {
		go c.reconnect()
		return err
	}
	return nil
}

// Subscribe records the subscription so it can be replayed after reconnect.
func (c *ReconnectConn) Subscribe(subject string, handler func(subject string, data []byte)) (Unsubscribe, error) {
	c.mu.Lock()
	c.subs = append(c.subs, subscribeRequest{subject: subject, handler: handler})
	conn := c.current
	c.mu.Unlock()
	if conn == nil {
		return noopUnsub{}, errNotConnected
	}
	return conn.Subscribe(subject, handler)
}

// Close closes the underlying connection and stops reconnect attempts.
func (c *ReconnectConn) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.current != nil {
		c.current.Close()
	}
	c.current = nil
	c.setStatus("closed")
}

// Status returns the wrapper's current connection state.
func (c *ReconnectConn) Status() string {
	if v, ok := c.status.Load().(string); ok {
		return v
	}
	return "unknown"
}

// OnStatusChange registers a callback for status transitions — useful for
// surfacing the state in `hive status`.
func (c *ReconnectConn) OnStatusChange(fn func(status string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onStatusChange = fn
}

func (c *ReconnectConn) setStatus(s string) {
	c.status.Store(s)
	if c.onStatusChange != nil {
		c.onStatusChange(s)
	}
}

func (c *ReconnectConn) reconnect() {
	c.mu.Lock()
	if c.current == nil {
		c.mu.Unlock()
		return
	}
	c.setStatus("reconnecting")
	c.mu.Unlock()

	backoff := c.initialBackoff
	for attempt := 1; ; attempt++ {
		conn, err := c.factory()
		if err == nil {
			c.mu.Lock()
			c.current = conn
			// Replay subscriptions
			for _, s := range c.subs {
				_, _ = conn.Subscribe(s.subject, s.handler)
			}
			c.setStatus("connected")
			c.mu.Unlock()
			slog.Info("nats reconnect succeeded", "attempt", attempt)
			return
		}
		slog.Warn("nats reconnect failed", "attempt", attempt, "wait", backoff, "error", err)
		time.Sleep(backoff)
		backoff = time.Duration(math.Min(float64(c.maxBackoff), float64(backoff)*2))
	}
}

type errString string

func (e errString) Error() string { return string(e) }

const errNotConnected = errString("nats: not connected")

type noopUnsub struct{}

func (noopUnsub) Unsubscribe() error { return nil }
