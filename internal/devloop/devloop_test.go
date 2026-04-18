package devloop

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedProject inserts a minimal project + epic + story + ACs so the
// supervisor has something to chew on. Returns the project id so tests
// can poll its status.
func seedProject(t *testing.T, db *sql.DB, workdir string) string {
	t.Helper()
	projectID := "prj_test_" + t.Name()
	epicID := "epc_1"
	storyID := "sty_1"

	_, err := db.Exec(
		`INSERT INTO projects (id, name, idea, prd, workdir, status, tenant_id)
		 VALUES (?, 'demo', 'writers app', 'stub PRD', ?, 'building', 'default')`,
		projectID, workdir,
	)
	require.NoError(t, err)
	_, err = db.Exec(
		`INSERT INTO epics (id, project_id, title, ordering, status)
		 VALUES (?, ?, 'Foundations', 0, 'pending')`,
		epicID, projectID,
	)
	require.NoError(t, err)
	_, err = db.Exec(
		`INSERT INTO stories (id, epic_id, title, description, ordering, status)
		 VALUES (?, ?, 'Scaffold', 'minimum scaffold', 0, 'pending')`,
		storyID, epicID,
	)
	require.NoError(t, err)
	_, err = db.Exec(
		`INSERT INTO acceptance_criteria (story_id, ordering, text, passed)
		 VALUES (?, 0, 'A fresh clone builds', 0),
		        (?, 1, 'README documents how to run', 0)`,
		storyID, storyID,
	)
	require.NoError(t, err)
	return projectID
}

func TestSupervisorAdvancesStoryToDoneOnHappyPath(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()
	workdir := filepath.Join(t.TempDir(), "work")
	projectID := seedProject(t, st.DB, workdir)

	sup := NewSupervisor(st.DB, NewScriptedDev(), NewScriptedReviewer(), time.Second)
	// Drive one tick manually — no goroutine.
	sup.tick(context.Background())

	// Story should be done (ScriptedDev writes a notes file listing every
	// AC text, which ScriptedReviewer passes).
	var storyStatus string
	require.NoError(t, st.DB.QueryRow(`SELECT status FROM stories WHERE id = 'sty_1'`).Scan(&storyStatus))
	assert.Equal(t, storyStatusDone, storyStatus)

	var iterations int
	require.NoError(t, st.DB.QueryRow(`SELECT iterations FROM stories WHERE id = 'sty_1'`).Scan(&iterations))
	assert.Equal(t, 1, iterations, "happy path is one iteration")

	// Every AC passed.
	var passed int
	require.NoError(t, st.DB.QueryRow(
		`SELECT COUNT(*) FROM acceptance_criteria WHERE story_id = 'sty_1' AND passed = 1`,
	).Scan(&passed))
	assert.Equal(t, 2, passed, "both ACs verified")

	// Since that was the only story, next tick should flip the project.
	sup.tick(context.Background())
	var projStatus string
	require.NoError(t, st.DB.QueryRow(`SELECT status FROM projects WHERE id = ?`, projectID).Scan(&projStatus))
	assert.Equal(t, projectStatusShipped, projStatus)
}

// failingReviewer always returns fail + a fixed feedback to exercise
// the iteration cap.
type failingReviewer struct{}

func (*failingReviewer) Name() string { return "always-fail" }
func (*failingReviewer) Review(_ context.Context, _ ProjectContext, story Story, _ DevOutput) (ReviewVerdict, error) {
	v := ReviewVerdict{Pass: false, Feedback: "nope"}
	for _, ac := range story.ACs {
		v.ACs = append(v.ACs, ReviewedCriterion{ID: ac.ID, Passed: false, Reason: "nope"})
	}
	return v, nil
}

func TestSupervisorBlocksStoryAfterMaxIterations(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()
	workdir := filepath.Join(t.TempDir(), "work")
	_ = seedProject(t, st.DB, workdir)

	sup := NewSupervisor(st.DB, NewScriptedDev(), &failingReviewer{}, time.Second)
	for i := 0; i < MaxIterations+1; i++ {
		sup.tick(context.Background())
	}

	var status string
	var iterations int
	require.NoError(t, st.DB.QueryRow(
		`SELECT status, iterations FROM stories WHERE id = 'sty_1'`,
	).Scan(&status, &iterations))
	assert.Equal(t, storyStatusBlocked, status, "story must be blocked after max iterations")
	assert.Equal(t, MaxIterations, iterations, "iteration counter capped at MaxIterations")
}

// TestSupervisorRecoversStuckStoriesOnStart models a crash mid-review:
// a story was left in status=`review` for a project in `building`. The
// supervisor must rewind it to pending on Start so the next tick picks
// it up, instead of leaving the project wedged forever.
func TestSupervisorRecoversStuckStoriesOnStart(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()

	workdir := filepath.Join(t.TempDir(), "work")
	_ = seedProject(t, st.DB, workdir)
	// Force the seeded story into `review` to mimic a crashed in-flight
	// iteration.
	_, err = st.DB.Exec(`UPDATE stories SET status = 'review', iterations = 1 WHERE id = 'sty_1'`)
	require.NoError(t, err)

	sup := NewSupervisor(st.DB, NewScriptedDev(), NewScriptedReviewer(), time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sup.Start(ctx)
	// Give the goroutine a moment to run the sweep + first tick. The
	// sweep runs synchronously before the goroutine spawns, so by now
	// the story is either still pending (if the tick hasn't fired yet)
	// or already done (if it has). Both outcomes prove recovery worked.
	time.Sleep(200 * time.Millisecond)

	var status string
	require.NoError(t, st.DB.QueryRow(`SELECT status FROM stories WHERE id = 'sty_1'`).Scan(&status))
	assert.Contains(t, []string{"pending", "dev", "review", "done"}, status,
		"stuck `review` must have been rewound out of the orphan state")
	assert.NotEqual(t, "review", status, "the original stuck state must have been cleared")
}

// decisionNeededReviewer simule un /bmad-code-review qui retourne un
// finding tagged "decision-needed" : pass=false, NeedsArchitect=true,
// DecisionCount=1. Sert à tester l'escalation autonome vers Architect.
type decisionNeededReviewer struct{ calls int }

func (*decisionNeededReviewer) Name() string { return "decision-reviewer" }
func (r *decisionNeededReviewer) Review(_ context.Context, _ ProjectContext, story Story, _ DevOutput) (ReviewVerdict, error) {
	r.calls++
	v := ReviewVerdict{
		Pass:           false,
		Feedback:       "Choix de cache: Redis vs in-process — decision-needed",
		NeedsArchitect: true,
		DecisionCount:  1,
	}
	for _, ac := range story.ACs {
		v.ACs = append(v.ACs, ReviewedCriterion{ID: ac.ID, Passed: false, Reason: "decision pending"})
	}
	return v, nil
}

// recordingArchitect note ses invocations pour qu'on puisse asserter
// qu'il a été appelé sans dépendre de BMAD réel.
type recordingArchitect struct{ calls int }

func (*recordingArchitect) Name() string { return "test-architect" }
func (a *recordingArchitect) Resolve(_ context.Context, _ ProjectContext, _ Story, _ string) error {
	a.calls++
	return nil
}

func TestSupervisorEscalatesDecisionNeededToArchitect(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()
	workdir := filepath.Join(t.TempDir(), "work")
	_ = seedProject(t, st.DB, workdir)

	reviewer := &decisionNeededReviewer{}
	arch := &recordingArchitect{}
	sup := NewSupervisor(st.DB, NewScriptedDev(), reviewer, time.Second).
		WithArchitect(arch)

	// Premier tick : decision-needed → escalation, iteration NE doit PAS
	// bouger, story doit revenir à pending.
	sup.tick(context.Background())
	assert.Equal(t, 1, arch.calls, "architect doit être invoqué sur decision-needed")
	var status string
	var iterations int
	require.NoError(t, st.DB.QueryRow(
		`SELECT status, iterations FROM stories WHERE id = 'sty_1'`,
	).Scan(&status, &iterations))
	assert.Equal(t, "pending", status, "après escalation, story revient à pending")
	assert.Equal(t, 0, iterations, "l'escalation ne consomme PAS le budget d'itérations")

	// Un review 'architect_resolved' doit avoir été écrit.
	var resolvedRows int
	require.NoError(t, st.DB.QueryRow(
		`SELECT COUNT(*) FROM reviews WHERE story_id = 'sty_1' AND verdict = 'architect_resolved'`,
	).Scan(&resolvedRows))
	assert.Equal(t, 1, resolvedRows, "un review architect_resolved doit avoir été inséré")
}

func TestSupervisorCapsArchitectEscalations(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()
	workdir := filepath.Join(t.TempDir(), "work")
	_ = seedProject(t, st.DB, workdir)

	reviewer := &decisionNeededReviewer{}
	arch := &recordingArchitect{}
	sup := NewSupervisor(st.DB, NewScriptedDev(), reviewer, time.Second).
		WithArchitect(arch)

	// MaxArchitectEscalations + N ticks : l'architect ne doit JAMAIS être
	// appelé plus de MaxArchitectEscalations fois, et la story doit
	// finir blocked (cap d'itérations atteint après l'escalation).
	for i := 0; i < MaxArchitectEscalations+MaxIterations+2; i++ {
		sup.tick(context.Background())
	}
	assert.LessOrEqual(t, arch.calls, MaxArchitectEscalations,
		"le cap d'escalations architect doit tenir")
	var status string
	require.NoError(t, st.DB.QueryRow(
		`SELECT status FROM stories WHERE id = 'sty_1'`,
	).Scan(&status))
	assert.Equal(t, storyStatusBlocked, status,
		"après le cap d'escalations + cap d'itérations, la story doit être blocked")
}

func TestSupervisorIgnoresNonBuildingProjects(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	require.NoError(t, err)
	defer st.Close()

	// Project in 'draft' — must be skipped.
	_, err = st.DB.Exec(
		`INSERT INTO projects (id, name, idea, status, tenant_id)
		 VALUES ('prj_draft', 'x', 'y', 'draft', 'default')`,
	)
	require.NoError(t, err)
	_, err = st.DB.Exec(
		`INSERT INTO epics (id, project_id, title, ordering, status) VALUES ('e1', 'prj_draft', 'E', 0, 'pending')`,
	)
	require.NoError(t, err)
	_, err = st.DB.Exec(
		`INSERT INTO stories (id, epic_id, title, ordering, status) VALUES ('s1', 'e1', 'S', 0, 'pending')`,
	)
	require.NoError(t, err)

	sup := NewSupervisor(st.DB, NewScriptedDev(), NewScriptedReviewer(), time.Second)
	sup.tick(context.Background())

	var status string
	require.NoError(t, st.DB.QueryRow(`SELECT status FROM stories WHERE id = 's1'`).Scan(&status))
	assert.Equal(t, "pending", status, "draft project's stories must stay untouched")
}
