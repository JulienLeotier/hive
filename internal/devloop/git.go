package devloop

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GitCommitter is the minimum Git surface the supervisor needs: init a
// repo if one doesn't exist, and land a commit after a successful story
// iteration so the build history is navigable. All operations are
// best-effort — a missing `git` CLI or a failing commit degrades to
// "no-op + warn" rather than blocking the build.
//
// We deliberately do NOT create one branch per epic. Claude Code tends
// to touch files across epic boundaries (Foundations' scaffolding + a
// later epic's flow both edit the main entrypoint), so a flat linear
// history on the default branch keeps the story→commit mapping
// unambiguous for humans reviewing the run. Operators who want per-epic
// branches can rebase afterwards.
type GitCommitter struct {
	gitPath string
	timeout time.Duration
}

// NewGitCommitter probes for the git CLI. Returns nil + a `disabled`
// marker if git isn't on PATH — callers can branch on that or just let
// the methods no-op via the nil receiver.
func NewGitCommitter() *GitCommitter {
	path, err := exec.LookPath("git")
	if err != nil {
		return nil
	}
	return &GitCommitter{gitPath: path, timeout: 30 * time.Second}
}

// EnsureRepo initialises a git repo in workdir if none exists. Also
// seeds an initial commit so subsequent commits have a parent (git
// otherwise complains on empty-tree commits on macOS stock git). Safe
// to call repeatedly — no-op when workdir already has .git.
func (g *GitCommitter) EnsureRepo(ctx context.Context, workdir string) error {
	if g == nil {
		return nil
	}
	if workdir == "" {
		return errors.New("git: empty workdir")
	}
	if _, err := exec.LookPath("git"); err != nil {
		return nil
	}
	// Already a repo? Check for .git as directory or file (worktrees).
	if out, err := g.run(ctx, workdir, "rev-parse", "--is-inside-work-tree"); err == nil && strings.TrimSpace(string(out)) == "true" {
		return nil
	}
	if _, err := g.run(ctx, workdir, "init", "-b", "main"); err != nil {
		return fmt.Errorf("git init: %w", err)
	}
	// Configure a per-repo author so the commits don't inherit the
	// operator's global identity — the agents produced these.
	_, _ = g.run(ctx, workdir, "config", "user.name", "Hive BMAD")
	_, _ = g.run(ctx, workdir, "config", "user.email", "bmad@hive.local")
	// Seed an initial commit so the next one has a parent. Uses a
	// markdown placeholder so the repo isn't empty on clone.
	readmePath := filepath.Join(workdir, "BUILD.md")
	if _, err := exec.Command("sh", "-c", fmt.Sprintf("test -f %s || echo '# Hive BMAD build' > %s", readmePath, readmePath)).CombinedOutput(); err != nil {
		// best-effort; proceed even if it fails
		_ = err
	}
	_, _ = g.run(ctx, workdir, "add", "-A")
	_, _ = g.run(ctx, workdir, "commit", "-m", "chore: initial Hive BMAD build scaffold", "--allow-empty")
	return nil
}

// CommitStory stages all changes in workdir and commits with a
// story-scoped message. Safe when there's nothing to commit (empty
// diff), in which case it returns nil without creating an empty commit.
func (g *GitCommitter) CommitStory(ctx context.Context, workdir, storyTitle string, iteration int) error {
	if g == nil || workdir == "" {
		return nil
	}
	if _, err := g.run(ctx, workdir, "add", "-A"); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	// Skip when there are no staged changes — git commit would error.
	if out, err := g.run(ctx, workdir, "diff", "--cached", "--name-only"); err == nil {
		if strings.TrimSpace(string(out)) == "" {
			return nil
		}
	}
	msg := fmt.Sprintf("feat(story): %s\n\nIteration %d, produced by Hive BMAD devloop.",
		sanitiseCommitLine(storyTitle), iteration)
	if _, err := g.run(ctx, workdir, "commit", "-m", msg); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	return nil
}

// run executes a git subcommand in workdir with the configured timeout.
// Returns the combined stdout/stderr so error messages carry context.
func (g *GitCommitter) run(ctx context.Context, workdir string, args ...string) ([]byte, error) {
	callCtx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()
	cmd := exec.CommandContext(callCtx, g.gitPath, args...)
	cmd.Dir = workdir
	return cmd.CombinedOutput()
}

// sanitiseCommitLine trims newlines so a multi-line story title can't
// break the commit message subject line.
func sanitiseCommitLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) > 72 {
		s = s[:69] + "…"
	}
	return s
}
