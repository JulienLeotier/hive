// Package project owns the BMAD project model: a project has an idea, a
// PRD (produced by the PM agent), a tree of epics and stories, and an
// acceptance-criteria-driven review loop. The CRUD surface here is
// deliberately web-only — everything the operator does goes through the
// dashboard; there's no `hive build` CLI since the interactive PM Q&A
// needs a conversational UI.
package project

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
)

// Lifecycle status values. The state machine is roughly:
//   draft → planning → building → review → shipped
// with any state able to fall into `failed` on unrecoverable error.
const (
	StatusDraft    = "draft"
	StatusPlanning = "planning"
	StatusBuilding = "building"
	StatusReview   = "review"
	StatusShipped  = "shipped"
	StatusFailed   = "failed"
)

// Project mirrors the projects table. Epics + stories are loaded separately
// via WithTree so a list page doesn't pull the whole graph.
//
// BMADOutputPath and RepoPath let the operator opt out of phases when they
// already have artefacts on disk: set BMADOutputPath and the Architect
// skips decomposition and reads the existing epics/stories; set RepoPath
// and the Dev agents work in that repo instead of scaffolding a fresh one.
// This is what makes Hive usable for "add feature X to my existing
// codebase" and not just greenfield builds.
type Project struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Idea           string    `json:"idea"`
	PRD            string    `json:"prd,omitempty"`
	Workdir        string    `json:"workdir,omitempty"`
	BMADOutputPath string    `json:"bmad_output_path,omitempty"`
	RepoPath       string    `json:"repo_path,omitempty"`
	Status         string    `json:"status"`
	TenantID       string    `json:"tenant_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Epics          []Epic    `json:"epics,omitempty"`
}

// Epic is one top-level work chunk inside a project, produced by the
// Architect agent when it decomposes the PRD.
type Epic struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Ordering    int       `json:"ordering"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	Stories     []Story   `json:"stories,omitempty"`
}

// Story is one dev-sized unit of work inside an epic. It carries its own
// acceptance criteria and a dev→review iteration count.
type Story struct {
	ID                 string               `json:"id"`
	EpicID             string               `json:"epic_id"`
	Title              string               `json:"title"`
	Description        string               `json:"description,omitempty"`
	Ordering           int                  `json:"ordering"`
	Status             string               `json:"status"`
	Iterations         int                  `json:"iterations"`
	AgentID            string               `json:"agent_id,omitempty"`
	Branch             string               `json:"branch,omitempty"`
	CreatedAt          time.Time            `json:"created_at"`
	UpdatedAt          time.Time            `json:"updated_at"`
	AcceptanceCriteria []AcceptanceCriterion `json:"acceptance_criteria,omitempty"`
}

// AcceptanceCriterion is the smallest verifiable unit. BMAD says a story is
// done when every AC flips to passed.
type AcceptanceCriterion struct {
	ID         int64     `json:"id"`
	StoryID    string    `json:"story_id"`
	Ordering   int       `json:"ordering"`
	Text       string    `json:"text"`
	Passed     bool      `json:"passed"`
	VerifiedAt time.Time `json:"verified_at,omitempty"`
	VerifiedBy string    `json:"verified_by,omitempty"`
}

// Store manages project persistence.
type Store struct {
	db *sql.DB
}

// NewStore builds a store backed by the hive DB.
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// CreateOpts bundles the optional fields of a new project so Create's
// signature doesn't balloon every time a phase adds an optional reference
// (BMAD output path, existing repo, later: design mockup paths, etc).
type CreateOpts struct {
	Name           string
	Workdir        string
	BMADOutputPath string
	RepoPath       string
}

// Create persists a new project in `draft` state. Name falls back to a
// short snippet of the idea when the caller leaves it blank, so the user
// can just type an idea and go.
func (s *Store) Create(ctx context.Context, tenant, idea string, opts CreateOpts) (*Project, error) {
	if idea == "" {
		return nil, fmt.Errorf("idea is required")
	}
	name := opts.Name
	if name == "" {
		name = shortName(idea)
	}
	if tenant == "" {
		tenant = "default"
	}
	id := "prj_" + ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO projects (id, name, idea, workdir, bmad_output_path, repo_path, status, tenant_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, name, idea, opts.Workdir, opts.BMADOutputPath, opts.RepoPath, StatusDraft, tenant,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting project: %w", err)
	}
	return s.GetByID(ctx, id)
}

// List returns the projects in a tenant, newest first, without the epic
// tree. Callers that need the tree call GetByID.
func (s *Store) List(ctx context.Context, tenant string, limit int) ([]Project, error) {
	if limit <= 0 {
		limit = 200
	}
	q := `SELECT id, name, idea, COALESCE(prd, ''), COALESCE(workdir, ''),
	             COALESCE(bmad_output_path, ''), COALESCE(repo_path, ''),
	             status, tenant_id, created_at, updated_at
	      FROM projects`
	args := []any{}
	if tenant != "" {
		q += ` WHERE tenant_id = ?`
		args = append(args, tenant)
	}
	q += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Project
	for rows.Next() {
		var p Project
		var created, updated string
		if err := rows.Scan(&p.ID, &p.Name, &p.Idea, &p.PRD, &p.Workdir,
			&p.BMADOutputPath, &p.RepoPath,
			&p.Status, &p.TenantID, &created, &updated); err != nil {
			return nil, err
		}
		p.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
		p.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updated)
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetByID returns a single project with its full epic + story + AC tree.
// Fan-out is three queries rather than a monster JOIN so the row mapping
// stays readable; at realistic sizes (tens of epics, hundreds of stories)
// this is fine.
func (s *Store) GetByID(ctx context.Context, id string) (*Project, error) {
	var p Project
	var created, updated string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, idea, COALESCE(prd, ''), COALESCE(workdir, ''),
		        COALESCE(bmad_output_path, ''), COALESCE(repo_path, ''),
		        status, tenant_id, created_at, updated_at
		 FROM projects WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &p.Idea, &p.PRD, &p.Workdir,
		&p.BMADOutputPath, &p.RepoPath,
		&p.Status, &p.TenantID, &created, &updated)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("project %s not found", id)
	}
	if err != nil {
		return nil, err
	}
	p.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", created)
	p.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updated)

	// Epics
	epicRows, err := s.db.QueryContext(ctx,
		`SELECT id, project_id, title, COALESCE(description, ''),
		        ordering, status, created_at
		 FROM epics WHERE project_id = ? ORDER BY ordering ASC`, id)
	if err != nil {
		return nil, err
	}
	defer epicRows.Close()
	var epicIDs []string
	for epicRows.Next() {
		var e Epic
		var epicCreated string
		if err := epicRows.Scan(&e.ID, &e.ProjectID, &e.Title, &e.Description,
			&e.Ordering, &e.Status, &epicCreated); err != nil {
			return nil, err
		}
		e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", epicCreated)
		p.Epics = append(p.Epics, e)
		epicIDs = append(epicIDs, e.ID)
	}
	if err := epicRows.Err(); err != nil {
		return nil, err
	}
	if len(epicIDs) == 0 {
		return &p, nil
	}

	// Stories for all the epics at once
	storyByEpic := map[string][]*Story{}
	storyQ := `SELECT id, epic_id, title, COALESCE(description, ''), ordering,
	                  status, iterations, COALESCE(agent_id, ''), COALESCE(branch, ''),
	                  created_at, updated_at
	           FROM stories WHERE epic_id IN (` + placeholders(len(epicIDs)) + `)
	           ORDER BY ordering ASC`
	storyArgs := make([]any, len(epicIDs))
	for i, eid := range epicIDs {
		storyArgs[i] = eid
	}
	storyRows, err := s.db.QueryContext(ctx, storyQ, storyArgs...)
	if err != nil {
		return nil, err
	}
	defer storyRows.Close()
	var storyIDs []string
	storyByID := map[string]*Story{}
	for storyRows.Next() {
		var st Story
		var sCreated, sUpdated string
		if err := storyRows.Scan(&st.ID, &st.EpicID, &st.Title, &st.Description,
			&st.Ordering, &st.Status, &st.Iterations, &st.AgentID, &st.Branch,
			&sCreated, &sUpdated); err != nil {
			return nil, err
		}
		st.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", sCreated)
		st.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", sUpdated)
		ref := st
		storyByEpic[st.EpicID] = append(storyByEpic[st.EpicID], &ref)
		storyByID[st.ID] = &ref
		storyIDs = append(storyIDs, st.ID)
	}
	if err := storyRows.Err(); err != nil {
		return nil, err
	}

	// ACs for all stories at once
	if len(storyIDs) > 0 {
		acQ := `SELECT id, story_id, ordering, text, passed,
		               COALESCE(verified_at, ''), COALESCE(verified_by, '')
		        FROM acceptance_criteria
		        WHERE story_id IN (` + placeholders(len(storyIDs)) + `)
		        ORDER BY ordering ASC`
		acArgs := make([]any, len(storyIDs))
		for i, sid := range storyIDs {
			acArgs[i] = sid
		}
		acRows, err := s.db.QueryContext(ctx, acQ, acArgs...)
		if err != nil {
			return nil, err
		}
		defer acRows.Close()
		for acRows.Next() {
			var ac AcceptanceCriterion
			var vAt string
			var passed int
			if err := acRows.Scan(&ac.ID, &ac.StoryID, &ac.Ordering, &ac.Text,
				&passed, &vAt, &ac.VerifiedBy); err != nil {
				return nil, err
			}
			ac.Passed = passed == 1
			ac.VerifiedAt, _ = time.Parse("2006-01-02 15:04:05", vAt)
			if st := storyByID[ac.StoryID]; st != nil {
				st.AcceptanceCriteria = append(st.AcceptanceCriteria, ac)
			}
		}
		if err := acRows.Err(); err != nil {
			return nil, err
		}
	}

	// Re-assemble the tree with dereffed story values.
	for i := range p.Epics {
		for _, st := range storyByEpic[p.Epics[i].ID] {
			p.Epics[i].Stories = append(p.Epics[i].Stories, *st)
		}
	}
	return &p, nil
}

// Delete removes a project and cascades the tree. Safe to call on a
// project currently building — the supervisor will notice the FK gone and
// bail cleanly.
func (s *Store) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("project %s not found", id)
	}
	return nil
}

// UpdateStatus transitions the project's state machine.
func (s *Store) UpdateStatus(ctx context.Context, id, status string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE projects SET status = ?, updated_at = datetime('now') WHERE id = ?`,
		status, id,
	)
	return err
}

// shortName trims the idea down to a reasonable display label.
func shortName(idea string) string {
	if len(idea) > 40 {
		return idea[:40] + "…"
	}
	return idea
}

// placeholders builds "?,?,?" for an IN clause.
func placeholders(n int) string {
	if n <= 0 {
		return "''"
	}
	out := make([]byte, 0, 2*n)
	for i := 0; i < n; i++ {
		if i > 0 {
			out = append(out, ',')
		}
		out = append(out, '?')
	}
	return string(out)
}
