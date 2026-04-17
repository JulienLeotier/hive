// Summarizer agent — uses OpenAI's chat completions to condense text.
// Input shape: {text: string, max_words?: int}. Output: {summary: string}.
//
// Set OPENAI_API_KEY before running. The agent refuses to start without
// it so you catch the config problem up front instead of on first invoke.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

const openaiURL = "https://api.openai.com/v1/chat/completions"

type capabilities struct {
	Name       string   `json:"name"`
	TaskTypes  []string `json:"task_types"`
	CostPerRun float64  `json:"cost_per_run,omitempty"`
	Version    string   `json:"version,omitempty"`
}

type task struct {
	ID    string         `json:"id"`
	Type  string         `json:"type"`
	Input map[string]any `json:"input"`
}

type result struct {
	TaskID string `json:"task_id"`
	Status string `json:"status"`
	Output any    `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

var apiKey = os.Getenv("OPENAI_API_KEY")

func main() {
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY is required")
	}

	port := envOr("PORT", "9101")

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"status": "healthy"})
	})

	http.HandleFunc("/declare", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, capabilities{
			Name: "summarizer", TaskTypes: []string{"summarize"},
			CostPerRun: 0.002, Version: "1.0.0",
		})
	})

	http.HandleFunc("/invoke", func(w http.ResponseWriter, r *http.Request) {
		var t task
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			writeJSON(w, result{TaskID: t.ID, Status: "failed", Error: "decode: " + err.Error()})
			return
		}
		text, _ := t.Input["text"].(string)
		if text == "" {
			writeJSON(w, result{TaskID: t.ID, Status: "failed", Error: "missing input.text"})
			return
		}
		maxWords := 80
		if v, ok := t.Input["max_words"].(float64); ok && v > 0 {
			maxWords = int(v)
		}
		summary, err := summarize(r.Context(), text, maxWords)
		if err != nil {
			writeJSON(w, result{TaskID: t.ID, Status: "failed", Error: err.Error()})
			return
		}
		writeJSON(w, result{
			TaskID: t.ID, Status: "completed",
			Output: map[string]any{"summary": summary},
		})
	})

	addr := ":" + port
	log.Printf("summarizer agent listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func summarize(ctx context.Context, text string, maxWords int) (string, error) {
	body := map[string]any{
		"model": "gpt-4o-mini",
		"messages": []map[string]string{
			{"role": "system", "content": fmt.Sprintf(
				"Summarise the user's text in at most %d words. Return only the summary, no preamble.", maxWords)},
			{"role": "user", "content": text},
		},
		"temperature": 0.2,
	}
	buf, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, openaiURL, bytes.NewReader(buf))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("openai %d: %s", resp.StatusCode, string(raw))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("openai parse: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("openai returned no choices")
	}
	return out.Choices[0].Message.Content, nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
