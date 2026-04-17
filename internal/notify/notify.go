// Package notify posts Slack webhooks (and other transports later) on
// interesting BMAD events. Intentionally minimal : subscribe to the
// bus, serialize the event to a short Slack message, POST it. No
// queuing, no retries — notifications are advisory.
//
// Configured via env :
//
//	HIVE_SLACK_WEBHOOK=https://hooks.slack.com/services/...
//
// If the env var is absent, Attach() is a no-op so the build
// continues to work locally without any side effect.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/JulienLeotier/hive/internal/event"
)

// Events we forward to the webhook. Kept short so a Slack channel
// doesn't turn into noise — only the "project-level big deal" ones.
var relevantEvents = map[string]string{
	"project.shipped":          "🚀 Project shipped",
	"project.architect_failed": "❌ BMAD architect failed",
	"project.iteration_failed": "❌ BMAD iteration failed",
	"project.cost_cap_warning": "⚠️ Cost cap 80% reached",
	"project.cost_cap_reached": "💸 Cost cap reached — run cancelled",
}

// Attach subscribes a notifier to the bus if HIVE_SLACK_WEBHOOK is set.
// Returns true when actually attached, so callers can log.
func Attach(bus *event.Bus) bool {
	webhook := os.Getenv("HIVE_SLACK_WEBHOOK")
	if webhook == "" || bus == nil {
		return false
	}
	bus.Subscribe("*", func(e event.Event) {
		title, ok := relevantEvents[e.Type]
		if !ok {
			return
		}
		go postSlack(webhook, title, e)
	})
	slog.Info("notify: slack webhook attached")
	return true
}

func postSlack(webhook, title string, e event.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	payload := map[string]string{
		"text": fmt.Sprintf("*%s*\n```%s```", title, summarize(e)),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		slog.Warn("notify: marshal failed", "error", err)
		return
	}
	req, err := http.NewRequestWithContext(ctx, "POST", webhook, bytes.NewReader(body))
	if err != nil {
		slog.Warn("notify: build request failed", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Warn("notify: post failed", "error", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		slog.Warn("notify: webhook rejected", "status", resp.StatusCode)
	}
}

func summarize(e event.Event) string {
	// Payload is already a JSON string; try to pretty-print it.
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, []byte(e.Payload), "", "  "); err == nil {
		if pretty.Len() > 1500 {
			pretty.Truncate(1500)
		}
		return pretty.String()
	}
	s := e.Payload
	if len(s) > 1500 {
		s = s[:1500]
	}
	if s == "" {
		return e.Type
	}
	return s
}
