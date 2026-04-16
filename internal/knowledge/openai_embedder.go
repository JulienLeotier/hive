package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// OpenAIEmbedder calls the OpenAI embeddings API. Story 16.2.
// When the API is unavailable, the configured fallback embedder is used so
// knowledge features stay functional (AC: "falls back to local embeddings if
// the API is unavailable").
type OpenAIEmbedder struct {
	APIKey   string
	Model    string // e.g. "text-embedding-3-small"
	BaseURL  string // defaults to https://api.openai.com/v1
	Fallback Embedder
	Dim      int
	client   *http.Client
}

// NewOpenAIEmbedder builds an OpenAI embedder. A fallback is required so the
// "fall back to local" behaviour is explicit at construction time.
func NewOpenAIEmbedder(apiKey, model string, fallback Embedder) *OpenAIEmbedder {
	if model == "" {
		model = "text-embedding-3-small"
	}
	return &OpenAIEmbedder{
		APIKey:   apiKey,
		Model:    model,
		BaseURL:  "https://api.openai.com/v1",
		Fallback: fallback,
		Dim:      1536, // text-embedding-3-small native dimension
		client:   &http.Client{Timeout: 20 * time.Second},
	}
}

// Dimensions returns the embedder's vector size.
func (o *OpenAIEmbedder) Dimensions() int { return o.Dim }

// Embed hits /embeddings and returns the float32 vector. On any error, falls
// back to the local embedder — this guarantees knowledge operations never
// break because of a transient network blip.
func (o *OpenAIEmbedder) Embed(text string) ([]float32, error) {
	if o.APIKey == "" {
		return o.fallback(text)
	}

	body, _ := json.Marshal(map[string]any{"model": o.Model, "input": text})
	req, err := http.NewRequestWithContext(context.Background(), "POST", o.BaseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return o.fallback(text)
	}
	req.Header.Set("Authorization", "Bearer "+o.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		slog.Warn("openai embed failed, using fallback", "error", err)
		return o.fallback(text)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<10))
		slog.Warn("openai embed returned non-200, using fallback", "status", resp.StatusCode, "body", string(data))
		return o.fallback(text)
	}

	var parsed struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil || len(parsed.Data) == 0 {
		return o.fallback(text)
	}
	vec := parsed.Data[0].Embedding
	if len(vec) != o.Dim {
		o.Dim = len(vec)
	}
	return vec, nil
}

func (o *OpenAIEmbedder) fallback(text string) ([]float32, error) {
	if o.Fallback == nil {
		return nil, fmt.Errorf("openai unavailable and no fallback configured")
	}
	return o.Fallback.Embed(text)
}
