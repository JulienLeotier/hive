package storage

import (
	"database/sql"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/JulienLeotier/hive/internal/storage/migrations"
	_ "github.com/lib/pq"
)

// OpenPostgres opens a PostgreSQL-backed Store and runs the postgres migration set.
// Story 22.1. Connection string format: postgres://user:pass@host:port/db?sslmode=disable
func OpenPostgres(dsn string) (*Store, error) {
	if dsn == "" {
		return nil, fmt.Errorf("postgres DSN is required")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening postgres %s: %w", dsn, err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging postgres: %w", err)
	}

	s := &Store{DB: db, dir: ""}
	if err := migratePostgres(s); err != nil {
		db.Close()
		return nil, fmt.Errorf("running postgres migrations: %w", err)
	}

	slog.Info("postgres database opened")
	return s, nil
}

func migratePostgres(s *Store) error {
	if _, err := s.DB.Exec(`CREATE TABLE IF NOT EXISTS schema_versions (
		version INTEGER PRIMARY KEY,
		applied_at TEXT DEFAULT (to_char(CURRENT_TIMESTAMP, 'YYYY-MM-DD HH24:MI:SS'))
	)`); err != nil {
		return fmt.Errorf("creating schema_versions: %w", err)
	}

	entries, err := migrations.FS.ReadDir("postgres")
	if err != nil {
		return fmt.Errorf("reading postgres migrations: %w", err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, f := range files {
		version, err := parseVersion(f)
		if err != nil {
			return fmt.Errorf("parsing postgres migration %s: %w", f, err)
		}

		var exists int
		if err := s.DB.QueryRow("SELECT COUNT(*) FROM schema_versions WHERE version = $1", version).Scan(&exists); err != nil {
			return fmt.Errorf("checking postgres migration %d: %w", version, err)
		}
		if exists > 0 {
			continue
		}

		data, err := migrations.FS.ReadFile("postgres/" + f)
		if err != nil {
			return fmt.Errorf("reading postgres migration %s: %w", f, err)
		}

		tx, err := s.DB.Begin()
		if err != nil {
			return err
		}
		for _, stmt := range strings.Split(string(data), ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := tx.Exec(stmt); err != nil {
				tx.Rollback()
				return fmt.Errorf("executing postgres migration %s statement: %w", f, err)
			}
		}
		if _, err := tx.Exec("INSERT INTO schema_versions (version) VALUES ($1)", version); err != nil {
			tx.Rollback()
			return fmt.Errorf("recording postgres migration %d: %w", version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing postgres migration %d: %w", version, err)
		}
		slog.Info("postgres migration applied", "version", version, "file", f)
	}
	return nil
}
