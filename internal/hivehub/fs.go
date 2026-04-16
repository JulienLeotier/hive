package hivehub

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func joinPath(root, rel string) string {
	return filepath.Join(root, rel)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func writeTemplateFile(root, relPath, content string) error {
	abs := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(abs), err)
	}
	return os.WriteFile(abs, []byte(content), 0o644)
}

// walkTemplateDir visits every file under root, skipping common junk.
func walkTemplateDir(root string, visit func(relPath, content string)) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if shouldSkipFile(d.Name()) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		visit(filepath.ToSlash(rel), string(content))
		return nil
	})
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", "node_modules", "dist", ".svelte-kit", ".claude", "_bmad-output", ".hive":
		return true
	}
	return false
}

func shouldSkipFile(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch name {
	case "hive.db", "hive.db-wal", "hive.db-shm":
		return true
	}
	return false
}
