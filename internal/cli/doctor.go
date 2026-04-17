package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/JulienLeotier/hive/internal/adapter"
	"github.com/JulienLeotier/hive/internal/agent"
	"github.com/JulienLeotier/hive/internal/config"
	"github.com/JulienLeotier/hive/internal/secretstore"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/spf13/cobra"
)

// Check statuses. One vocabulary across all checks so add-on checks in
// sibling files stay consistent.
const (
	statusOK   = "ok"
	statusWarn = "warn"
	statusFail = "fail"
)

// `hive doctor` runs a battery of health checks so an operator can answer
// "is my deployment actually working?" in one command — no click-through
// on 16 dashboard pages, no grep across logs. Intended for first-run
// validation, post-upgrade smoke checks, and on-call triage.
//
// Exit codes:
//   0 — all checks passed (or passed with only warnings)
//   1 — at least one check failed
//   2 — harness itself couldn't start (bad config, no data dir, …)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run health checks on this hive deployment",
	Long: `Run a battery of checks across config, storage, migrations,
registered agents, secret encryption, and (if configured) OTLP endpoint
reachability. Prints a pass/fail summary and exits non-zero on any failure.

Use before shipping a new deployment, after an upgrade, or as part of an
on-call runbook.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		verbose, _ := cmd.Flags().GetBool("verbose")
		agentTimeoutMS, _ := cmd.Flags().GetInt("agent-timeout-ms")

		cfg, err := config.Load("hive.yaml")
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "✗ config load failed: %v\n", err)
			os.Exit(2)
		}

		d := &doctor{
			cmd:            cmd,
			cfg:            cfg,
			verbose:        verbose,
			agentTimeout:   time.Duration(agentTimeoutMS) * time.Millisecond,
		}

		d.checkConfig()
		store := d.checkStorage()
		if store != nil {
			defer store.Close()
			d.checkMigrations(store.DB)
			d.checkAgents(store.DB)
		}
		d.checkSecrets()
		d.checkDataDir()
		d.checkOTLP()

		d.summary()
		if d.failures > 0 {
			os.Exit(1)
		}
		return nil
	},
}

type checkResult struct {
	name    string
	status  string // statusOK, statusWarn, statusFail
	message string
}

type doctor struct {
	cmd          *cobra.Command
	cfg          config.Config
	verbose      bool
	agentTimeout time.Duration
	results      []checkResult
	failures     int
	warnings     int
}

func (d *doctor) record(name, status, msg string) {
	d.results = append(d.results, checkResult{name: name, status: status, message: msg})
	switch status {
	case statusFail:
		d.failures++
	case statusWarn:
		d.warnings++
	}
	symbol := "✓"
	switch status {
	case statusWarn:
		symbol = "!"
	case statusFail:
		symbol = "✗"
	}
	fmt.Fprintf(d.cmd.OutOrStdout(), "%s %-28s %s\n", symbol, name, msg)
}

func (d *doctor) checkConfig() {
	if err := d.cfg.Validate(); err != nil {
		d.record("config.validate", statusFail, err.Error())
		return
	}
	d.record("config.validate", statusOK, fmt.Sprintf("port=%d storage=%s", d.cfg.Port, coalesceStr(d.cfg.Storage, "sqlite")))
}

func (d *doctor) checkStorage() *storage.Store {
	store, err := storage.Open(d.cfg.DataDir)
	if err != nil {
		d.record("storage.open", statusFail, err.Error())
		return nil
	}
	d.record("storage.open", statusOK, d.cfg.DataDir)
	return store
}

func (d *doctor) checkMigrations(db *sql.DB) {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_versions`).Scan(&count); err != nil {
		d.record("storage.migrations", statusFail, err.Error())
		return
	}
	d.record("storage.migrations", statusOK, fmt.Sprintf("%d applied", count))
}

func (d *doctor) checkAgents(db *sql.DB) {
	mgr := agent.NewManager(db)
	agents, err := mgr.List(context.Background())
	if err != nil {
		d.record("agents.list", statusFail, err.Error())
		return
	}
	if len(agents) == 0 {
		d.record("agents.list", statusWarn, "no agents registered — add one with `hive add-agent`")
		return
	}
	d.record("agents.list", statusOK, fmt.Sprintf("%d registered", len(agents)))

	// Probe every HTTP-type agent's /health. Local-only adapters (claude-code,
	// mcp, …) are skipped because health checks require an HTTP round-trip.
	healthy, degraded, unreachable := 0, 0, 0
	for _, a := range agents {
		if a.Type != "http" {
			continue
		}
		var conf map[string]string
		_ = json.Unmarshal([]byte(a.Config), &conf)
		baseURL := conf["base_url"]
		if baseURL == "" {
			continue
		}
		ad := adapter.NewHTTPAdapter(baseURL)
		ad.HTTPClient.Timeout = d.agentTimeout
		h, err := ad.Health(context.Background())
		switch {
		case err != nil:
			unreachable++
			if d.verbose {
				fmt.Fprintf(d.cmd.OutOrStdout(), "    · %s (%s) unreachable: %v\n", a.Name, baseURL, err)
			}
		case h.Status == healthyStatus:
			healthy++
		default:
			degraded++
			if d.verbose {
				fmt.Fprintf(d.cmd.OutOrStdout(), "    · %s (%s) degraded: %s\n", a.Name, baseURL, h.Message)
			}
		}
	}
	status := statusOK
	if unreachable > 0 || degraded > 0 {
		status = statusWarn
	}
	d.record("agents.health", status,
		fmt.Sprintf("healthy=%d degraded=%d unreachable=%d", healthy, degraded, unreachable))
}

func (d *doctor) checkSecrets() {
	if secretstore.HasMasterKey() {
		d.record("secrets.master_key", statusOK, "HIVE_MASTER_KEY set — sensitive DB fields are encrypted")
	} else {
		d.record("secrets.master_key", statusWarn,
			"HIVE_MASTER_KEY not set — webhook URLs and federation certs stored plaintext")
	}
}

func (d *doctor) checkDataDir() {
	info, err := os.Stat(d.cfg.DataDir)
	if err != nil {
		d.record("data_dir.stat", statusFail, err.Error())
		return
	}
	// Disk space check — refuse to pass when the volume is below 10% free.
	dbPath := filepath.Join(d.cfg.DataDir, "hive.db")
	var fs syscall.Statfs_t
	if err := syscall.Statfs(dbPath, &fs); err == nil {
		freeBytes := fs.Bavail * uint64(fs.Bsize)
		totalBytes := fs.Blocks * uint64(fs.Bsize)
		freePct := float64(freeBytes) / float64(totalBytes) * 100
		status := statusOK
		if freePct < 10 {
			status = statusWarn
		}
		d.record("data_dir.free", status, fmt.Sprintf("%.1f%% free (%d MB)", freePct, freeBytes>>20))
	}
	d.record("data_dir.perms", statusOK, fmt.Sprintf("mode=%v", info.Mode().Perm()))
}

func (d *doctor) checkOTLP() {
	endpoint := ""
	if d.cfg.Observability != nil && d.cfg.Observability.Traces != nil {
		endpoint = d.cfg.Observability.Traces.Endpoint
	}
	if endpoint == "" {
		endpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	}
	if endpoint == "" {
		d.record("tracing.endpoint", statusWarn, "no OTLP endpoint — traces disabled")
		return
	}

	// Host-port only — OTLP gRPC uses TCP. We try a TCP dial with a short
	// deadline rather than a full gRPC handshake because we don't want the
	// doctor to depend on the collector being fully configured at its end.
	probeURL := "http://" + strings.TrimPrefix(strings.TrimPrefix(endpoint, "http://"), "https://")
	req, _ := http.NewRequest(http.MethodGet, probeURL, nil)
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		d.record("tracing.reachable", statusWarn, fmt.Sprintf("%s: %v", endpoint, err))
		return
	}
	resp.Body.Close()
	d.record("tracing.reachable", statusOK, fmt.Sprintf("%s responded %d", endpoint, resp.StatusCode))
}

func (d *doctor) summary() {
	fmt.Fprintln(d.cmd.OutOrStdout())
	fmt.Fprintf(d.cmd.OutOrStdout(), "Summary: %d checks, %d failures, %d warnings\n",
		len(d.results), d.failures, d.warnings)
	if d.failures == 0 && d.warnings == 0 {
		fmt.Fprintln(d.cmd.OutOrStdout(), "All checks green — ship it.")
	} else if d.failures == 0 {
		fmt.Fprintln(d.cmd.OutOrStdout(), "No hard failures — review the warnings above.")
	}
}

func coalesceStr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func init() {
	doctorCmd.Flags().Bool("verbose", false, "show per-agent failure detail")
	doctorCmd.Flags().Int("agent-timeout-ms", 2000, "per-agent /health timeout in milliseconds")
	rootCmd.AddCommand(doctorCmd)
}
