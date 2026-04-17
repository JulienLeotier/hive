// Package bmad drives the real BMAD-METHOD framework
// (github.com/bmad-code-org/BMAD-METHOD) against a project workdir.
//
// BMAD-METHOD ships as an npm package that installs a set of Claude
// Code skills into a target directory. Once installed, every BMAD
// workflow (bmad-create-prd, bmad-create-architecture,
// bmad-create-epics-and-stories, bmad-dev-story, bmad-code-review, …)
// becomes a Claude Code skill that Claude can execute autonomously
// when prompted. We invoke Claude in the workdir with a prompt that
// names the target skill and tells it to auto-continue past every
// A/P/C menu — without that, `claude --print` (no human in the loop)
// would deadlock at the first menu BMAD shows.
//
// This package is deliberately thin: two primitives, Install + Invoke.
// Everything above (phase orchestration, artifact ingestion) lives in
// internal/api/intake.go where it belongs with the rest of the
// supervisor glue.
package bmad

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Layout constants — where BMAD writes things after a default install
// of `--modules bmm --tools claude-code`. Centralised so future BMAD
// releases that move paths only require updates here.
const (
	PlanningDir      = "_bmad-output/planning-artifacts"
	PRDFile          = "_bmad-output/planning-artifacts/PRD.md"
	PRDFileLower     = "_bmad-output/planning-artifacts/prd.md"
	BMADConfigDir    = "_bmad"
)

// Runner orchestrates BMAD against a single workdir.
type Runner struct {
	cliPath string        // resolved `claude`
	timeout time.Duration // per-invocation cap
}

// NewRunner probes for the `claude` CLI and returns nil when it's
// missing so callers can fall back without every invocation failing.
func NewRunner() *Runner {
	path, err := exec.LookPath("claude")
	if err != nil {
		slog.Info("bmad: claude CLI not on PATH — bmad disabled", "error", err)
		return nil
	}
	return &Runner{cliPath: path, timeout: 15 * time.Minute}
}

// Install runs `npx bmad-method install` inside workdir. Safe to
// call on an already-installed workdir — it detects and upgrades in
// place.
func (r *Runner) Install(ctx context.Context, workdir string) error {
	if r == nil {
		return errors.New("bmad: runner unavailable (claude CLI missing)")
	}
	if workdir == "" {
		return errors.New("bmad: empty workdir")
	}
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		return fmt.Errorf("bmad: prepare workdir: %w", err)
	}
	if _, err := os.Stat(filepath.Join(workdir, BMADConfigDir)); err == nil {
		return nil // already installed
	}
	installCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(installCtx,
		"npx", "--yes", "bmad-method@latest", "install",
		"--directory", workdir,
		"--modules", "bmm",
		"--tools", "claude-code",
		"--yes",
	)
	cmd.Dir = workdir
	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bmad install: %w\noutput: %s", err, truncate(combined.String(), 400))
	}
	if err := pinLanguage(workdir, "Français"); err != nil {
		slog.Warn("bmad: could not pin language to French", "error", err)
	}
	slog.Info("bmad installed", "workdir", workdir)
	return nil
}

// pinLanguage rewrites _bmad/bmm/config.yaml so both
// communication_language and document_output_language are the given
// value. BMAD consults these fields at every workflow activation; the
// installer defaults them to English. Called right after install so
// every skill invocation afterwards sees French.
func pinLanguage(workdir, lang string) error {
	cfgPath := filepath.Join(workdir, "_bmad", "bmm", "config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	out := string(data)
	for _, key := range []string{"communication_language", "document_output_language"} {
		out = replaceYAMLScalar(out, key, lang)
	}
	return os.WriteFile(cfgPath, []byte(out), 0o644)
}

// replaceYAMLScalar replaces the value of a top-level scalar key in a
// simple YAML document. Good enough for the handful of fields BMAD's
// config exposes; not a full parser.
func replaceYAMLScalar(yaml, key, value string) string {
	lines := strings.Split(yaml, "\n")
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, key+":") || strings.HasPrefix(trim, key+" :") {
			lines[i] = key + ": " + value
		}
	}
	return strings.Join(lines, "\n")
}

// Result carries what Invoke returns: the text Claude emitted (useful
// for logs + dashboard activity) and the absolute paths of any
// expected output files that landed on disk.
type Result struct {
	Text    string
	Outputs []string
}

// Invoke runs a BMAD skill end-to-end in non-interactive mode. `goal`
// is the natural-language task description; we wrap it with a
// standard non-interactive contract that tells Claude to auto-continue
// every BMAD menu. `expectedOutputs` are workdir-relative paths the
// skill should produce; we stat them after the run and surface which
// landed so the caller can decide how to proceed.
//
// This deviates from BMAD's interactive-by-design model but is
// necessary for autonomous orchestration, and mirrors how BMAD's own
// CI-friendly invocations are meant to work.
func (r *Runner) Invoke(ctx context.Context, workdir, goal string, expectedOutputs []string) (Result, error) {
	if r == nil {
		return Result{}, errors.New("bmad: runner unavailable")
	}
	callCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	prompt := buildPrompt(goal)
	// --dangerously-skip-permissions: we are a local single-user tool
	// sandboxed to the project workdir. BMAD workflows run go build,
	// go test, npm install, git commit, etc. — acceptEdits auto-signs
	// file writes but still blocks bash tools, which would stall every
	// bmad-dev-story iteration. Skip-permissions is the right call
	// here; if the workdir is ever untrusted the whole BMAD model
	// breaks anyway.
	cmd := exec.CommandContext(callCtx, r.cliPath,
		"--print", "--output-format", "json",
		"--dangerously-skip-permissions")
	cmd.Dir = workdir
	cmd.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return Result{}, fmt.Errorf("claude invoke: %w\nstderr: %s",
			err, truncate(stderr.String(), 300))
	}

	var envelope struct {
		Result  string `json:"result"`
		IsError bool   `json:"is_error"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		return Result{}, fmt.Errorf("parse envelope: %w\nraw: %s",
			err, truncate(stdout.String(), 300))
	}
	if envelope.IsError {
		return Result{Text: envelope.Result},
			fmt.Errorf("skill reported error: %s", truncate(envelope.Result, 300))
	}

	var landed []string
	for _, rel := range expectedOutputs {
		abs := filepath.Join(workdir, rel)
		if info, err := os.Stat(abs); err == nil && !info.IsDir() && info.Size() > 0 {
			landed = append(landed, abs)
		}
	}
	return Result{Text: envelope.Result, Outputs: landed}, nil
}

// buildPrompt wraps the caller's goal with the non-interactive contract
// every BMAD skill needs. Kept in one place so rules stay consistent.
// All human-readable output (summaries, review feedback, logs) MUST
// be in French — the operator is French-speaking; artefacts generated
// by the skills (PRD, code, tests) follow BMAD's own
// document_output_language which we also pin to French via the _bmad
// config at install time.
func buildPrompt(goal string) string {
	var b strings.Builder
	b.WriteString("Tu tournes dans une boucle d'orchestration autonome — il n'y a AUCUN humain pour répondre aux menus. ")
	b.WriteString("Quand un workflow BMAD présente un menu A/P/C ou équivalent, choisis toujours Continue et avance. ")
	b.WriteString("Exécute le workflow d'une traite, jamais de halt. ")
	b.WriteString("Toutes tes réponses textuelles (résumés, feedback, logs) doivent être EN FRANÇAIS. ")
	b.WriteString("Utilise tes outils d'édition directement — les permissions sont déjà accordées. ")
	b.WriteString("À la fin, renvoie un résumé court (moins de 10 lignes, en français) listant les fichiers que tu as produits.\n\n")
	b.WriteString("Tâche :\n")
	b.WriteString(goal)
	return b.String()
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
