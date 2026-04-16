package event

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeNATS is a tiny in-memory NATSConn double for tests.
type fakeNATS struct {
	mu   sync.Mutex
	subs []subscription
}

type subscription struct {
	subject string
	handler func(subject string, data []byte)
}

type fakeUnsub struct{}

func (fakeUnsub) Unsubscribe() error { return nil }

func (f *fakeNATS) Publish(subject string, data []byte) error {
	f.mu.Lock()
	subs := append([]subscription{}, f.subs...)
	f.mu.Unlock()
	for _, s := range subs {
		if match(s.subject, subject) {
			s.handler(subject, data)
		}
	}
	return nil
}

func (f *fakeNATS) Subscribe(subject string, handler func(subject string, data []byte)) (Unsubscribe, error) {
	f.mu.Lock()
	f.subs = append(f.subs, subscription{subject: subject, handler: handler})
	f.mu.Unlock()
	return fakeUnsub{}, nil
}

func (f *fakeNATS) Close() {}

// match honours the one wildcard we need — subject suffix ">"
func match(pattern, subject string) bool {
	if len(pattern) > 0 && pattern[len(pattern)-1] == '>' {
		return len(subject) >= len(pattern)-1 && subject[:len(pattern)-1] == pattern[:len(pattern)-1]
	}
	return pattern == subject
}

func TestNATSBusPublishDeliversToSubscribers(t *testing.T) {
	bus, err := NewNATSBus(&fakeNATS{}, DefaultNATSConfig())
	require.NoError(t, err)

	var got []Event
	var mu sync.Mutex
	bus.Subscribe("task", func(e Event) {
		mu.Lock()
		got = append(got, e)
		mu.Unlock()
	})

	_, err = bus.Publish(context.Background(), "task.created", "test", map[string]string{"id": "t1"})
	require.NoError(t, err)
	_, err = bus.Publish(context.Background(), "agent.registered", "test", nil)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, got, 1)
	assert.Equal(t, "task.created", got[0].Type)
}

func TestNATSBusQueryReturnsHistory(t *testing.T) {
	bus, err := NewNATSBus(&fakeNATS{}, DefaultNATSConfig())
	require.NoError(t, err)
	ctx := context.Background()

	_, _ = bus.Publish(ctx, "task.created", "a", nil)
	_, _ = bus.Publish(ctx, "task.completed", "a", nil)
	_, _ = bus.Publish(ctx, "agent.registered", "a", nil)

	results, err := bus.Query(ctx, QueryOpts{Type: "task"})
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestNATSBusHistoryIsBounded(t *testing.T) {
	bus, err := NewNATSBus(&fakeNATS{}, NATSConfig{Subject: "h", MaxHistory: 3})
	require.NoError(t, err)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		_, _ = bus.Publish(ctx, "x", "s", nil)
	}

	results, err := bus.Query(ctx, QueryOpts{})
	require.NoError(t, err)
	assert.Len(t, results, 3)
}
