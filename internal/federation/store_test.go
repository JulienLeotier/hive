package federation

import (
	"context"
	"testing"

	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupStore(t *testing.T) *Store {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	return NewStore(st.DB)
}

func TestAddAndListLinks(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	require.NoError(t, s.Add(ctx, &Link{
		Name: "peer-1", URL: "https://peer.example.com", Status: "active",
		SharedCaps: []string{"code-review", "translate"},
	}, "", "", ""))

	links, err := s.List(ctx)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, "peer-1", links[0].Name)
	assert.ElementsMatch(t, []string{"code-review", "translate"}, links[0].SharedCaps)
}

func TestUpsertLink(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()

	require.NoError(t, s.Add(ctx, &Link{Name: "peer", URL: "https://a", Status: "active"}, "", "", ""))
	require.NoError(t, s.Add(ctx, &Link{Name: "peer", URL: "https://b", Status: "degraded"}, "", "", ""))

	links, _ := s.List(ctx)
	require.Len(t, links, 1)
	assert.Equal(t, "https://b", links[0].URL)
	assert.Equal(t, "degraded", links[0].Status)
}

func TestTLSConfigNoMaterialReturnsNil(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	require.NoError(t, s.Add(ctx, &Link{Name: "peer", URL: "https://a"}, "", "", ""))

	tlsCfg, err := s.TLSConfigFor(ctx, "peer")
	require.NoError(t, err)
	assert.Nil(t, tlsCfg, "no mTLS material → no TLS config")
}

func TestHydrateFromStore(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	require.NoError(t, s.Add(ctx, &Link{Name: "peer", URL: "https://a", Status: "active"}, "", "", ""))

	m := NewManager()
	require.NoError(t, s.Hydrate(ctx, m))
	assert.Len(t, m.ListLinks(), 1)
}
