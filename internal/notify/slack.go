package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/JulienLeotier/hive/internal/event"
)

// SlackConfig captures the minimum needed to POST to a Slack Incoming Webhook.
// The webhook URL already encodes the target channel, so no channel/username
// fields are required here. Operators who want multiple channels should
// configure multiple generic webhooks via the dashboard instead.
type SlackConfig struct {
	WebhookURL  string
	TimeoutSecs int
}

// Enabled reports whether the config is sufficient to attempt delivery.
func (c SlackConfig) Enabled() bool { return c.WebhookURL != "" }

// SlackNotifier mirrors the email notifier: subscribes to ops-shaped events,
// debounces per type, posts a formatted message to the Slack webhook URL.
// Separate from the generic webhook.Dispatcher because that one is for
// user-configured integrations — this one is specifically for the
// hive.yaml:notifications.slack ops channel, so it doesn't require an
// operator to click through the /webhooks form to get alerts wired.
type SlackNotifier struct {
	cfg      SlackConfig
	debounce time.Duration
	now      func() time.Time
	sendFunc func(ctx context.Context, url, text string) error

	m      slackMu
	events []string // event types we listen to (set at Attach time)
}

// slackMu is a named type so tests can reach into lastSent without exporting.
type slackMu struct {
	lastSent map[string]time.Time
}

// NewSlackNotifier builds a Slack notifier. Delivery is a no-op when the
// webhook URL is empty, matching the email-side contract.
func NewSlackNotifier(cfg SlackConfig) *SlackNotifier {
	return &SlackNotifier{
		cfg:      cfg,
		debounce: 60 * time.Second,
		now:      time.Now,
		sendFunc: postSlack,
		m:        slackMu{lastSent: map[string]time.Time{}},
	}
}

// WithDebounce overrides the default 60s suppression window.
func (n *SlackNotifier) WithDebounce(d time.Duration) *SlackNotifier {
	n.debounce = d
	return n
}

// Enabled reports whether the notifier will actually send.
func (n *SlackNotifier) Enabled() bool { return n.cfg.Enabled() }

// Attach subscribes to the ops event types on the bus.
func (n *SlackNotifier) Attach(bus *event.Bus) {
	if !n.cfg.Enabled() {
		slog.Info("notify: slack channel disabled — no webhook URL")
		return
	}
	types := []string{
		event.TaskFailed,
		event.CostAlert,
		event.AgentIsolated,
	}
	n.events = types
	for _, typ := range types {
		t := typ
		bus.Subscribe(t, func(e event.Event) {
			n.handle(context.Background(), e)
		})
	}
	slog.Info("notify: slack channel armed", "types", types)
}

func (n *SlackNotifier) handle(ctx context.Context, e event.Event) {
	if !n.shouldSend(e.Type) {
		return
	}
	text := formatSlack(e)
	timeout := time.Duration(n.cfg.TimeoutSecs) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	sendCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := n.sendFunc(sendCtx, n.cfg.WebhookURL, text); err != nil {
		slog.Warn("notify: slack send failed", "type", e.Type, "error", err)
		return
	}
	slog.Info("notify: slack sent", "type", e.Type)
}

func (n *SlackNotifier) shouldSend(eventType string) bool {
	// Small internal lock-free helper — simple enough that a single mutex
	// inside slackMu would be overkill for this low-frequency path.
	now := n.now()
	if last, ok := n.m.lastSent[eventType]; ok {
		if now.Sub(last) < n.debounce {
			return false
		}
	}
	n.m.lastSent[eventType] = now
	return true
}

// formatSlack produces the JSON body Slack's Incoming Webhook API accepts:
// {"text": "..."}. Could grow to blocks/attachments later if needed.
func formatSlack(e event.Event) string {
	return fmt.Sprintf(":warning: *%s* from `%s` — %v", e.Type, e.Source, e.Payload)
}

// postSlack sends a plain Slack webhook POST.
func postSlack(ctx context.Context, url, text string) error {
	body, _ := json.Marshal(map[string]string{"text": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("slack responded %d", resp.StatusCode)
	}
	return nil
}
