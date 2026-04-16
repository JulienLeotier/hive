package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, 8233, cfg.Port)
	assert.Contains(t, cfg.DataDir, ".hive")
}

func TestLoadFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "hive.yaml")
	err := os.WriteFile(cfgPath, []byte("log_level: debug\nport: 9999\n"), 0644)
	require.NoError(t, err)

	cfg, err := Load(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, 9999, cfg.Port)
}

func TestLoadMissingFileUsesDefaults(t *testing.T) {
	cfg, err := Load("/nonexistent/hive.yaml")
	require.NoError(t, err)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, 8233, cfg.Port)
}

func TestEnvOverrides(t *testing.T) {
	t.Setenv("HIVE_LOG_LEVEL", "debug")
	t.Setenv("HIVE_PORT", "7777")
	t.Setenv("HIVE_DATA_DIR", "/tmp/hive-test")

	cfg, err := Load("/nonexistent/hive.yaml")
	require.NoError(t, err)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, 7777, cfg.Port)
	assert.Equal(t, "/tmp/hive-test", cfg.DataDir)
}

func TestEnvOverridesTakesPrecedenceOverYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "hive.yaml")
	err := os.WriteFile(cfgPath, []byte("log_level: warn\nport: 1111\n"), 0644)
	require.NoError(t, err)

	t.Setenv("HIVE_LOG_LEVEL", "error")

	cfg, err := Load(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, "error", cfg.LogLevel)
	assert.Equal(t, 1111, cfg.Port)
}

func TestTildeExpansion(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "hive.yaml")
	err := os.WriteFile(cfgPath, []byte("data_dir: ~/.hive/data\n"), 0644)
	require.NoError(t, err)

	cfg, err := Load(cfgPath)
	require.NoError(t, err)

	home, _ := os.UserHomeDir()
	assert.Equal(t, filepath.Join(home, ".hive", "data"), cfg.DataDir)
}
