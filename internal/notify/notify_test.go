package notify

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JulienLeotier/hive/internal/event"
	_ "modernc.org/sqlite"
)

// busForTest spins up a minimal in-memory Bus. We skip storage.Open to
// avoid pulling in the full migration stack — the bus only needs a
// *sql.DB with an `events` table to persist to. Since Publish persists
// before delivery, we point it at :memory: with the events table
// created inline.
func busForTest(t *testing.T) *event.Bus {
	t.Helper()
	db, err := openMemoryDB()
	if err != nil {
		t.Fatalf("openMemoryDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return event.NewBus(db)
}

func TestAttachNoWebhook(t *testing.T) {
	t.Setenv("HIVE_SLACK_WEBHOOK", "")
	bus := busForTest(t)
	if Attach(bus) {
		t.Fatal("Attach should return false when webhook env var is empty")
	}
}

func TestAttachNilBus(t *testing.T) {
	t.Setenv("HIVE_SLACK_WEBHOOK", "https://example.invalid")
	if Attach(nil) {
		t.Fatal("Attach should return false for nil bus")
	}
}

func TestAttachForwardsRelevantEvent(t *testing.T) {
	var hits int32
	var lastBody []byte
	done := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		lastBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(200)
		select {
		case done <- struct{}{}:
		default:
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("HIVE_SLACK_WEBHOOK", srv.URL)

	bus := busForTest(t)
	if !Attach(bus) {
		t.Fatal("Attach should have wired up")
	}

	_, err := bus.Publish(context.Background(), "project.shipped", "test",
		map[string]string{"project_id": "prj_42"})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("webhook was not called within 3s")
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("want 1 hit, got %d", hits)
	}
	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(lastBody, &payload); err != nil {
		t.Fatalf("unmarshal body: %v (%s)", err, string(lastBody))
	}
	if payload.Text == "" {
		t.Fatal("text field empty")
	}
	if !containsAll(payload.Text, "shipped", "prj_42") {
		t.Fatalf("text missing expected tokens: %q", payload.Text)
	}
}

func TestAttachIgnoresIrrelevantEvent(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("HIVE_SLACK_WEBHOOK", srv.URL)

	bus := busForTest(t)
	Attach(bus)

	_, _ = bus.Publish(context.Background(), "story.dev_started", "test", nil)
	_, _ = bus.Publish(context.Background(), "project.bmad_step_finished", "test", nil)

	// Give any goroutines a beat to misfire.
	time.Sleep(150 * time.Millisecond)
	if got := atomic.LoadInt32(&hits); got != 0 {
		t.Fatalf("irrelevant events leaked to webhook: %d hits", got)
	}
}

func TestSummarizePrettyJSON(t *testing.T) {
	e := event.Event{Type: "project.shipped", Payload: `{"project_id":"p1","ok":true}`}
	got := summarize(e)
	if !containsAll(got, "project_id", "p1", "true") {
		t.Fatalf("summarize dropped fields: %q", got)
	}
}

func TestSummarizeFallbackRaw(t *testing.T) {
	e := event.Event{Type: "project.shipped", Payload: "not json"}
	if summarize(e) != "not json" {
		t.Fatalf("unexpected fallback: %q", summarize(e))
	}
}

func TestSummarizeTruncates(t *testing.T) {
	big := make([]byte, 4000)
	for i := range big {
		big[i] = 'a'
	}
	e := event.Event{Type: "x", Payload: string(big)}
	if got := len(summarize(e)); got > 1501 {
		t.Fatalf("summary not truncated, len=%d", got)
	}
}

func TestSummarizeEmpty(t *testing.T) {
	e := event.Event{Type: "project.shipped", Payload: ""}
	if summarize(e) != e.Type {
		t.Fatalf("empty payload should fall back to type")
	}
}

// --- helpers ---

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(sub) == 0 || indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// Minimal sqlite DB with only the events table the bus needs — skips
// the full migration stack so we don't pull in storage→bus cycles.
func openMemoryDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		type TEXT NOT NULL,
		source TEXT,
		payload TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}
