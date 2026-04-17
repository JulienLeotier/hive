package api

import (
	"database/sql"
	"log/slog"
)

// scanAll drains rows into whatever the scanRow closure appends to. Scan
// errors are logged (not silently swallowed as the previous copy-pasted
// loop did — that masked data corruption and schema drift) while still
// letting the rest of the page render. Caller is responsible for handling
// the terminal rows.Err() after this returns if they care about partial
// results vs total failure.
//
// Usage:
//
//	var out []Foo
//	scanAll(rows, "foos", func() error {
//	    var f Foo
//	    if err := rows.Scan(&f.A, &f.B); err != nil { return err }
//	    out = append(out, f)
//	    return nil
//	})
//	writeJSON(w, out)
func scanAll(rows *sql.Rows, table string, scanRow func() error) {
	defer rows.Close()
	for rows.Next() {
		if err := scanRow(); err != nil {
			slog.Warn("row scan failed", "table", table, "error", err)
		}
	}
	if err := rows.Err(); err != nil {
		slog.Warn("row iteration failed", "table", table, "error", err)
	}
}
