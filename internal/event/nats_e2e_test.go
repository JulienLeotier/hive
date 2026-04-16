package event

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runEmbeddedNATS spins up a real nats-server in-process on a random port.
// Returns the URL + a shutdown func. Story 15.2/15.3 end-to-end.
func runEmbeddedNATS(t *testing.T) (string, func()) {
	t.Helper()
	opts := &server.Options{
		Host:     "127.0.0.1",
		Port:     -1, // random
		NoSigs:   true,
		NoLog:    true,
		MaxPayload: 1 << 20,
	}
	s, err := server.NewServer(opts)
	require.NoError(t, err)
	go s.Start()
	if !s.ReadyForConnections(3 * time.Second) {
		t.Fatalf("nats server did not come up")
	}
	return s.ClientURL(), func() { s.Shutdown() }
}

// TestNATSBusEndToEnd verifies the NATS wiring against a real nats-server.
// Story 15.2 AC: events published to and subscribed from NATS subjects,
// ordering maintained per-subject.
func TestNATSBusEndToEnd(t *testing.T) {
	url, stop := runEmbeddedNATS(t)
	defer stop()

	conn, err := NewNATSConnFromURL(url)
	require.NoError(t, err)
	defer conn.Close()

	bus, err := NewNATSBus(conn, DefaultNATSConfig())
	require.NoError(t, err)

	var got atomic.Int32
	bus.Subscribe("task", func(e Event) { got.Add(1) })

	for i := 0; i < 5; i++ {
		_, err := bus.Publish(context.Background(), "task.created", "test", map[string]int{"i": i})
		require.NoError(t, err)
	}

	// Events flow across the real server — give it a moment to propagate.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got.Load() >= 5 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	assert.Equal(t, int32(5), got.Load(), "every publish must round-trip")
}

// TestNATSConnStatusReportsConnected wires the real Conn through the status
// interface so `hive status` can show "connected" without faking.
func TestNATSConnStatusReportsConnected(t *testing.T) {
	url, stop := runEmbeddedNATS(t)
	defer stop()

	conn, err := NewNATSConnFromURL(url)
	require.NoError(t, err)
	defer conn.Close()

	sc, ok := conn.(NATSConnStatus)
	require.True(t, ok, "real NATS conn must satisfy NATSConnStatus")
	assert.Equal(t, "connected", sc.Status())
}

// TestNATSTwoBusesShareEvents confirms Story 22.2 AC: "agent registers on
// node A, replicated to node B via NATS". Two NATSBus instances on the same
// server must see each other's publishes.
func TestNATSTwoBusesShareEvents(t *testing.T) {
	url, stop := runEmbeddedNATS(t)
	defer stop()

	connA, _ := NewNATSConnFromURL(url)
	defer connA.Close()
	connB, _ := NewNATSConnFromURL(url)
	defer connB.Close()

	busA, err := NewNATSBus(connA, DefaultNATSConfig())
	require.NoError(t, err)
	busB, err := NewNATSBus(connB, DefaultNATSConfig())
	require.NoError(t, err)

	var seen atomic.Int32
	busB.Subscribe("agent", func(e Event) { seen.Add(1) })

	// A publishes; B must see it.
	_, err = busA.Publish(context.Background(), "agent.registered", "node-a", map[string]string{"id": "a1"})
	require.NoError(t, err)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if seen.Load() >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	assert.Equal(t, int32(1), seen.Load(), "node B must observe node A's agent.registered event")
}
