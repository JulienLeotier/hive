package api

import (
	"database/sql"
	"log/slog"
)

// scanAll drains rows into whatever the scanRow closure appends to. Scan
// errors are logged (not silently swallowed as the previous copy-pasted
// loop did — that masked data corruption and schema drift) while still
// letting the rest of the page render.
//
// The CALLER must `defer rows.Close()` before calling scanAll. Keeping the
// close at the call site keeps lifetime analysis (and static linters like
// sqlclosecheck) happy, and matches the idiomatic database/sql pattern.
//
// Usage:
//
//	rows, err := db.QueryContext(...)
//	if err != nil { return err }
//	defer rows.Close()
//	var out []Foo
//	scanAll(rows, "foos", func() error {
//	    var f Foo
//	    if err := rows.Scan(&f.A, &f.B); err != nil { return err }
//	    out = append(out, f)
//	    return nil
//	})
func scanAll(rows *sql.Rows, table string, scanRow func() error) {
	for rows.Next() {
		if err := scanRow(); err != nil {
			slog.Warn("row scan failed", "table", table, "error", err)
		}
	}
	if err := rows.Err(); err != nil {
		slog.Warn("row iteration failed", "table", table, "error", err)
	}
}
