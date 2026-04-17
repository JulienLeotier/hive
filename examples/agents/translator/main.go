// Translator agent — delegates to OpenAI's chat completions to translate
// text between languages. Input: {text: string, target_lang: string,
// source_lang?: string}. Output: {translation: string}.
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
	port := envOr("PORT", "9102")

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"status": "healthy"})
	})
	http.HandleFunc("/declare", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, capabilities{
			Name: "translator", TaskTypes: []string{"translate"},
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
		target, _ := t.Input["target_lang"].(string)
		if text == "" || target == "" {
			writeJSON(w, result{TaskID: t.ID, Status: "failed",
				Error: "input.text and input.target_lang are required"})
			return
		}
		source, _ := t.Input["source_lang"].(string)
		translation, err := translate(r.Context(), text, source, target)
		if err != nil {
			writeJSON(w, result{TaskID: t.ID, Status: "failed", Error: err.Error()})
			return
		}
		writeJSON(w, result{
			TaskID: t.ID, Status: "completed",
			Output: map[string]any{"translation": translation, "target_lang": target},
		})
	})

	addr := ":" + port
	log.Printf("translator agent listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func translate(ctx context.Context, text, source, target string) (string, error) {
	sys := fmt.Sprintf("Translate the user's text into %s. Return only the translation, no preamble.", target)
	if source != "" {
		sys = fmt.Sprintf("Translate the user's text from %s into %s. Return only the translation, no preamble.", source, target)
	}
	body := map[string]any{
		"model": "gpt-4o-mini",
		"messages": []map[string]string{
			{"role": "system", "content": sys},
			{"role": "user", "content": text},
		},
		"temperature": 0.0,
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
