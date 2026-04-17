//go:build claude_e2e

// Package devloop's Claude-Code-backed adapters invoke the real `claude`
// CLI — that's expensive, non-deterministic, and requires the CLI to be
// installed. This file is behind the `claude_e2e` build tag so the
// normal `go test ./...` run never touches it. To exercise it:
//
//	go test -tags claude_e2e ./internal/devloop -run TestClaudeCode -count=1 -v
package devloop

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClaudeCodeDevWritesSomething smoke-tests the real Claude CLI
// invocation: build a throwaway workdir, hand the Dev adapter a tiny
// story, and verify Claude produced a non-empty summary. This isn't a
// quality check — it's a smoke test that the CLI integration doesn't
// silently swallow every response.
func TestClaudeCodeDevWritesSomething(t *testing.T) {
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude CLI not on PATH; skipping")
	}

	workdir := t.TempDir()
	// Drop a README so Claude has something to reference; without it the
	// CLI sometimes refuses to operate on an empty directory.
	require.NoError(t, os.WriteFile(filepath.Join(workdir, "README.md"),
		[]byte("# Hive e2e smoke\n\nwrite a HELLO.txt saying hi\n"), 0o644))

	dev := NewClaudeCodeDev()
	require.NotNil(t, dev)
	// If the CLI was present but NewClaudeCodeDev fell back to scripted
	// for any reason, this test would silently pass on the scripted path.
	// Assert we got the real adapter.
	assert.Equal(t, "claude-dev", dev.Name(),
		"wanted the real claude adapter, got: "+dev.Name())

	proj := ProjectContext{
		ID:      "prj_e2e",
		Idea:    "write a HELLO.txt that says hi",
		Workdir: workdir,
	}
	story := Story{
		ID:    "sty_e2e",
		Title: "HELLO.txt",
		ACs: []AcceptanceCriterion{
			{ID: 1, Text: "HELLO.txt exists in the workdir and contains the word hi"},
		},
	}

	out, err := dev.Develop(context.Background(), proj, story, 1, "")
	require.NoError(t, err)
	assert.NotEmpty(t, out.Summary, "Claude should have returned a non-empty summary")

	// Does not assert the file exists — Claude Code may refuse writes
	// under some sandboxing modes and our adapter still returns a text
	// explanation. The thing we want to catch is a broken invocation
	// (empty stdout, timeout, exit != 0), which NoError + NotEmpty
	// cover. File-level assertions happen at the Reviewer layer.
}
