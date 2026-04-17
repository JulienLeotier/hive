package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"
)

// TriggerPayload is the input handed to the first task of a triggered workflow.
type TriggerPayload struct {
	Source string         `json:"source"` // "manual", "webhook", "schedule"
	Body   map[string]any `json:"body,omitempty"`
}

// RunFn executes a workflow given its config and the trigger payload.
// The engine.Run entry point satisfies this shape.
type RunFn func(ctx context.Context, cfg *Config, payload TriggerPayload) error

// TriggerManager wires workflow triggers (manual, webhook, schedule).
// Multiple workflows can be registered; the manager fans out firings.
type TriggerManager struct {
	run RunFn

	mu         sync.Mutex
	workflows  []*Config
	schedulers []*scheduler
	webhooks   map[string]*Config // path → workflow
}

// NewTriggerManager builds a trigger manager that calls run on each firing.
func NewTriggerManager(run RunFn) *TriggerManager {
	return &TriggerManager{
		run:      run,
		webhooks: map[string]*Config{},
	}
}

// Register adds a workflow and starts any schedule it declares.
func (m *TriggerManager) Register(ctx context.Context, cfg *Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.workflows = append(m.workflows, cfg)
	if cfg.Trigger == nil {
		return nil
	}

	switch cfg.Trigger.Type {
	case "schedule":
		interval, err := parseSchedule(cfg.Trigger.Schedule)
		if err != nil {
			return fmt.Errorf("workflow %s: %w", cfg.Name, err)
		}
		s := newScheduler(cfg, interval, m.run)
		m.schedulers = append(m.schedulers, s)
		s.start(ctx)
		slog.Info("schedule trigger armed", "workflow", cfg.Name, "interval", interval)
	case "webhook":
		path := cfg.Trigger.Webhook
		if path == "" {
			return fmt.Errorf("workflow %s: webhook trigger needs a path", cfg.Name)
		}
		m.webhooks[path] = cfg
		slog.Info("webhook trigger armed", "workflow", cfg.Name, "path", path)
	case "manual", "":
		// Manual: no wiring needed; FireManual() triggers explicitly.
	default:
		return fmt.Errorf("workflow %s: unknown trigger type %q", cfg.Name, cfg.Trigger.Type)
	}
	return nil
}

// Stop halts every scheduled firing.
func (m *TriggerManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.schedulers {
		s.stop()
	}
	m.schedulers = nil
}

// FireManual runs a registered workflow on demand with the given payload.
func (m *TriggerManager) FireManual(ctx context.Context, workflowName string, body map[string]any) error {
	m.mu.Lock()
	var cfg *Config
	for _, c := range m.workflows {
		if c.Name == workflowName {
			cfg = c
			break
		}
	}
	m.mu.Unlock()
	if cfg == nil {
		return fmt.Errorf("workflow %q not registered", workflowName)
	}
	return m.run(ctx, cfg, TriggerPayload{Source: "manual", Body: body})
}

// FireWebhook runs the workflow bound to path with the webhook body.
// Multiple concurrent firings of the same path spawn independent workflow runs (FR10).
func (m *TriggerManager) FireWebhook(ctx context.Context, path string, body map[string]any) error {
	m.mu.Lock()
	cfg := m.webhooks[path]
	m.mu.Unlock()
	if cfg == nil {
		return fmt.Errorf("no webhook trigger registered at %q", path)
	}
	return m.run(ctx, cfg, TriggerPayload{Source: "webhook", Body: body})
}

// WebhookPaths returns the paths that have a webhook trigger wired.
func (m *TriggerManager) WebhookPaths() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.webhooks))
	for p := range m.webhooks {
		out = append(out, p)
	}
	return out
}

// WebhookSecret returns the HMAC secret declared for the workflow bound at
// `path`, or "" if none is registered. Used by the HTTP transport to decide
// whether to require `X-Hive-Signature` on incoming requests.
func (m *TriggerManager) WebhookSecret(path string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg := m.webhooks[path]
	if cfg == nil || cfg.Trigger == nil {
		return ""
	}
	return cfg.Trigger.Secret
}

// ---------------------------------------------------------------------
// scheduler — tiny cron-like firing loop
// ---------------------------------------------------------------------

type scheduler struct {
	cfg      *Config
	interval time.Duration
	run      RunFn
	stopCh   chan struct{}
}

func newScheduler(cfg *Config, interval time.Duration, run RunFn) *scheduler {
	return &scheduler{cfg: cfg, interval: interval, run: run, stopCh: make(chan struct{})}
}

func (s *scheduler) start(ctx context.Context) {
	go func() {
		t := time.NewTicker(s.interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				if err := s.run(ctx, s.cfg, TriggerPayload{Source: "schedule"}); err != nil {
					slog.Error("scheduled workflow failed", "workflow", s.cfg.Name, "error", err)
				}
			case <-s.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (s *scheduler) stop() {
	select {
	case <-s.stopCh:
		// already closed
	default:
		close(s.stopCh)
	}
}

// parseSchedule accepts two formats:
//   - a Go duration string ("30s", "5m", "1h")
//   - a very narrow cron-like "*/N * * * *" meaning "every N minutes"
//
// Everything else is rejected with a clear error. Full cron support is
// intentionally out of scope — the common cases are interval-based reminders.
func parseSchedule(spec string) (time.Duration, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return 0, fmt.Errorf("empty schedule")
	}
	if d, err := time.ParseDuration(spec); err == nil && d > 0 {
		return d, nil
	}
	// Minimal cron: "*/N * * * *"
	parts := strings.Fields(spec)
	if len(parts) == 5 && strings.HasPrefix(parts[0], "*/") && parts[1] == "*" && parts[2] == "*" && parts[3] == "*" && parts[4] == "*" {
		n, err := strconv.Atoi(strings.TrimPrefix(parts[0], "*/"))
		if err == nil && n > 0 {
			return time.Duration(n) * time.Minute, nil
		}
	}
	return 0, fmt.Errorf("schedule %q: use a Go duration (e.g. 30s, 5m) or \"*/N * * * *\"", spec)
}
