package bmad

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeRunner writes a shell stub at path that mimics `claude --print
// --output-format json`: it reads stdin (the prompt) and echoes the
// given envelope JSON to stdout. Lets us exercise Invoke without the
// real Claude CLI.
func fakeRunner(t *testing.T, envelopeJSON string) *Runner {
	t.Helper()
	dir := t.TempDir()
	stub := filepath.Join(dir, "claude")
	script := "#!/bin/sh\ncat > /dev/null\ncat <<'JSON'\n" + envelopeJSON + "\nJSON\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	return &Runner{cliPath: stub, timeout: 10 * time.Second}
}

// failingRunner writes a stub that exits non-zero, so we can verify
// the "claude invoke" error path.
func failingRunner(t *testing.T) *Runner {
	t.Helper()
	dir := t.TempDir()
	stub := filepath.Join(dir, "claude")
	script := "#!/bin/sh\ncat > /dev/null\necho boom >&2\nexit 7\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	return &Runner{cliPath: stub, timeout: 5 * time.Second}
}

func TestNilRunnerInvoke(t *testing.T) {
	var r *Runner
	_, err := r.Invoke(context.Background(), t.TempDir(), "/test", nil)
	if err == nil {
		t.Fatal("nil runner must refuse to invoke")
	}
}

func TestNilRunnerInstall(t *testing.T) {
	var r *Runner
	err := r.Install(context.Background(), t.TempDir())
	if err == nil {
		t.Fatal("nil runner must refuse to install")
	}
}

func TestInstallRejectsEmptyWorkdir(t *testing.T) {
	r := &Runner{cliPath: "/bin/true"}
	if err := r.Install(context.Background(), ""); err == nil {
		t.Fatal("empty workdir should error")
	}
}

func TestInvokeParsesEnvelope(t *testing.T) {
	env := `{"result":"fichiers produits","is_error":false,"total_cost_usd":0.42,"usage":{"input_tokens":123,"output_tokens":456}}`
	r := fakeRunner(t, env)
	res, err := r.Invoke(context.Background(), t.TempDir(), "/bmad-create-prd", nil)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if res.Text != "fichiers produits" {
		t.Errorf("text = %q", res.Text)
	}
	if res.CostUSD != 0.42 {
		t.Errorf("cost = %v", res.CostUSD)
	}
	if res.InputTokens != 123 || res.OutputTokens != 456 {
		t.Errorf("tokens = in=%d out=%d", res.InputTokens, res.OutputTokens)
	}
}

func TestInvokeSurfacesIsError(t *testing.T) {
	env := `{"result":"PRD manquant","is_error":true,"total_cost_usd":0.01,"usage":{"input_tokens":10,"output_tokens":0}}`
	r := fakeRunner(t, env)
	res, err := r.Invoke(context.Background(), t.TempDir(), "/bmad-validate-prd", nil)
	if err == nil {
		t.Fatal("is_error=true must propagate")
	}
	// Cost+tokens must still be returned so callers can bill the partial run.
	if res.CostUSD != 0.01 || res.InputTokens != 10 {
		t.Errorf("partial envelope fields lost: %+v", res)
	}
}

func TestInvokeRejectsNonJSONStdout(t *testing.T) {
	r := fakeRunner(t, "not valid json at all")
	_, err := r.Invoke(context.Background(), t.TempDir(), "/x", nil)
	if err == nil {
		t.Fatal("bad envelope should error")
	}
	if !strings.Contains(err.Error(), "parse envelope") {
		t.Errorf("want parse envelope error, got %v", err)
	}
}

func TestInvokeSurfacesCLICrash(t *testing.T) {
	r := failingRunner(t)
	_, err := r.Invoke(context.Background(), t.TempDir(), "/x", nil)
	if err == nil {
		t.Fatal("non-zero CLI exit should error")
	}
	if !strings.Contains(err.Error(), "claude invoke") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInvokeReportsLandedOutputs(t *testing.T) {
	env := `{"result":"ok","is_error":false,"total_cost_usd":0,"usage":{"input_tokens":0,"output_tokens":0}}`
	r := fakeRunner(t, env)
	wd := t.TempDir()
	// Pre-create one of the expected outputs; leave the other absent.
	if err := os.WriteFile(filepath.Join(wd, "out.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := r.Invoke(context.Background(), wd, "/x", []string{"out.md", "missing.md"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Outputs) != 1 || !strings.HasSuffix(res.Outputs[0], "out.md") {
		t.Errorf("outputs = %v", res.Outputs)
	}
}

func TestRunSequenceObservedOrder(t *testing.T) {
	env := `{"result":"ok","is_error":false,"total_cost_usd":0.01,"usage":{"input_tokens":1,"output_tokens":1}}`
	r := fakeRunner(t, env)
	var starts []int
	var finishes []int
	var startCmds []string
	obs := StepObserver{
		OnStart: func(i, _ int, cmd string, _ context.CancelFunc) {
			starts = append(starts, i)
			startCmds = append(startCmds, cmd)
		},
		OnFinish: func(i, _ int, _ string, _ Result, _ error) {
			finishes = append(finishes, i)
		},
	}
	cmds := []string{"/a", "/b", "/c"}
	history, err := r.RunSequenceObserved(context.Background(), t.TempDir(), cmds, obs)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("history = %d", len(history))
	}
	if got := starts; len(got) != 3 || got[0] != 1 || got[2] != 3 {
		t.Errorf("starts = %v", got)
	}
	if got := finishes; len(got) != 3 || got[0] != 1 || got[2] != 3 {
		t.Errorf("finishes = %v", got)
	}
	if startCmds[0] != "/a" || startCmds[2] != "/c" {
		t.Errorf("startCmds = %v", startCmds)
	}
}

func TestRunSequenceNilRunner(t *testing.T) {
	var r *Runner
	_, err := r.RunSequence(context.Background(), t.TempDir(), []string{"/x"})
	if err == nil {
		t.Fatal("nil runner should error")
	}
}

func TestRunSequenceStopsOnError(t *testing.T) {
	r := failingRunner(t)
	var hits int32
	obs := StepObserver{
		OnFinish: func(_, _ int, _ string, _ Result, err error) {
			if err != nil {
				atomic.AddInt32(&hits, 1)
			}
		},
	}
	_, err := r.RunSequenceObserved(context.Background(), t.TempDir(),
		[]string{"/a", "/b"}, obs)
	if err == nil {
		t.Fatal("failure must bubble up")
	}
	// Only /a should have run before the abort.
	if atomic.LoadInt32(&hits) != 1 {
		t.Errorf("expected 1 finish with error, got %d", hits)
	}
}

func TestReadSprintStatusMissing(t *testing.T) {
	wd := t.TempDir()
	s, err := ReadSprintStatus(wd)
	if err != nil {
		t.Fatalf("missing file should not error, got %v", err)
	}
	if s != nil {
		t.Fatalf("want nil, got %+v", s)
	}
}

func TestReadSprintStatusParses(t *testing.T) {
	wd := t.TempDir()
	dir := filepath.Join(wd, "_bmad-output", "implementation-artifacts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := `last_updated: 2026-04-18
development_status:
  "1.1": ready-for-done
  "1.2": in-progress
  "2.1": ready-for-dev
`
	if err := os.WriteFile(filepath.Join(dir, "sprint-status.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := ReadSprintStatus(wd)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.StoryStatus("1.1") != "ready-for-done" {
		t.Errorf("1.1 status = %q", s.StoryStatus("1.1"))
	}
	if s.StoryStatus("2.1") != "ready-for-dev" {
		t.Errorf("2.1 status = %q", s.StoryStatus("2.1"))
	}
	if s.LastUpdated != "2026-04-18" {
		t.Errorf("last_updated = %q", s.LastUpdated)
	}
}

func TestReadStoryFileWithFrontMatter(t *testing.T) {
	wd := t.TempDir()
	dir := filepath.Join(wd, "_bmad-output", "implementation-artifacts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `---
story_key: "1.1"
branch: feat/hello
pr_url: https://github.com/me/repo/pull/42
status: review
---
# Story body
Here is the description.
`
	if err := os.WriteFile(filepath.Join(dir, "1.1.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	sf, err := ReadStoryFile(wd, "1.1")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if sf.StoryKey != "1.1" {
		t.Errorf("story_key = %q", sf.StoryKey)
	}
	if sf.PRURL != "https://github.com/me/repo/pull/42" {
		t.Errorf("pr_url = %q", sf.PRURL)
	}
	if sf.Status != "review" {
		t.Errorf("status = %q", sf.Status)
	}
}

func TestReadStoryFileMissingReturnsNil(t *testing.T) {
	wd := t.TempDir()
	sf, err := ReadStoryFile(wd, "99.9")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if sf != nil {
		t.Fatalf("missing story should yield nil")
	}
}

func TestReadStoryFileNoFrontMatterIsBestEffort(t *testing.T) {
	wd := t.TempDir()
	dir := filepath.Join(wd, "_bmad-output", "implementation-artifacts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2.1.md"), []byte("# body only"), 0o644); err != nil {
		t.Fatal(err)
	}
	sf, err := ReadStoryFile(wd, "2.1")
	if err != nil {
		t.Fatalf("best-effort should not error: %v", err)
	}
	if sf == nil {
		t.Fatal("want empty StoryFile, got nil")
	}
}

func TestReplaceYAMLScalar(t *testing.T) {
	in := `communication_language: English
document_output_language: English
project_name: demo
`
	out := replaceYAMLScalar(in, "communication_language", "Français")
	out = replaceYAMLScalar(out, "document_output_language", "Français")
	if !strings.Contains(out, "communication_language: Français") {
		t.Errorf("communication_language not replaced: %s", out)
	}
	if !strings.Contains(out, "document_output_language: Français") {
		t.Errorf("document_output_language not replaced: %s", out)
	}
	// Unrelated keys untouched.
	if !strings.Contains(out, "project_name: demo") {
		t.Errorf("unrelated key got rewritten")
	}
}

func TestReplaceYAMLScalarMissingKeyNoop(t *testing.T) {
	in := "foo: bar\n"
	out := replaceYAMLScalar(in, "missing_key", "value")
	if out != in {
		t.Errorf("missing key should be a noop: %q", out)
	}
}

func TestBuildPromptHasContract(t *testing.T) {
	p := buildPrompt("ma tâche")
	for _, want := range []string{"FRANÇAIS", "permissions", "ma tâche"} {
		if !strings.Contains(p, want) {
			t.Errorf("buildPrompt missing %q:\n%s", want, p)
		}
	}
}

func TestTruncate(t *testing.T) {
	s := strings.Repeat("a", 50)
	if got := truncate(s, 10); len(got) != 10+len("…") {
		t.Errorf("truncate len = %d", len(got))
	}
	if truncate("ok", 50) != "ok" {
		t.Error("short string should be untouched")
	}
}
