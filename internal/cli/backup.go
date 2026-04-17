package cli

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/JulienLeotier/hive/internal/config"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/spf13/cobra"
)

// A Hive backup is a .tar.gz of the SQLite file(s) produced by the VACUUM
// INTO primitive — a stable, point-in-time snapshot taken without blocking
// writers. Postgres backends are intentionally out of scope here: operators
// running Postgres have richer native tooling (pg_dump, PITR, WAL shipping)
// and wrapping them would imply owning their backup policy.

var backupCmd = &cobra.Command{
	Use:   "backup [file]",
	Short: "Snapshot the Hive SQLite database to a .tar.gz file",
	Long: `Create a point-in-time backup of the Hive SQLite database.

Uses SQLite VACUUM INTO to produce a consistent copy without blocking
writers, then wraps the resulting file in a .tar.gz archive. Postgres
storage is NOT backed up by this command — use your Postgres-native
tooling (pg_dump, PITR, WAL shipping) for that.

  hive backup hive-2026-04-17.tar.gz
  hive backup                           # writes to hive-backup-<ts>.tar.gz`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load("hive.yaml")
		if err != nil {
			return err
		}
		if strings.EqualFold(cfg.Storage, "postgres") {
			return fmt.Errorf("hive backup only supports the SQLite backend; use pg_dump for Postgres deployments")
		}

		out := ""
		if len(args) > 0 {
			out = args[0]
		}
		if out == "" {
			out = filepath.Join(".", fmt.Sprintf("hive-backup-%s.tar.gz",
				strings.ReplaceAll(timestamp(), ":", "-")))
		}

		store, err := storage.Open(cfg.DataDir)
		if err != nil {
			return err
		}
		defer store.Close()

		// VACUUM INTO to a tempfile so we capture a consistent snapshot
		// rather than a copy-through-fs that could be mid-write.
		tmp, err := os.CreateTemp("", "hive-vacuum-*.db")
		if err != nil {
			return fmt.Errorf("creating temp file: %w", err)
		}
		tmpPath := tmp.Name()
		_ = tmp.Close()
		_ = os.Remove(tmpPath) // VACUUM INTO must target a non-existent path
		defer func() { _ = os.Remove(tmpPath) }()

		if _, err := store.DB.ExecContext(context.Background(),
			fmt.Sprintf("VACUUM INTO %q", tmpPath)); err != nil {
			return fmt.Errorf("VACUUM INTO: %w", err)
		}

		if err := writeTarGz(out, tmpPath); err != nil {
			return err
		}
		fmt.Printf("✓ backup → %s\n", out)
		return nil
	},
}

var restoreCmd = &cobra.Command{
	Use:   "restore <file>",
	Short: "Restore a Hive SQLite database from a .tar.gz backup",
	Long: `Restore a Hive SQLite database from a backup produced by ` + "`hive backup`" + `.

Refuses to overwrite an existing database unless --force is set.
Stop any running Hive server first — this writes to the data directory.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		cfg, err := config.Load("hive.yaml")
		if err != nil {
			return err
		}
		if strings.EqualFold(cfg.Storage, "postgres") {
			return fmt.Errorf("hive restore only supports the SQLite backend")
		}

		dbPath := filepath.Join(cfg.DataDir, "hive.db")
		if _, err := os.Stat(dbPath); err == nil && !force {
			return fmt.Errorf("refusing to overwrite %s (pass --force to accept data loss)", dbPath)
		}

		if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
			return fmt.Errorf("preparing data dir: %w", err)
		}

		if err := extractTarGz(args[0], cfg.DataDir); err != nil {
			return err
		}
		fmt.Printf("✓ restored into %s — start `hive serve` to verify\n", cfg.DataDir)
		return nil
	},
}

// writeTarGz packages a single db file into a .tar.gz archive. Using tar
// rather than plain gzip leaves room for multi-file backups later (e.g.
// bundling the config + db), without breaking format for existing users.
func writeTarGz(outPath, dbPath string) error {
	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("creating %s: %w", outPath, err)
	}
	defer out.Close()
	gz := gzip.NewWriter(out)
	defer func() { _ = gz.Close() }()
	tw := tar.NewWriter(gz)
	defer func() { _ = tw.Close() }()

	info, err := os.Stat(dbPath)
	if err != nil {
		return err
	}
	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	hdr.Name = "hive.db"
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	f, err := os.Open(dbPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(tw, f); err != nil {
		return fmt.Errorf("copying db into archive: %w", err)
	}
	return nil
}

// extractTarGz is the inverse of writeTarGz. Only accepts entries named
// "hive.db" — anything else is rejected so a malicious archive can't
// traverse into a parent directory.
func extractTarGz(archivePath, destDir string) error {
	in, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("opening %s: %w", archivePath, err)
	}
	defer in.Close()
	gz, err := gzip.NewReader(in)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		// Guard against path traversal. Only the exact filename is allowed.
		if h.Name != "hive.db" {
			return fmt.Errorf("unexpected entry %q in archive", h.Name)
		}
		dest := filepath.Join(destDir, "hive.db")
		f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return fmt.Errorf("opening %s: %w", dest, err)
		}
		// LimitReader protects against decompression bombs. 8GiB ceiling
		// is generous for any real Hive deployment; bump if needed.
		if _, err := io.Copy(f, io.LimitReader(tr, 8<<30)); err != nil {
			_ = f.Close()
			return fmt.Errorf("writing %s: %w", dest, err)
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
	return nil
}

func timestamp() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05")
}

func init() {
	restoreCmd.Flags().Bool("force", false, "overwrite an existing hive.db")
	rootCmd.AddCommand(backupCmd)
	rootCmd.AddCommand(restoreCmd)
}
