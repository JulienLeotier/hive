package hivehub

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// PushToRegistry clones a HiveHub Git registry, drops the manifest file in,
// commits it on a publish branch, and pushes. The caller opens a PR against
// that branch. Story 14.1.
//
// Requires a working `git` binary in PATH and credentials already configured
// (SSH key or token helper). Clone path is deterministic so callers can
// inspect the workspace on failure.
func PushToRegistry(repoURL, manifestPath, name, version string) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found in PATH: %w", err)
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("reading manifest: %w", err)
	}

	workdir, err := os.MkdirTemp("", "hivehub-push-")
	if err != nil {
		return err
	}

	clone := filepath.Join(workdir, "registry")
	if out, err := run(workdir, "git", "clone", "--depth", "1", repoURL, clone); err != nil {
		return fmt.Errorf("git clone: %w (%s)", err, out)
	}

	branch := fmt.Sprintf("publish/%s-%s", name, version)
	if out, err := run(clone, "git", "checkout", "-b", branch); err != nil {
		return fmt.Errorf("git checkout: %w (%s)", err, out)
	}

	destDir := filepath.Join(clone, "templates", name)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	destFile := filepath.Join(destDir, filepath.Base(manifestPath))
	if err := os.WriteFile(destFile, data, 0o644); err != nil {
		return err
	}

	if out, err := run(clone, "git", "add", "."); err != nil {
		return fmt.Errorf("git add: %w (%s)", err, out)
	}
	if out, err := run(clone, "git", "commit", "-m", fmt.Sprintf("Publish %s v%s", name, version)); err != nil {
		return fmt.Errorf("git commit: %w (%s)", err, out)
	}
	if out, err := run(clone, "git", "push", "--set-upstream", "origin", branch); err != nil {
		return fmt.Errorf("git push: %w (%s)", err, out)
	}

	return nil
}

func run(dir string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}
