package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCopyTemplateCopiesFullTree verifies Stories 7.5/7.6/7.7: `hive init
// --template <name>` copies workflow + agent configs + README.
func TestCopyTemplateCopiesFullTree(t *testing.T) {
	dest := t.TempDir() + "/proj"
	require.NoError(t, copyTemplate("code-review", dest))

	for _, rel := range []string{
		"hive.yaml",
		"agents/reviewer.yaml",
		"agents/summarizer.yaml",
		"README.md",
	} {
		path := filepath.Join(dest, rel)
		info, err := os.Stat(path)
		require.NoError(t, err, "missing %s", rel)
		assert.Greater(t, info.Size(), int64(0), "%s should have content", rel)
	}
}

func TestCopyTemplateRejectsUnknown(t *testing.T) {
	err := copyTemplate("does-not-exist", t.TempDir())
	assert.Error(t, err)
}
