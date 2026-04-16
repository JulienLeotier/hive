package federation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProxyInvokeRoundTrips exercises Story 19.3: a local task addressed to
// a federated peer is proxied over HTTP and the peer's response streams back.
func TestProxyInvokeRoundTrips(t *testing.T) {
	peer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ProxyRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		_ = json.NewEncoder(w).Encode(ProxyResponse{
			TaskID: req.TaskID,
			Status: "completed",
			Output: map[string]string{"hello": "from peer"},
		})
	}))
	defer peer.Close()

	st := setupStore(t)
	ctx := context.Background()
	require.NoError(t, st.Add(ctx, &Link{Name: "peer-1", URL: peer.URL, Status: "active"}, "", "", ""))

	p := NewProxy(st)
	resp, err := p.Invoke(ctx, "peer-1", ProxyRequest{
		TaskID: "t-42", Type: "code-review", Input: map[string]any{"pr": 7},
	})
	require.NoError(t, err)
	assert.Equal(t, "t-42", resp.TaskID)
	assert.Equal(t, "completed", resp.Status)
}

func TestProxyUnknownPeerErrors(t *testing.T) {
	st := setupStore(t)
	p := NewProxy(st)
	_, err := p.Invoke(context.Background(), "nobody", ProxyRequest{TaskID: "x"})
	assert.Error(t, err)
}

func TestResolverFindsMatchingCapability(t *testing.T) {
	st := setupStore(t)
	ctx := context.Background()
	require.NoError(t, st.Add(ctx, &Link{
		Name: "peer", URL: "https://peer", Status: "active",
		SharedCaps: []string{"code-review"},
	}, "", "", ""))

	resolver, _ := NewResolver(ctx, st)
	name, url, ok := resolver(ctx, "code-review")
	assert.True(t, ok)
	assert.Equal(t, "peer", name)
	assert.Equal(t, "https://peer", url)

	_, _, ok = resolver(ctx, "other-capability")
	assert.False(t, ok)
}
