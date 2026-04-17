// Package devloop runs the BMAD execution cycle: pick the next pending
// story on a building project, dispatch it to the Dev agent, send the
// output to the Reviewer agent, check each acceptance criterion, iterate
// on failure, mark the story done on success. When every story of a
// project has passed, the project flips to `shipped`.
//
// Agent implementations ship in two flavours:
//
//   - Scripted — deterministic, produces predictable outputs and always
//     passes ACs on first try. Used in CI and as the safety net when the
//     Claude CLI is unreachable so a build never dead-ends.
//   - ClaudeCode — invokes the local `claude` CLI in the project workdir
//     (or repo_path when set). The Dev agent asks Claude to write code
//     satisfying the story + ACs; the Reviewer agent asks Claude to
//     evaluate the diff against each AC.
//
// The Supervisor is a goroutine that polls for projects in `building`
// state every N seconds and advances one story at a time per project.
// Slow iteration by design — this is a "run once, get a shipped product"
// flow, not a realtime system.
package devloop

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/JulienLeotier/hive/internal/bmad"
)

// Project lifecycle strings (mirrored here so the package doesn't import
// internal/project and avoid cycles if project later imports devloop).
const (
	projectStatusBuilding = "building"
	projectStatusShipped  = "shipped"
)

// Story statuses.
const (
	storyStatusPending = "pending"
	storyStatusDev     = "dev"
	storyStatusReview  = "review"
	storyStatusDone    = "done"
	storyStatusBlocked = "blocked"
)

// MaxIterations caps dev/review cycles per story before we give up and
// flip the story to `blocked`. Without a cap a flaky reviewer could
// loop forever and burn Claude tokens.
const MaxIterations = 3

// Story is the minimal story shape devloop needs. Mirror of the stories
// table (+ ACs eagerly loaded) so agents don't have to poke at SQL.
// Branch/PRURL are populated when a prior iteration of the same story
// already opened a PR — the dev skill uses that to push follow-up
// commits onto the existing branch instead of opening a second PR.
type Story struct {
	ID          string
	EpicID      string
	ProjectID   string
	Title       string
	Description string
	Iterations  int
	Status      string
	Branch      string
	PRURL       string
	ACs         []AcceptanceCriterion
}

// AcceptanceCriterion is one verifiable requirement.
type AcceptanceCriterion struct {
	ID     int64
	Text   string
	Passed bool
}

// ProjectContext carries the few project-level fields the agents need
// (workdir + repo_path + idea). Built once per supervisor tick.
type ProjectContext struct {
	ID       string
	Idea     string
	PRD      string
	Workdir  string
	RepoPath string
}

// DevOutput is what Dev returns to the Reviewer.
type DevOutput struct {
	Summary      string   // one-line summary of what was done
	Details      string   // full explanation for the reviewer to check against ACs
	Diff         string   // unified diff when code was modified (optional)
	FilesTouched []string // paths the agent touched (optional)
	// Branch + PRURL are populated when the BMAD dev skill opened a
	// feature branch / pull-request during the iteration. Hive tracks
	// both so the dashboard can link out to the PR and the reviewer
	// can focus on the diff.
	Branch string
	PRURL  string
	// PreSprintStatus est la snapshot de development_status dans
	// sprint-status.yaml capturée JUSTE AVANT que le Dev tourne. Le
	// Reviewer diffe cette snapshot avec l'état post-review pour
	// identifier la story que BMAD vient de traiter et en déduire le
	// verdict (ready-for-done ou renvoyée en ready-for-dev).
	PreSprintStatus map[string]string
}

// DevAgent implementations produce code / artefacts for a story.
type DevAgent interface {
	Name() string
	Develop(ctx context.Context, proj ProjectContext, story Story, iteration int, reviewFeedback string) (DevOutput, error)
}

// ReviewVerdict is the Reviewer's call on one story iteration.
type ReviewVerdict struct {
	Pass     bool
	Feedback string              // free-form notes for the dev on failure
	ACs      []ReviewedCriterion // one-to-one with story.ACs
}

// ReviewedCriterion pairs an AC with the reviewer's pass/fail call.
type ReviewedCriterion struct {
	ID     int64
	Passed bool
	Reason string
}

// ReviewerAgent decides whether a Dev iteration meets every AC.
type ReviewerAgent interface {
	Name() string
	Review(ctx context.Context, proj ProjectContext, story Story, output DevOutput) (ReviewVerdict, error)
}

// Publisher is the minimum event-bus surface the supervisor needs to
// broadcast state transitions. Matches event.Bus.PublishErr. Kept as a
// callback so this package doesn't import event and avoids cycle risk.
type Publisher func(ctx context.Context, eventType, source string, payload any) error

// Supervisor drives the dev→review loop. Kept simple: one goroutine,
// polls on a tick, advances one story per project per tick. Parallelism
// across projects is fine (independent state) but stories within a
// project are strictly sequential — Foundations must land before Core
// Flows touch the same files.
type Supervisor struct {
	db       *sql.DB
	dev      DevAgent
	reviewer ReviewerAgent
	git      *GitCommitter
	publish  Publisher
	interval time.Duration
}

// NewSupervisor builds a supervisor. Pass the deterministic Scripted
// agents in tests and the Claude-backed ones in production. interval=0
// defaults to 10s. Git committer auto-detected from PATH.
func NewSupervisor(db *sql.DB, dev DevAgent, reviewer ReviewerAgent, interval time.Duration) *Supervisor {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	return &Supervisor{
		db: db, dev: dev, reviewer: reviewer,
		git:      NewGitCommitter(),
		interval: interval,
	}
}

// WithPublisher wires an event-bus publisher so the supervisor broadcasts
// story.dev_started, story.reviewed, story.blocked, and project.shipped
// events. Without it, the supervisor still runs but the dashboard
// WebSocket won't light up in real time.
func (s *Supervisor) WithPublisher(p Publisher) *Supervisor {
	s.publish = p
	return s
}

// WithGit overrides the auto-detected git committer. Pass nil in tests
// that don't want to touch the filesystem with real git state.
func (s *Supervisor) WithGit(g *GitCommitter) *Supervisor {
	s.git = g
	return s
}

// emit publishes an event if a bus is wired. Silently drops errors —
// event delivery is nice-to-have, not essential.
func (s *Supervisor) emit(ctx context.Context, eventType string, payload any) {
	if s.publish == nil {
		return
	}
	_ = s.publish(ctx, eventType, "devloop", payload)
}

// Start runs the supervisor loop until ctx is cancelled. Non-blocking on
// the caller — spins a goroutine.
func (s *Supervisor) Start(ctx context.Context) {
	// Crash recovery: any story left in dev/review belongs to an
	// iteration that was in flight when the last supervisor went down.
	// The dev/review handlers are not durable — on restart they'd never
	// resume and the project would wedge. Rewind those stories to
	// pending (keeping their iteration counter) so we retry them on the
	// next tick. We don't touch iteration counts because the work may
	// have partially landed on disk and the reviewer will catch it.
	if _, err := s.db.ExecContext(ctx,
		`UPDATE stories
		 SET status = ?, updated_at = datetime('now')
		 WHERE status IN (?, ?)
		   AND epic_id IN (SELECT id FROM epics WHERE project_id IN (
		       SELECT id FROM projects WHERE status = ?
		   ))`,
		storyStatusPending, storyStatusDev, storyStatusReview,
		projectStatusBuilding,
	); err != nil {
		slog.Warn("devloop: crash-recovery sweep failed", "error", err)
	}
	go func() {
		t := time.NewTicker(s.interval)
		defer t.Stop()
		// Kick once immediately so operators don't wait a full interval
		// to see the first story advance after finalising the PRD.
		s.tick(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.tick(ctx)
			}
		}
	}()
}

// maxParallelProjects cappe le nombre de projets qu'on avance en
// parallèle à chaque tick. Chaque project.advance déclenche plusieurs
// `claude --print` (create-story, dev-story, code-review) ; laisser
// N projets tourner à la fois évite de saturer la machine + les
// crédits Claude. Override via HIVE_MAX_PARALLEL_PROJECTS si besoin.
const maxParallelProjects = 3

// tick fait avancer plusieurs projets en parallèle (borné par
// maxParallelProjects). Les stories d'un même projet restent
// séquentielles — Hive ne lance pas deux dev-story concurrents sur
// le même sprint-status.yaml. Entre projets, parallélisme sans
// synchro : chaque projet a son workdir.
func (s *Supervisor) tick(ctx context.Context) {
	projects, err := s.buildingProjects(ctx)
	if err != nil {
		slog.Warn("devloop: listing building projects", "error", err)
		return
	}
	if len(projects) == 0 {
		return
	}
	sem := make(chan struct{}, maxParallelProjects)
	var wg sync.WaitGroup
	for _, p := range projects {
		wg.Add(1)
		go func(p ProjectContext) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := s.advance(ctx, p); err != nil {
				slog.Warn("devloop: advance failed", "project", p.ID, "error", err)
			}
		}(p)
	}
	wg.Wait()
}

// advance picks the next pending story for the project, runs one
// dev→review iteration, and updates persistence. If there's no pending
// story left, the project graduates to `shipped`.
func (s *Supervisor) advance(ctx context.Context, proj ProjectContext) error {
	story, err := s.nextStory(ctx, proj.ID)
	if err != nil {
		return err
	}
	if story == nil {
		// No unfinished story → the project is complete. Flip it once.
		res, err := s.db.ExecContext(ctx,
			`UPDATE projects SET status = ?, updated_at = datetime('now')
			 WHERE id = ? AND status = ?`,
			projectStatusShipped, proj.ID, projectStatusBuilding,
		)
		if err != nil {
			return fmt.Errorf("marking project shipped: %w", err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			slog.Info("devloop: project shipped", "project", proj.ID)
			s.emit(ctx, "project.shipped", map[string]string{"project_id": proj.ID})
		}
		return nil
	}

	// Mark story `dev` so dashboards can show which one is live. Bump
	// iteration counter once per cycle — first iteration is iteration 1.
	newIteration := story.Iterations + 1
	_, err = s.db.ExecContext(ctx,
		`UPDATE stories SET status = ?, iterations = ?, updated_at = datetime('now') WHERE id = ?`,
		storyStatusDev, newIteration, story.ID,
	)
	if err != nil {
		return fmt.Errorf("marking story dev: %w", err)
	}
	s.emit(ctx, "story.dev_started", map[string]any{
		"project_id": proj.ID,
		"story_id":   story.ID,
		"story":      story.Title,
		"iteration":  newIteration,
	})

	// Ensure the workdir is a git repo before the dev touches it so the
	// story commit has a place to land.
	workdir := proj.Workdir
	if workdir == "" {
		workdir = proj.RepoPath
	}
	if workdir != "" && s.git != nil {
		if err := s.git.EnsureRepo(ctx, workdir); err != nil {
			slog.Warn("devloop: git init failed — continuing without version control",
				"project", proj.ID, "error", err)
		}
	}

	feedback := s.previousFeedback(ctx, story.ID)
	output, err := s.dev.Develop(ctx, proj, *story, newIteration, feedback)
	if err != nil {
		return fmt.Errorf("dev: %w", err)
	}

	// Si BMAD a ouvert une branche / PR pendant l'itération, on les
	// persiste sur la story pour que le dashboard puisse les afficher
	// et que l'itération suivante voie qu'une PR existe déjà.
	if output.Branch != "" || output.PRURL != "" {
		_, _ = s.db.ExecContext(ctx,
			`UPDATE stories SET branch = COALESCE(NULLIF(?, ''), branch),
			                     pr_url = COALESCE(NULLIF(?, ''), pr_url),
			                     updated_at = datetime('now')
			 WHERE id = ?`,
			output.Branch, output.PRURL, story.ID)
		if output.PRURL != "" {
			s.emit(ctx, "story.pr_created", map[string]any{
				"project_id": proj.ID,
				"story_id":   story.ID,
				"story":      story.Title,
				"pr_url":     output.PRURL,
			})
		}
	}

	_, err = s.db.ExecContext(ctx,
		`UPDATE stories SET status = ? WHERE id = ?`,
		storyStatusReview, story.ID,
	)
	if err != nil {
		return fmt.Errorf("marking story review: %w", err)
	}

	verdict, err := s.reviewer.Review(ctx, proj, *story, output)
	if err != nil {
		return fmt.Errorf("reviewer: %w", err)
	}

	// Record the review row + each AC update.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	verdictStr := "pass"
	if !verdict.Pass {
		verdictStr = "fail"
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO reviews (story_id, iteration, reviewer_agent_id, verdict, feedback)
		 VALUES (?, ?, ?, ?, ?)`,
		story.ID, newIteration, s.reviewer.Name(), verdictStr, verdict.Feedback,
	)
	if err != nil {
		return err
	}
	for _, rc := range verdict.ACs {
		if rc.Passed {
			_, err = tx.ExecContext(ctx,
				`UPDATE acceptance_criteria SET passed = 1, verified_at = datetime('now'), verified_by = ?
				 WHERE id = ?`,
				s.reviewer.Name(), rc.ID,
			)
		} else {
			_, err = tx.ExecContext(ctx,
				`UPDATE acceptance_criteria SET passed = 0 WHERE id = ?`, rc.ID,
			)
		}
		if err != nil {
			return err
		}
	}
	switch {
	case verdict.Pass:
		_, err = tx.ExecContext(ctx,
			`UPDATE stories SET status = ?, updated_at = datetime('now') WHERE id = ?`,
			storyStatusDone, story.ID,
		)
	case newIteration >= MaxIterations:
		_, err = tx.ExecContext(ctx,
			`UPDATE stories SET status = ?, updated_at = datetime('now') WHERE id = ?`,
			storyStatusBlocked, story.ID,
		)
	default:
		_, err = tx.ExecContext(ctx,
			`UPDATE stories SET status = ?, updated_at = datetime('now') WHERE id = ?`,
			storyStatusPending, story.ID,
		)
	}
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	// BMAD gère lui-même branches/commits/push/PR via bmad-dev-story,
	// donc Hive ne commite plus rien sur main directement. On se
	// contente de déclencher la rétrospective BMAD quand tout l'epic
	// de la story vient d'être complété — doc officielle Phase 4 :
	// "bmad-agent-dev + bmad-retrospective (after epic completion)".
	if verdict.Pass && workdir != "" && s.epicComplete(ctx, story.EpicID) {
		if runner := bmad.NewRunner(); runner != nil {
			//nolint:gosec // G118: retrospective tourne détachée ; le ctx de l'itération meurt avec elle
			go func(rnr *bmad.Runner, wd, pid, eid, title string) {
				rctx, rcancel := context.WithTimeout(context.Background(), 10*time.Minute)
				defer rcancel()
				if _, err := rnr.RunSequence(rctx, wd, bmad.RetrospectiveSequence); err != nil {
					slog.Warn("devloop: bmad-retrospective failed",
						"project", pid, "epic", eid, "error", err)
					return
				}
				s.emit(rctx, "epic.retrospective", map[string]any{
					"project_id": pid,
					"epic_id":    eid,
					"epic_title": title,
				})
			}(runner, workdir, proj.ID, story.EpicID, story.Title)
		}
	}

	slog.Info("devloop: iteration done",
		"project", proj.ID, "story", story.ID, "iteration", newIteration,
		"pass", verdict.Pass)

	evtPayload := map[string]any{
		"project_id": proj.ID,
		"story_id":   story.ID,
		"story":      story.Title,
		"iteration":  newIteration,
		"pass":       verdict.Pass,
		"feedback":   verdict.Feedback,
	}
	if verdict.Pass {
		s.emit(ctx, "story.reviewed", evtPayload)
	} else if newIteration >= MaxIterations {
		s.emit(ctx, "story.blocked", evtPayload)
	} else {
		s.emit(ctx, "story.review_failed", evtPayload)
	}
	return nil
}

// buildingProjects returns the project contexts currently in the
// `building` state.
func (s *Supervisor) buildingProjects(ctx context.Context) ([]ProjectContext, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, idea, COALESCE(prd, ''), COALESCE(workdir, ''), COALESCE(repo_path, '')
		 FROM projects WHERE status = ? ORDER BY created_at ASC`,
		projectStatusBuilding,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProjectContext
	for rows.Next() {
		var p ProjectContext
		if err := rows.Scan(&p.ID, &p.Idea, &p.PRD, &p.Workdir, &p.RepoPath); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// nextStory picks the first pending story for a project, or nil when
// everything is done or blocked. Reads ACs eagerly so the agents can
// see what to satisfy.
func (s *Supervisor) nextStory(ctx context.Context, projectID string) (*Story, error) {
	var story Story
	err := s.db.QueryRowContext(ctx,
		`SELECT s.id, s.epic_id, e.project_id, s.title, COALESCE(s.description, ''),
		        s.iterations, s.status, COALESCE(s.branch, ''), COALESCE(s.pr_url, '')
		 FROM stories s
		 JOIN epics e ON e.id = s.epic_id
		 WHERE e.project_id = ? AND s.status IN (?, ?, ?)
		 ORDER BY e.ordering ASC, s.ordering ASC
		 LIMIT 1`,
		projectID, storyStatusPending, storyStatusDev, storyStatusReview,
	).Scan(&story.ID, &story.EpicID, &story.ProjectID, &story.Title,
		&story.Description, &story.Iterations, &story.Status,
		&story.Branch, &story.PRURL)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	// Load ACs.
	acRows, err := s.db.QueryContext(ctx,
		`SELECT id, text, passed FROM acceptance_criteria WHERE story_id = ? ORDER BY ordering ASC`,
		story.ID,
	)
	if err != nil {
		return nil, err
	}
	defer acRows.Close()
	for acRows.Next() {
		var ac AcceptanceCriterion
		var passed int
		if err := acRows.Scan(&ac.ID, &ac.Text, &passed); err != nil {
			return nil, err
		}
		ac.Passed = passed == 1
		story.ACs = append(story.ACs, ac)
	}
	return &story, acRows.Err()
}

// epicComplete reports whether every story of the given epic is now
// in `done` state. Used to trigger the BMAD retrospective hook.
func (s *Supervisor) epicComplete(ctx context.Context, epicID string) bool {
	var pending int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM stories WHERE epic_id = ? AND status <> ?`,
		epicID, storyStatusDone,
	).Scan(&pending); err != nil {
		return false
	}
	return pending == 0
}

// previousFeedback pulls the latest failing review feedback so the next
// dev iteration knows what to fix. Empty string when no prior review.
func (s *Supervisor) previousFeedback(ctx context.Context, storyID string) string {
	var feedback string
	_ = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(feedback, '') FROM reviews
		 WHERE story_id = ? AND verdict = 'fail'
		 ORDER BY iteration DESC LIMIT 1`,
		storyID,
	).Scan(&feedback)
	return feedback
}
