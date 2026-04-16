package knowledge

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashingEmbedderDeterministic(t *testing.T) {
	e := NewHashingEmbedder(64)
	v1, _ := e.Embed("the quick brown fox")
	v2, _ := e.Embed("the quick brown fox")
	assert.Equal(t, v1, v2)
}

func TestHashingEmbedderSimilarityIntuition(t *testing.T) {
	e := NewHashingEmbedder(256)
	a, _ := e.Embed("null pointer check in go")
	b, _ := e.Embed("null pointer check for go code")
	c, _ := e.Embed("python pandas dataframe merge")

	sim := Cosine(a, b)
	dist := Cosine(a, c)
	assert.Greater(t, sim, dist, "similar texts should score higher than unrelated ones")
}

func TestEncodeDecodeRoundtrip(t *testing.T) {
	v := []float32{0.1, -0.5, 3.14, 0, 2.71}
	decoded := Decode(Encode(v))
	assert.Equal(t, v, decoded)
}

func TestVectorSearchRanksBySimilarity(t *testing.T) {
	s := setupStore(t).WithEmbedder(NewHashingEmbedder(256))
	ctx := context.Background()

	require.NoError(t, s.Record(ctx, "code-review", "check null pointers in go", "success", `{"lang":"go"}`))
	require.NoError(t, s.Record(ctx, "code-review", "verify error wrapping with fmt.Errorf", "success", `{"lang":"go"}`))
	require.NoError(t, s.Record(ctx, "data-science", "merge pandas dataframes", "success", `{"lang":"python"}`))

	results, err := s.VectorSearch(ctx, "null pointer check in go", 2)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(results), 1)
	assert.Contains(t, results[0].Approach, "null pointers")
}

func TestVectorSearchRequiresEmbedder(t *testing.T) {
	s := setupStore(t) // no embedder
	_, err := s.VectorSearch(context.Background(), "anything", 5)
	assert.Error(t, err)
}
