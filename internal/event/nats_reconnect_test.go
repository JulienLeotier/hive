package event

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReconnectConnStatusConnected(t *testing.T) {
	rc, err := NewReconnectConn(func() (NATSConn, error) { return &fakeNATS{}, nil })
	require.NoError(t, err)
	assert.Equal(t, "connected", rc.Status())
}

func TestReconnectConnFailsIfInitialDialFails(t *testing.T) {
	_, err := NewReconnectConn(func() (NATSConn, error) { return nil, errors.New("no route") })
	assert.Error(t, err)
}

type flakyNATS struct {
	publishErr atomic.Value
}

func (f *flakyNATS) Publish(subject string, data []byte) error {
	if v := f.publishErr.Load(); v != nil {
		if err, ok := v.(error); ok && err != nil {
			return err
		}
	}
	return nil
}
func (f *flakyNATS) Subscribe(subject string, handler func(subject string, data []byte)) (Unsubscribe, error) {
	return fakeUnsub{}, nil
}
func (f *flakyNATS) Close() {}

func TestReconnectTransitionsToReconnecting(t *testing.T) {
	f := &flakyNATS{}
	f.publishErr.Store(errors.New("boom"))

	dials := int32(0)
	rc, err := NewReconnectConn(func() (NATSConn, error) {
		atomic.AddInt32(&dials, 1)
		if atomic.LoadInt32(&dials) >= 2 {
			return &fakeNATS{}, nil // second call succeeds
		}
		return f, nil
	})
	require.NoError(t, err)

	_ = rc.Publish("x", []byte("hi")) // triggers reconnect goroutine

	// Give reconnect a chance.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if rc.Status() == "connected" && atomic.LoadInt32(&dials) >= 2 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("reconnect never completed (status=%s, dials=%d)", rc.Status(), atomic.LoadInt32(&dials))
}
