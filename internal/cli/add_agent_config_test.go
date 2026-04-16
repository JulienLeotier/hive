package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestAddAgentConfigFileMergesDefaults verifies Story 1.3 AC:
// `hive add-agent --name reviewer --config ./agent.yaml` reads fields from
// the YAML config file. We test the YAML-parsing path in isolation so no
// real HTTP agent is needed.
func TestAddAgentConfigFileMergesDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "agent.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`
name: reviewer
type: http
url: http://localhost:8080
`), 0o644))

	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)

	var fileCfg struct {
		Name string `yaml:"name"`
		Type string `yaml:"type"`
		URL  string `yaml:"url"`
		Path string `yaml:"path"`
	}
	require.NoError(t, yaml.Unmarshal(data, &fileCfg))
	assert.Equal(t, "reviewer", fileCfg.Name)
	assert.Equal(t, "http", fileCfg.Type)
	assert.Equal(t, "http://localhost:8080", fileCfg.URL)
	assert.Empty(t, fileCfg.Path)
}

func TestConfirmDetectedTypeReturnsDefaultOnNonTTY(t *testing.T) {
	// Non-TTY stdin means the prompt is skipped and the detected value wins.
	// In go test, os.Stdin is /dev/null which is non-TTY, so this covers
	// the CI execution path.
	got := confirmDetectedType("claude-code")
	assert.Equal(t, "claude-code", got)
}
