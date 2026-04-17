package devloop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ScriptedDev implements DevAgent deterministically. It writes a
// NOTES-style markdown file per story into the project workdir so the
// directory actually accumulates artefacts across a build, and returns
// an output summary the Reviewer can evaluate against the ACs.
//
// Why an on-disk write even for the scripted path: the BMAD contract
// says "the agent builds something". Having the scripted dev leave a
// real file exercises the workdir plumbing end-to-end and gives the
// dashboard / git integration (Phase 4+) something to display.
type ScriptedDev struct{}

// NewScriptedDev returns a deterministic dev agent.
func NewScriptedDev() *ScriptedDev { return &ScriptedDev{} }

// Name tags the agent in reviews + events.
func (*ScriptedDev) Name() string { return "scripted-dev" }

// Develop writes a per-story notes file and returns a summary that
// restates every AC — ScriptedReviewer keys off that so a clean run
// passes on first iteration.
func (d *ScriptedDev) Develop(_ context.Context, proj ProjectContext, story Story, iteration int, feedback string) (DevOutput, error) {
	workdir := pickWorkdir(proj)
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		return DevOutput{}, fmt.Errorf("prepare workdir %s: %w", workdir, err)
	}

	path := filepath.Join(workdir, "STORIES", sanitiseFilename(story.Title)+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return DevOutput{}, err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", story.Title)
	fmt.Fprintf(&b, "_Project: %s · iteration %d_\n\n", proj.ID, iteration)
	if story.Description != "" {
		fmt.Fprintf(&b, "## Description\n\n%s\n\n", story.Description)
	}
	if feedback != "" {
		fmt.Fprintf(&b, "## Applied feedback from previous review\n\n%s\n\n", feedback)
	}
	b.WriteString("## Acceptance criteria addressed\n\n")
	for _, ac := range story.ACs {
		fmt.Fprintf(&b, "- [scripted-dev] %s\n", ac.Text)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return DevOutput{}, fmt.Errorf("write notes: %w", err)
	}

	return DevOutput{
		Summary: fmt.Sprintf("Wrote story notes to %s", path),
		Details: b.String(),
		FilesTouched: []string{path},
	}, nil
}

// ScriptedReviewer pairs with ScriptedDev. It passes every AC that the
// dev's output `Details` explicitly mentions. Because ScriptedDev
// restates every AC in its output, a clean cycle always passes — good
// for CI + happy-path e2e. To simulate a failure, an operator can
// remove an AC from the story between iterations; ScriptedReviewer
// will then flag the missing line.
type ScriptedReviewer struct{}

// NewScriptedReviewer returns a deterministic reviewer.
func NewScriptedReviewer() *ScriptedReviewer { return &ScriptedReviewer{} }

// Name tags the agent in the reviews table.
func (*ScriptedReviewer) Name() string { return "scripted-reviewer" }

// Review checks that every AC text appears in the dev's output details.
func (*ScriptedReviewer) Review(_ context.Context, _ ProjectContext, story Story, output DevOutput) (ReviewVerdict, error) {
	haystack := output.Details + "\n" + output.Summary
	verdict := ReviewVerdict{Pass: true}
	var missing []string
	for _, ac := range story.ACs {
		found := strings.Contains(haystack, ac.Text)
		verdict.ACs = append(verdict.ACs, ReviewedCriterion{
			ID: ac.ID, Passed: found,
			Reason: reviewerReason(found),
		})
		if !found {
			verdict.Pass = false
			missing = append(missing, ac.Text)
		}
	}
	if !verdict.Pass {
		verdict.Feedback = "Dev output didn't cover these ACs:\n- " + strings.Join(missing, "\n- ")
	} else {
		verdict.Feedback = "All ACs satisfied by dev output."
	}
	return verdict, nil
}

func reviewerReason(passed bool) string {
	if passed {
		return "dev output references this AC"
	}
	return "AC text not found in dev output"
}

// pickWorkdir returns the directory the scripted dev will write into.
// Preference: explicit workdir → repo_path → ./hive-builds/<project_id>.
// Falls back so the BMAD flow works before the operator has configured
// anything.
func pickWorkdir(p ProjectContext) string {
	if p.Workdir != "" {
		return p.Workdir
	}
	if p.RepoPath != "" {
		return p.RepoPath
	}
	return filepath.Join(".", "hive-builds", p.ID)
}

// sanitiseFilename trims a title down to a filesystem-friendly token.
func sanitiseFilename(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ', r == '-', r == '_':
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "story"
	}
	if len(out) > 60 {
		out = out[:60]
	}
	return out
}
