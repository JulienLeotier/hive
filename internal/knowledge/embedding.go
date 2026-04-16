package knowledge

import (
	"encoding/binary"
	"hash/fnv"
	"math"
	"strings"
)

// Embedder turns text into a fixed-dimensional vector.
// Story 16.1: interface extracted so external providers (OpenAI, Ollama) can
// be plugged in later without touching the knowledge package.
type Embedder interface {
	Embed(text string) ([]float32, error)
	Dimensions() int
}

// HashingEmbedder is a zero-dependency deterministic baseline embedder.
// Good enough for in-repo tests and for users who don't want to call an
// external model; swap in a proper Embedder in production via WithEmbedder.
type HashingEmbedder struct {
	Dim int
}

// NewHashingEmbedder returns a hashing embedder with the given dimensionality.
func NewHashingEmbedder(dim int) *HashingEmbedder {
	if dim <= 0 {
		dim = 128
	}
	return &HashingEmbedder{Dim: dim}
}

// Dimensions returns the vector size.
func (h *HashingEmbedder) Dimensions() int { return h.Dim }

// Embed hashes each token into a slot and L2-normalises the vector.
// Uses signed feature hashing so collisions partially cancel rather than pile up.
func (h *HashingEmbedder) Embed(text string) ([]float32, error) {
	vec := make([]float32, h.Dim)
	for _, token := range tokenize(text) {
		slot, sign := hashSlot(token, h.Dim)
		vec[slot] += sign
	}
	return l2Normalise(vec), nil
}

func tokenize(text string) []string {
	text = strings.ToLower(text)
	out := make([]string, 0, 8)
	var cur strings.Builder
	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			cur.WriteRune(r)
			continue
		}
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func hashSlot(token string, dim int) (int, float32) {
	h := fnv.New32a()
	_, _ = h.Write([]byte(token))
	sum := h.Sum32()
	slot := int(sum % uint32(dim))
	sign := float32(1)
	if sum&1 == 1 {
		sign = -1
	}
	return slot, sign
}

func l2Normalise(v []float32) []float32 {
	var norm float64
	for _, x := range v {
		norm += float64(x) * float64(x)
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		return v
	}
	for i, x := range v {
		v[i] = float32(float64(x) / norm)
	}
	return v
}

// Encode packs a vector as little-endian float32s for BLOB storage.
func Encode(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, x := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(x))
	}
	return buf
}

// Decode unpacks a float32 BLOB. Returns nil on malformed input.
func Decode(b []byte) []float32 {
	if len(b)%4 != 0 {
		return nil
	}
	out := make([]float32, len(b)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return out
}

// Cosine returns cosine similarity of two vectors; 0 if dimensions mismatch.
func Cosine(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(na) * math.Sqrt(nb)))
}
