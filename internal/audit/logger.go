package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Entry represents an audit log entry.
type Entry struct {
	ID        int64     `json:"id"`
	Action    string    `json:"action"`
	Actor     string    `json:"actor"`
	Resource  string    `json:"resource"`
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"created_at"`
}

// Logger records audit events for compliance.
type Logger struct {
	db *sql.DB
}

// NewLogger creates an audit logger.
func NewLogger(db *sql.DB) *Logger {
	return &Logger{db: db}
}

// Log records an audit entry.
func (l *Logger) Log(ctx context.Context, action, actor, resource, detail string) error {
	_, err := l.db.ExecContext(ctx,
		`INSERT INTO audit_log (action, actor, resource, detail) VALUES (?, ?, ?, ?)`,
		action, actor, resource, detail,
	)
	return err
}

// Query returns audit entries matching filters.
func (l *Logger) Query(ctx context.Context, since time.Time, limit int) ([]Entry, error) {
	// datetime('now') in SQLite is UTC — normalise the filter to match.
	rows, err := l.db.QueryContext(ctx,
		`SELECT id, action, actor, resource, detail, created_at FROM audit_log
		 WHERE created_at >= ? ORDER BY created_at DESC LIMIT ?`,
		since.UTC().Format("2006-01-02 15:04:05"), limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var created string
		if err := rows.Scan(&e.ID, &e.Action, &e.Actor, &e.Resource, &e.Detail, &created); err != nil {
			continue
		}
		e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ExportJSON exports audit entries as JSON.
func (l *Logger) ExportJSON(entries []Entry) ([]byte, error) {
	return json.MarshalIndent(entries, "", "  ")
}

// ExportCSV exports audit entries as CSV with injection protection.
func (l *Logger) ExportCSV(entries []Entry) string {
	csv := "id,action,actor,resource,detail,created_at\n"
	for _, e := range entries {
		csv += fmt.Sprintf("%d,%q,%q,%q,%q,%s\n",
			e.ID, csvSafe(e.Action), csvSafe(e.Actor), csvSafe(e.Resource), csvSafe(e.Detail), e.CreatedAt.Format(time.RFC3339))
	}
	return csv
}

// csvSafe prevents CSV injection by prefixing dangerous characters with a single quote.
func csvSafe(s string) string {
	if len(s) > 0 && (s[0] == '=' || s[0] == '+' || s[0] == '-' || s[0] == '@') {
		return "'" + s
	}
	return s
}
