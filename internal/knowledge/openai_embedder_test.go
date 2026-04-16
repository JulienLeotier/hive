package knowledge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIEmbedderReturnsVector(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"embedding": []float32{0.1, 0.2, 0.3}}},
		})
	}))
	defer srv.Close()

	fallback := NewHashingEmbedder(3)
	emb := NewOpenAIEmbedder("test-key", "text-embedding-3-small", fallback)
	emb.BaseURL = srv.URL
	emb.Dim = 3

	vec, err := emb.Embed("hello")
	require.NoError(t, err)
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, vec)
}

func TestOpenAIEmbedderFallbackOnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	fallback := NewHashingEmbedder(128)
	emb := NewOpenAIEmbedder("test-key", "", fallback)
	emb.BaseURL = srv.URL

	vec, err := emb.Embed("fallback path")
	require.NoError(t, err)
	assert.Len(t, vec, 128, "should have fallen back to the local hashing embedder")
}

func TestOpenAIEmbedderFallbackWhenNoKey(t *testing.T) {
	fallback := NewHashingEmbedder(64)
	emb := NewOpenAIEmbedder("", "", fallback)
	vec, err := emb.Embed("no key")
	require.NoError(t, err)
	assert.Len(t, vec, 64)
}
