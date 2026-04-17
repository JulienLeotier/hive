package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAIAdapter connects to an OpenAI Assistant via the Assistants API.
type OpenAIAdapter struct {
	AssistantID string
	APIKey      string
	Name        string
	client      *http.Client
}

// NewOpenAIAdapter creates an adapter for an OpenAI Assistant.
func NewOpenAIAdapter(assistantID, apiKey, name string) *OpenAIAdapter {
	return &OpenAIAdapter{
		AssistantID: assistantID,
		APIKey:      apiKey,
		Name:        name,
		client:      &http.Client{Timeout: 120 * time.Second},
	}
}

func (a *OpenAIAdapter) Declare(ctx context.Context) (AgentCapabilities, error) {
	return AgentCapabilities{
		Name:      a.Name,
		TaskTypes: []string{"openai-assistant"},
	}, nil
}

func (a *OpenAIAdapter) Invoke(ctx context.Context, task Task) (TaskResult, error) {
	// 1. Create a thread
	threadID, err := a.createThread(ctx)
	if err != nil {
		return TaskResult{TaskID: task.ID, Status: "failed", Error: err.Error()}, nil
	}

	// 2. Add message
	inputStr := fmt.Sprintf("%v", task.Input)
	if err := a.addMessage(ctx, threadID, inputStr); err != nil {
		return TaskResult{TaskID: task.ID, Status: "failed", Error: err.Error()}, nil
	}

	// 3. Create run and poll
	result, err := a.createRunAndPoll(ctx, threadID)
	if err != nil {
		return TaskResult{TaskID: task.ID, Status: "failed", Error: err.Error()}, nil
	}

	return TaskResult{TaskID: task.ID, Status: "completed", Output: result}, nil
}

func (a *OpenAIAdapter) Health(ctx context.Context) (HealthStatus, error) {
	if a.APIKey == "" {
		return HealthStatus{Status: "unavailable", Message: "no API key configured"}, nil
	}
	return HealthStatus{Status: "healthy"}, nil
}

func (a *OpenAIAdapter) Checkpoint(ctx context.Context) (Checkpoint, error) { return Checkpoint{}, nil }
func (a *OpenAIAdapter) Resume(ctx context.Context, cp Checkpoint) error    { return nil }

func (a *OpenAIAdapter) apiCall(ctx context.Context, method, path string, body any) ([]byte, error) {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, "https://api.openai.com/v1"+path, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OpenAI-Beta", "assistants=v2")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("OpenAI API %d: %s", resp.StatusCode, string(data))
	}
	return data, nil
}

func (a *OpenAIAdapter) createThread(ctx context.Context) (string, error) {
	data, err := a.apiCall(ctx, "POST", "/threads", map[string]any{})
	if err != nil {
		return "", fmt.Errorf("creating thread: %w", err)
	}
	var result struct{ ID string `json:"id"` }
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("parsing thread response: %w", err)
	}
	if result.ID == "" {
		return "", fmt.Errorf("empty thread ID in response")
	}
	return result.ID, nil
}

func (a *OpenAIAdapter) addMessage(ctx context.Context, threadID, content string) error {
	_, err := a.apiCall(ctx, "POST", "/threads/"+threadID+"/messages", map[string]any{
		"role": "user", "content": content,
	})
	return err
}

func (a *OpenAIAdapter) createRunAndPoll(ctx context.Context, threadID string) (string, error) {
	data, err := a.apiCall(ctx, "POST", "/threads/"+threadID+"/runs", map[string]any{
		"assistant_id": a.AssistantID,
	})
	if err != nil {
		return "", err
	}
	var run struct{ ID string `json:"id"` }
	if err := json.Unmarshal(data, &run); err != nil {
		return "", fmt.Errorf("parsing run response: %w", err)
	}

	// Poll for completion (max 60 attempts, 2s intervals). Using a single
	// Ticker instead of time.After per iteration so we allocate one timer
	// for the whole loop and the Stop() on ctx cancel is explicit.
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for i := 0; i < 60; i++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
		}
		data, err := a.apiCall(ctx, "GET", "/threads/"+threadID+"/runs/"+run.ID, nil)
		if err != nil {
			return "", err
		}
		var status struct{ Status string `json:"status"` }
		if err := json.Unmarshal(data, &status); err != nil {
			return "", fmt.Errorf("parsing run status: %w", err)
		}
		if status.Status == "completed" {
			return a.getLastMessage(ctx, threadID)
		}
		if status.Status == "failed" || status.Status == "cancelled" {
			return "", fmt.Errorf("run %s: %s", run.ID, status.Status)
		}
	}
	return "", fmt.Errorf("run %s timed out", run.ID)
}

func (a *OpenAIAdapter) getLastMessage(ctx context.Context, threadID string) (string, error) {
	data, err := a.apiCall(ctx, "GET", "/threads/"+threadID+"/messages?limit=1&order=desc", nil)
	if err != nil {
		return "", err
	}
	var msgs struct {
		Data []struct {
			Content []struct {
				Text struct{ Value string `json:"value"` } `json:"text"`
			} `json:"content"`
		} `json:"data"`
	}
	json.Unmarshal(data, &msgs)
	if len(msgs.Data) > 0 && len(msgs.Data[0].Content) > 0 {
		return msgs.Data[0].Content[0].Text.Value, nil
	}
	return "", nil
}

var _ Adapter = (*OpenAIAdapter)(nil)
