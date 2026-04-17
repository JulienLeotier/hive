package cli

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTarGzRoundTrip(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "orig.db")
	payload := []byte("SQLite format 3\x00fake db bytes")
	require.NoError(t, os.WriteFile(src, payload, 0o600))

	archive := filepath.Join(dir, "backup.tar.gz")
	require.NoError(t, writeTarGz(archive, src))

	destDir := filepath.Join(dir, "restore")
	require.NoError(t, os.MkdirAll(destDir, 0o700))
	require.NoError(t, extractTarGz(archive, destDir))

	restored, err := os.ReadFile(filepath.Join(destDir, "hive.db"))
	require.NoError(t, err)
	assert.Equal(t, payload, restored)
}

func TestExtractRejectsTraversalEntry(t *testing.T) {
	// Build an archive containing an entry named "../evil" to verify that
	// the extractor rejects anything not literally "hive.db" — otherwise a
	// malicious backup could overwrite files outside the data dir.
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.tar.gz")

	f, err := os.Create(bad)
	require.NoError(t, err)
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name: "../evil.db", Mode: 0o600, Size: 4,
	}))
	_, _ = tw.Write([]byte("xxxx"))
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	require.NoError(t, f.Close())

	err = extractTarGz(bad, dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected entry")
}
