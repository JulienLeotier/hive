// Package notify delivers operational alerts (task failures, budget breaches,
// agent isolation) to channels outside the event bus — email today, room for
// PagerDuty/Opsgenie later. Unlike the generic webhook.Dispatcher, notify is
// purpose-built for ops: the event filter is hardcoded to incident-shaped
// types, payloads are human-formatted, and SMTP is a first-class channel.
package notify

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"github.com/JulienLeotier/hive/internal/event"
)

// EmailConfig captures the SMTP settings required to deliver mail. Password
// is read from an env var so ops can avoid writing it to hive.yaml.
type EmailConfig struct {
	Host        string   // e.g. smtp.sendgrid.net
	Port        int      // 587 for STARTTLS, 465 for SMTPS
	From        string   // "Hive <alerts@example.com>"
	To          []string // default recipient list
	Username    string   // usually same as From local-part or an API key id
	Password    string   // populated from Env at load time
	StartTLS    bool     // true for port 587, false for plain 25 or implicit SMTPS on 465
	SMTPSOnly   bool     // true when the server expects TLS from the first byte (465)
	TimeoutSecs int      // total deadline per send, default 10
}

// Enabled reports whether the config is sufficient to attempt delivery.
func (c EmailConfig) Enabled() bool {
	return c.Host != "" && c.Port > 0 && c.From != "" && len(c.To) > 0
}

// Notifier subscribes to ops-shaped events and delivers them over email.
// Each event is debounced per-type so a storm of failures doesn't trigger
// hundreds of mails in the same minute; the first in a bucket goes through,
// subsequent ones are suppressed for debounceWindow.
type Notifier struct {
	email       EmailConfig
	debounce    time.Duration
	mu          sync.Mutex
	lastSent    map[string]time.Time // event.Type → last delivery timestamp
	now         func() time.Time
	sendFunc    func(ctx context.Context, cfg EmailConfig, subject, body string) error
}

// NewNotifier builds an email notifier. Leaves sending unconfigured when
// cfg.Enabled() is false — Attach then becomes a no-op so callers don't
// branch on config at wire time.
func NewNotifier(cfg EmailConfig) *Notifier {
	return &Notifier{
		email:    cfg,
		debounce: 60 * time.Second,
		lastSent: map[string]time.Time{},
		now:      time.Now,
		sendFunc: sendSMTP,
	}
}

// WithDebounce overrides the default 60s suppression window.
func (n *Notifier) WithDebounce(d time.Duration) *Notifier {
	n.debounce = d
	return n
}

// Enabled is a shortcut for email.Enabled() so serve.go can conditionally log.
func (n *Notifier) Enabled() bool {
	return n.email.Enabled()
}

// Attach subscribes the notifier to the ops event types. When the email
// config is not usable (missing host, recipients, etc.) Attach is a no-op,
// which keeps dev deployments quiet without the caller having to check.
func (n *Notifier) Attach(bus *event.Bus) {
	if !n.email.Enabled() {
		slog.Info("notify: email channel disabled — no SMTP config")
		return
	}
	opsTypes := []string{
		event.TaskFailed,
		event.CostAlert,
		event.AgentIsolated,
	}
	for _, typ := range opsTypes {
		t := typ
		bus.Subscribe(t, func(e event.Event) {
			n.handle(context.Background(), e)
		})
	}
	slog.Info("notify: email channel armed", "types", opsTypes, "recipients", len(n.email.To))
}

func (n *Notifier) handle(ctx context.Context, e event.Event) {
	if !n.shouldSend(e.Type) {
		slog.Debug("notify: debounced", "type", e.Type)
		return
	}
	subject, body := format(e)
	timeout := time.Duration(n.email.TimeoutSecs) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	sendCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := n.sendFunc(sendCtx, n.email, subject, body); err != nil {
		slog.Warn("notify: email send failed", "type", e.Type, "error", err)
		return
	}
	slog.Info("notify: email sent", "type", e.Type)
}

// shouldSend applies per-type debounce so bursts don't spam recipients.
// Returns true if the caller should send.
func (n *Notifier) shouldSend(eventType string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	now := n.now()
	if last, ok := n.lastSent[eventType]; ok {
		if now.Sub(last) < n.debounce {
			return false
		}
	}
	n.lastSent[eventType] = now
	return true
}

// format turns an event into a human-readable subject + body. Kept deliberately
// minimal — multi-line text with the payload dumped as JSON under a heading,
// which travels well through every mail client without requiring HTML.
func format(e event.Event) (subject, body string) {
	subject = fmt.Sprintf("[Hive] %s — %s", strings.ToUpper(e.Type), e.Source)
	var b strings.Builder
	fmt.Fprintf(&b, "Event:    %s\n", e.Type)
	fmt.Fprintf(&b, "Source:   %s\n", e.Source)
	fmt.Fprintf(&b, "When:     %s\n", e.CreatedAt.Format(time.RFC3339))
	b.WriteString("\n--- Payload ---\n")
	fmt.Fprintf(&b, "%v\n", e.Payload)
	b.WriteString("\n--\nSent by Hive notify. Adjust filters in hive.yaml:notifications.email.\n")
	body = b.String()
	return
}

// sendSMTP dispatches a single message. Supports STARTTLS (port 587 style) and
// implicit SMTPS (port 465). Authentication is PLAIN when a password is set.
// Falls back to unauthenticated relay for dev MTAs (MailHog, Mailpit).
func sendSMTP(ctx context.Context, cfg EmailConfig, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	msg := buildMessage(cfg.From, cfg.To, subject, body)

	dialer := &smtpDialer{addr: addr, host: cfg.Host}

	var c *smtp.Client
	var err error
	if cfg.SMTPSOnly {
		c, err = dialer.dialTLS()
	} else {
		c, err = dialer.dial()
	}
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	defer func() { _ = c.Close() }()

	if cfg.StartTLS && !cfg.SMTPSOnly {
		if err := c.StartTLS(&tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}

	if cfg.Password != "" {
		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}

	if err := c.Mail(extractAddress(cfg.From)); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	for _, to := range cfg.To {
		if err := c.Rcpt(to); err != nil {
			return fmt.Errorf("rcpt %s: %w", to, err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		_ = w.Close()
		return fmt.Errorf("write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close data: %w", err)
	}
	// Quit is best-effort: if the server has already hung up after DATA,
	// returning the error would mask a successful send.
	_ = c.Quit()
	return nil
}

type smtpDialer struct {
	addr string
	host string
}

func (d *smtpDialer) dial() (*smtp.Client, error) {
	return smtp.Dial(d.addr)
}

func (d *smtpDialer) dialTLS() (*smtp.Client, error) {
	conn, err := tls.Dial("tcp", d.addr, &tls.Config{ServerName: d.host, MinVersion: tls.VersionTLS12})
	if err != nil {
		return nil, err
	}
	return smtp.NewClient(conn, d.host)
}

func buildMessage(from string, to []string, subject, body string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	b.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	b.WriteString("\r\n")
	b.WriteString(body)
	return b.String()
}

// extractAddress pulls just the "user@host" portion out of a "Name <user@host>"
// From header — SMTP MAIL FROM doesn't accept display names.
func extractAddress(addr string) string {
	if i := strings.LastIndex(addr, "<"); i >= 0 {
		if j := strings.Index(addr[i:], ">"); j >= 0 {
			return addr[i+1 : i+j]
		}
	}
	return strings.TrimSpace(addr)
}
