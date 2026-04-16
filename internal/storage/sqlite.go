package storage

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/JulienLeotier/hive/internal/storage/migrations"
	_ "modernc.org/sqlite"
)

// Store manages the SQLite database connection and migrations.
type Store struct {
	DB  *sql.DB
	dir string
}

// Open creates or opens the SQLite database at the given directory.
// It ensures the directory exists, enables WAL mode, and runs migrations.
func Open(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("creating data directory %s: %w", dataDir, err)
	}

	dbPath := filepath.Join(dataDir, "hive.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database %s: %w", dbPath, err)
	}

	if err := configureSQLite(db); err != nil {
		db.Close()
		return nil, err
	}

	s := &Store{DB: db, dir: dataDir}

	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	slog.Info("database opened", "path", dbPath)
	return s, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.DB.Close()
}

func configureSQLite(db *sql.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA journal_size_limit=67108864",
		"PRAGMA foreign_keys=ON",
		"PRAGMA synchronous=NORMAL",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return fmt.Errorf("executing %s: %w", p, err)
		}
	}
	return nil
}

func (s *Store) migrate() error {
	// Ensure schema_versions table exists first
	if _, err := s.DB.Exec(`CREATE TABLE IF NOT EXISTS schema_versions (
		version INTEGER PRIMARY KEY,
		applied_at TEXT DEFAULT (datetime('now'))
	)`); err != nil {
		return fmt.Errorf("creating schema_versions: %w", err)
	}

	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return fmt.Errorf("reading migrations: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, f := range files {
		// Parse version from filename (e.g., "001_initial.sql" → 1)
		version, err := parseVersion(f)
		if err != nil {
			return fmt.Errorf("parsing migration filename %s: %w", f, err)
		}

		var exists int
		err = s.DB.QueryRow("SELECT COUNT(*) FROM schema_versions WHERE version = ?", version).Scan(&exists)
		if err != nil {
			return fmt.Errorf("checking migration version %d: %w", version, err)
		}
		if exists > 0 {
			continue
		}

		data, err := migrations.FS.ReadFile(f)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", f, err)
		}

		tx, err := s.DB.Begin()
		if err != nil {
			return fmt.Errorf("beginning transaction for migration %d: %w", version, err)
		}

		statements := strings.Split(string(data), ";")
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := tx.Exec(stmt); err != nil {
				tx.Rollback()
				return fmt.Errorf("executing migration %s statement: %w", f, err)
			}
		}

		if _, err := tx.Exec("INSERT INTO schema_versions (version) VALUES (?)", version); err != nil {
			tx.Rollback()
			return fmt.Errorf("recording migration version %d: %w", version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing migration %d: %w", version, err)
		}

		slog.Info("migration applied", "version", version, "file", f)
	}

	return nil
}

// parseVersion extracts the version number from a migration filename like "001_initial.sql".
func parseVersion(filename string) (int, error) {
	parts := strings.SplitN(filename, "_", 2)
	if len(parts) == 0 {
		return 0, fmt.Errorf("invalid migration filename: %s", filename)
	}
	v, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid version number in %s: %w", filename, err)
	}
	return v, nil
}
