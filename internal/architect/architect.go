// Package architect turns a finalised PRD into the BMAD work breakdown:
// epics, stories, and acceptance criteria ready for the Dev + Reviewer
// agents to pick up. Runs synchronously from the intake-finalize flow so
// clicking "Finalize PRD" on the dashboard moves the project straight to
// `building` state with its story tree populated.
//
// Two Agent implementations:
//
//   - ScriptedAgent — deterministic. Splits the PRD by markdown heading,
//     maps the BMAD rubric sections (Audience / Core Flows /
//     Non-Goals / Tech / Definition of Done) to epics, and seeds 2–3
//     stories per epic with 2–3 canned acceptance criteria each.
//     Always ships something valid; handy in CI and as the fallback.
//
//   - ClaudeCodeAgent — invokes the local `claude` CLI with the PRD and
//     a system prompt asking for a JSON-structured breakdown. Falls back
//     to the scripted agent when the CLI is missing, times out, or
//     returns malformed output so a build can't dead-end on env issues.
package architect

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"crypto/rand"

	"github.com/oklog/ulid/v2"
)

// EpicDraft is what an Agent returns. Persisted into epics + stories +
// acceptance_criteria tables by Dispatcher.Decompose.
type EpicDraft struct {
	Title       string
	Description string
	Stories     []StoryDraft
}

// StoryDraft with its acceptance criteria.
type StoryDraft struct {
	Title              string
	Description        string
	AcceptanceCriteria []string
}

// Agent is the decomposition surface.
type Agent interface {
	Name() string
	// Decompose turns a PRD (and the project's raw idea) into a list of
	// epics. Caller persists the result.
	Decompose(ctx context.Context, projectIdea, prd string) ([]EpicDraft, error)
}

// Dispatcher persists an Agent's decomposition into the hive DB. Kept
// standalone (rather than a method on Agent) so the same Agent can drive
// dry-run previews on the dashboard without touching the DB.
type Dispatcher struct {
	db    *sql.DB
	agent Agent
}

// NewDispatcher wraps an Agent with its DB handle.
func NewDispatcher(db *sql.DB, agent Agent) *Dispatcher {
	return &Dispatcher{db: db, agent: agent}
}

// Run decomposes the project's PRD and writes the epics/stories/ACs. If
// the project already has epics (re-run case), Run is a no-op so we don't
// stomp progress. Returns the number of epics + stories created.
func (d *Dispatcher) Run(ctx context.Context, projectID, projectIdea, prd string) (epics, stories int, err error) {
	// Guard: skip when epics already exist for this project. Re-runs
	// would otherwise duplicate work mid-build.
	var existing int
	_ = d.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM epics WHERE project_id = ?`, projectID,
	).Scan(&existing)
	if existing > 0 {
		return 0, 0, nil
	}

	drafts, err := d.agent.Decompose(ctx, projectIdea, prd)
	if err != nil {
		return 0, 0, fmt.Errorf("decompose: %w", err)
	}
	if len(drafts) == 0 {
		return 0, 0, fmt.Errorf("architect produced zero epics — PRD too thin or agent errored silently")
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback() //nolint:errcheck // post-Commit rollback is a no-op

	for i, ed := range drafts {
		epicID := "epc_" + ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
		_, err = tx.ExecContext(ctx,
			`INSERT INTO epics (id, project_id, title, description, ordering, status)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			epicID, projectID, ed.Title, ed.Description, i, "pending",
		)
		if err != nil {
			return 0, 0, fmt.Errorf("insert epic %q: %w", ed.Title, err)
		}
		epics++
		for j, sd := range ed.Stories {
			storyID := "sty_" + ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
			_, err = tx.ExecContext(ctx,
				`INSERT INTO stories (id, epic_id, title, description, ordering, status)
				 VALUES (?, ?, ?, ?, ?, ?)`,
				storyID, epicID, sd.Title, sd.Description, j, "pending",
			)
			if err != nil {
				return 0, 0, fmt.Errorf("insert story %q: %w", sd.Title, err)
			}
			stories++
			for k, ac := range sd.AcceptanceCriteria {
				_, err = tx.ExecContext(ctx,
					`INSERT INTO acceptance_criteria (story_id, ordering, text, passed)
					 VALUES (?, ?, ?, 0)`,
					storyID, k, ac,
				)
				if err != nil {
					return 0, 0, fmt.Errorf("insert AC for story %q: %w", sd.Title, err)
				}
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return epics, stories, nil
}

// ExtractSection pulls the content under a markdown H2 heading matching
// one of the aliases (case-insensitive). Used by the scripted agent to
// walk a PRD section-by-section. Exported for tests.
func ExtractSection(prd string, aliases ...string) string {
	normalised := strings.ToLower(prd)
	for _, alias := range aliases {
		needle := "## " + strings.ToLower(alias)
		i := strings.Index(normalised, needle)
		if i == -1 {
			continue
		}
		// Advance past the heading line.
		start := i + len(needle)
		if nl := strings.Index(prd[start:], "\n"); nl >= 0 {
			start += nl + 1
		}
		// Section ends at the next H2 or EOF.
		end := strings.Index(prd[start:], "\n## ")
		if end == -1 {
			return strings.TrimSpace(prd[start:])
		}
		return strings.TrimSpace(prd[start : start+end])
	}
	return ""
}
