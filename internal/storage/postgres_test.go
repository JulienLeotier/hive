package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOpenPostgresRejectsEmptyDSN(t *testing.T) {
	_, err := OpenPostgres("")
	assert.Error(t, err)
}

func TestOpen2DispatchesOnType(t *testing.T) {
	// Unknown backend type should error clearly.
	_, err := Open2(Backend{Type: "mongo"})
	assert.Error(t, err)

	// Empty type falls through to SQLite which needs a valid DataDir.
	store, err := Open2(Backend{DataDir: t.TempDir()})
	assert.NoError(t, err)
	if store != nil {
		store.Close()
	}
}
