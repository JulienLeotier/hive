package storage

import "fmt"

// OpenFromConfig picks a storage backend based on a config snapshot.
// Story 22.1: `storage: postgres` routes through OpenPostgres; everything else
// stays on the SQLite default.
type Backend struct {
	Type        string // "sqlite" or "postgres"
	DataDir     string // for sqlite
	PostgresURL string // for postgres
}

// Open picks the right backend constructor for the configured type.
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
