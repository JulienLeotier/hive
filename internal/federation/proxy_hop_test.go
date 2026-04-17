package federation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProxy_HopLimit proves that a task whose Hop count has already reached
// MaxHops is rejected locally before the HTTP call — i.e. two hives that
// mutually advertise the same capability won't loop forever.
func TestProxy_HopLimit(t *testing.T) {
	// Minimal store + fake peer, just enough that clientFor would succeed
	// if we ever reached it.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("proxy should have short-circuited before reaching the peer")
	}))
	defer srv.Close()

	store := setupStore(t)
	require.NoError(t, store.Add(context.Background(),
		&Link{Name: "peer-x", URL: srv.URL, Status: "active"}, "", "", ""))

	p := NewProxy(store)
	_, err := p.Invoke(context.Background(), "peer-x", ProxyRequest{
		TaskID: "t-loop",
		Type:   "whatever",
		Hop:    MaxHops, // already at the cap
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hop limit",
		"error must explicitly mention the hop limit so operators can debug loops")
}

// TestProxy_HopIncrement is a smoke test that a request with Hop=0 is
// accepted (not at the cap). We don't assert on the server side since the
// test server doesn't implement the /tasks/proxied endpoint — just that
// the local gate lets us through.
func TestProxy_HopIncrement(t *testing.T) {
	store := setupStore(t)
	require.NoError(t, store.Add(context.Background(),
		&Link{Name: "peer-y", URL: "http://127.0.0.1:1", Status: "active"}, "", "", ""))

	p := NewProxy(store)
	_, err := p.Invoke(context.Background(), "peer-y", ProxyRequest{
		TaskID: "t-ok",
		Hop:    0,
	})
	// We expect a connection error (no listener on :1), NOT the hop-limit
	// error — proving the hop guard passed.
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "hop limit")
}
