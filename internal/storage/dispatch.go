package storage

import "fmt"

// Backend identifies the storage driver to open plus its driver-specific
// parameters. Story 22.1: `storage: postgres` routes through OpenPostgres;
// everything else stays on the SQLite default.
type Backend struct {
	Type        string // "sqlite" or "postgres"
	DataDir     string // for sqlite
	PostgresURL string // for postgres
}

// Open2 picks the right backend constructor for the configured type.
// (The `2` suffix distinguishes this dispatcher from the legacy
// single-driver Open; both coexist while callers migrate.)
func Open2(cfg Backend) (*Store, error) {
	switch cfg.Type {
	case "", "sqlite":
		return Open(cfg.DataDir)
	case "postgres":
		return OpenPostgres(cfg.PostgresURL)
	default:
		return nil, fmt.Errorf("unknown storage backend %q", cfg.Type)
	}
}
