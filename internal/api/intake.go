package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/JulienLeotier/hive/internal/bmad"
	"github.com/JulienLeotier/hive/internal/intake"
	"github.com/JulienLeotier/hive/internal/project"
)

// handleIntakeGet returns the PM conversation for a project, creating it
// on first access so the dashboard can render the chat without a separate
// "start" call. The response is the conversation with its full message
// list attached.
func (s *Server) handleIntakeGet(w http.ResponseWriter, r *http.Request) {
	if s.projectStore == nil || s.intakeStore == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_INTAKE",
			"intake subsystem is not configured on this node")
		return
	}
	id := r.PathValue("id")
	p, err := s.projectStore.GetByID(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	conv, err := s.intakeStore.GetOrStart(r.Context(), p.ID, p.Idea, s.intakeAgent())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTAKE_START_FAILED", err.Error())
		return
	}
	writeJSON(w, conv)
}

// handleIntakeMessage posts a user reply in the PM conversation and
// returns the updated conversation including the agent's follow-up.
// Body: {content}.
func (s *Server) handleIntakeMessage(w http.ResponseWriter, r *http.Request) {
	if s.projectStore == nil || s.intakeStore == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_INTAKE",
			"intake subsystem is not configured on this node")
		return
	}
	id := r.PathValue("id")
	p, err := s.projectStore.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	agent := s.intakeAgent()
	conv, err := s.intakeStore.GetOrStart(r.Context(), p.ID, p.Idea, agent)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTAKE_START_FAILED", err.Error())
		return
	}
	updated, done, err := s.intakeStore.AppendUserMessage(r.Context(), conv.ID, p.Idea, body.Content, agent)
	if err != nil {
		writeError(w, http.StatusBadRequest, "APPEND_FAILED", err.Error())
		return
	}
	writeJSON(w, map[string]any{
		"conversation": updated,
		"done":         done,
	})
}

// handleIntakeFinalize asks the PM agent for the final PRD, stores it on
// the project, and flips the project status from `draft` to `planning`
// so Phase 3's Architect pipeline picks it up.
func (s *Server) handleIntakeFinalize(w http.ResponseWriter, r *http.Request) {
	if s.projectStore == nil || s.intakeStore == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_INTAKE",
			"intake subsystem is not configured on this node")
		return
	}
	id := r.PathValue("id")
	p, err := s.projectStore.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	conv, err := s.intakeStore.GetOrStart(r.Context(), p.ID, p.Idea, s.intakeAgent())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTAKE_START_FAILED", err.Error())
		return
	}

	// Flip to planning right away so the dashboard shows the spinner
	// while BMAD runs. The PRD itself is produced by bmad-create-prd
	// inside runBMADAsync — the old scripted PM agent that used to
	// Finalize() into a fake PRD is gone.
	if _, err := s.db().ExecContext(r.Context(),
		`UPDATE projects SET status = ?, updated_at = datetime('now') WHERE id = ?`,
		project.StatusPlanning, p.ID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "PROJECT_UPDATE_FAILED", err.Error())
		return
	}

	// BMAD is the slow path — `npx bmad-method install` + multiple
	// `claude --print` invocations each take minutes. Detach so the
	// HTTP request returns immediately; progress is broadcast via WS
	// (project.architect_started / _done / _failed events).
	go s.runArchitectAsync(p.ID, p.Idea, flattenConversation(conv)) //nolint:gosec // G118: request ctx dies with the handler

	writeJSON(w, map[string]any{
		"project_id": p.ID,
		"status":     project.StatusPlanning,
	})
}

// flattenConversation turns the intake conversation into a plain
// text transcript BMAD's create-prd skill can ingest as a product
// brief.
func flattenConversation(conv *intake.Conversation) string {
	if conv == nil {
		return ""
	}
	var b strings.Builder
	for _, m := range conv.Messages {
		role := "PM"
		if m.Author == "user" {
			role = "User"
		}
		fmt.Fprintf(&b, "%s: %s\n\n", role, strings.TrimSpace(m.Content))
	}
	return strings.TrimSpace(b.String())
}

// RecoverStuckPlanning re-kicks the Architect for every project parked
// in status=`planning` that has a saved PRD but no epic tree. This
// handles the case where the server was killed mid-architect: the
// detached goroutine dies with the process, leaving the project in a
// state the UI can't recover from. Safe to call more than once — the
// Architect dispatcher is idempotent on existing trees.
func (s *Server) RecoverStuckPlanning(ctx context.Context) error {
	if s.projectStore == nil {
		return nil
	}
	rows, err := s.db().QueryContext(ctx,
		`SELECT p.id, p.idea, COALESCE(p.prd, '')
		 FROM projects p
		 WHERE p.status = ?
		   AND p.prd IS NOT NULL AND p.prd <> ''
		   AND NOT EXISTS (SELECT 1 FROM epics e WHERE e.project_id = p.id)`,
		project.StatusPlanning,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	type stuck struct{ id, idea, prd string }
	var list []stuck
	for rows.Next() {
		var s stuck
		if err := rows.Scan(&s.id, &s.idea, &s.prd); err != nil {
			return err
		}
		list = append(list, s)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, st := range list {
		slog.Info("recovering stuck planning project", "project", st.id)
		go s.runArchitectAsync(st.id, st.idea, st.prd) //nolint:gosec // G118: boot-time recovery; no request ctx exists here
	}
	return nil
}

// runArchitectAsync drives the real BMAD-METHOD planning pipeline
// against the project's workdir: install BMAD, run bmad-create-prd,
// run bmad-create-epics-and-stories, ingest the resulting artefacts
// back into our DB, flip the project to `building`.
//
// Our hand-rolled architect is gone — BMAD does the same work with a
// real framework (14-step PRD, story sharding, checklist validation,
// etc.). The only glue that remains is ingesting BMAD's output back
// into the epics/stories/ACs tables so the dashboard stays
// story-centric.
//
// `seedDoc` is a text blob we pass to BMAD as the product brief. On
// first finalize it's the flattened PM chat; on recovery/regenerate
// it's whatever we have on file (previous PRD, raw idea).
func (s *Server) runArchitectAsync(projectID, idea, seedDoc string) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()

	fail := func(stage string, err error) {
		slog.Warn("bmad pipeline failed", "project", projectID, "stage", stage, "error", err)
		if s.eventBus != nil {
			_, _ = s.eventBus.Publish(ctx, "project.architect_failed", "api", map[string]any{
				"project_id": projectID,
				"stage":      stage,
				"error":      err.Error(),
			})
		}
	}

	if s.eventBus != nil {
		_, _ = s.eventBus.Publish(ctx, "project.architect_started", "api",
			map[string]string{"project_id": projectID})
	}

	proj, err := s.projectStore.GetByID(ctx, projectID)
	if err != nil {
		fail("lookup", err)
		return
	}
	if proj.Workdir == "" {
		fail("prepare", fmt.Errorf("project has no workdir"))
		return
	}
	workdir := proj.Workdir
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		fail("prepare", err)
		return
	}

	runner := bmad.NewRunner()
	if runner == nil {
		fail("prepare", fmt.Errorf("claude CLI missing; BMAD cannot run"))
		return
	}

	// Seed BMAD with the intake transcript so its create-prd workflow
	// has a product brief to work from. BMAD scans the planning dir
	// for input docs at step-02-discovery — we drop this one there.
	briefPath := filepath.Join(workdir, bmad.PlanningDir, "_intake.md")
	if err := os.MkdirAll(filepath.Dir(briefPath), 0o755); err != nil {
		fail("prepare", err)
		return
	}
	if err := os.WriteFile(briefPath, []byte(buildIntakeDoc(idea, seedDoc)), 0o644); err != nil {
		fail("prepare", err)
		return
	}

	if err := runner.Install(ctx, workdir); err != nil {
		fail("install", err)
		return
	}

	// Phase 2 — Planning: bmad-create-prd.
	prdGoal := fmt.Sprintf(
		"Invoke the bmad-create-prd skill. Treat %s/_intake.md as the product brief "+
			"(it contains the user's idea + PM Q&A). Auto-continue every menu and "+
			"complete the full PRD workflow in one pass. The PRD must end up at "+
			"%s (or %s).",
		bmad.PlanningDir, bmad.PRDFile, bmad.PRDFileLower)
	if _, err := runner.Invoke(ctx, workdir, prdGoal,
		[]string{bmad.PRDFile, bmad.PRDFileLower}); err != nil {
		fail("create-prd", err)
		return
	}
	prdText, err := readFirst(workdir, bmad.PRDFile, bmad.PRDFileLower)
	if err != nil {
		fail("read-prd", err)
		return
	}
	if _, err := s.db().ExecContext(ctx,
		`UPDATE projects SET prd = ?, updated_at = datetime('now') WHERE id = ?`,
		prdText, projectID,
	); err != nil {
		fail("save-prd", err)
		return
	}

	// Phase 3 — Solutioning: bmad-create-epics-and-stories.
	// We ask Claude to emit a JSON mirror of the tree so we don't need
	// a second parse pass over BMAD's markdown output.
	epicsGoal := "Invoke the bmad-create-epics-and-stories skill. Auto-continue every menu. " +
		"After the skill finishes its normal markdown output, append ONE fenced code " +
		"block with language `json-hive` at the very end of your reply. The block " +
		"must contain valid JSON matching exactly this shape:\n" +
		"[{\"title\":\"Epic Title\",\"description\":\"1-2 sentences\",\"stories\":" +
		"[{\"title\":\"Story Title\",\"description\":\"1-2 sentences\"," +
		"\"acceptance_criteria\":[\"AC text\",\"AC text\"]}]}]\n" +
		"Use the same epics/stories the skill just generated. Keep AC strings under 150 chars. " +
		"No prose after the json-hive block."
	res, err := runner.Invoke(ctx, workdir, epicsGoal, nil)
	if err != nil {
		fail("create-epics", err)
		return
	}
	tree, err := parseBMADTree(res.Text)
	if err != nil {
		fail("parse-epics", err)
		return
	}
	if err := s.ingestBMADTree(ctx, projectID, tree); err != nil {
		fail("ingest", err)
		return
	}

	if _, err := s.db().ExecContext(ctx,
		`UPDATE projects SET status = ?, updated_at = datetime('now') WHERE id = ?`,
		project.StatusBuilding, projectID,
	); err != nil {
		slog.Warn("bmad done but project update failed", "project", projectID, "error", err)
		return
	}
	slog.Info("bmad planning done", "project", projectID,
		"epics", len(tree), "stories", countBMADStories(tree))
	if s.eventBus != nil {
		_, _ = s.eventBus.Publish(ctx, "project.architect_done", "api", map[string]any{
			"project_id": projectID,
			"epics":      len(tree),
			"stories":    countBMADStories(tree),
		})
	}
}

// bmadEpic / bmadStory mirror the JSON shape we ask Claude to emit at
// the end of bmad-create-epics-and-stories. Kept flat so Claude
// doesn't have to match a deep nested format.
type bmadEpic struct {
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Stories     []bmadStory `json:"stories"`
}
type bmadStory struct {
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
}

func parseBMADTree(reply string) ([]bmadEpic, error) {
	marker := "```json-hive"
	start := strings.LastIndex(reply, marker)
	if start < 0 {
		return nil, fmt.Errorf("no json-hive block in reply")
	}
	body := reply[start+len(marker):]
	end := strings.Index(body, "```")
	if end < 0 {
		return nil, fmt.Errorf("json-hive block never closes")
	}
	var epics []bmadEpic
	if err := json.Unmarshal([]byte(strings.TrimSpace(body[:end])), &epics); err != nil {
		return nil, fmt.Errorf("parse json-hive: %w", err)
	}
	if len(epics) == 0 {
		return nil, fmt.Errorf("bmad emitted an empty epic tree")
	}
	return epics, nil
}

func (s *Server) ingestBMADTree(ctx context.Context, projectID string, epics []bmadEpic) error {
	tx, err := s.db().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for ei, e := range epics {
		epicID := fmt.Sprintf("epc_%s_%d", projectID, ei)
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO epics (id, project_id, title, description, ordering, status)
			 VALUES (?, ?, ?, ?, ?, 'pending')`,
			epicID, projectID, e.Title, e.Description, ei,
		); err != nil {
			return fmt.Errorf("insert epic %d: %w", ei, err)
		}
		for si, st := range e.Stories {
			storyID := fmt.Sprintf("%s_s%d", epicID, si)
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO stories (id, epic_id, title, description, ordering, status)
				 VALUES (?, ?, ?, ?, ?, 'pending')`,
				storyID, epicID, st.Title, st.Description, si,
			); err != nil {
				return fmt.Errorf("insert story %d/%d: %w", ei, si, err)
			}
			for ai, ac := range st.AcceptanceCriteria {
				if _, err := tx.ExecContext(ctx,
					`INSERT INTO acceptance_criteria (story_id, ordering, text, passed)
					 VALUES (?, ?, ?, 0)`,
					storyID, ai, ac,
				); err != nil {
					return fmt.Errorf("insert ac %d/%d/%d: %w", ei, si, ai, err)
				}
			}
		}
	}
	return tx.Commit()
}

func countBMADStories(epics []bmadEpic) int {
	n := 0
	for _, e := range epics {
		n += len(e.Stories)
	}
	return n
}

// buildIntakeDoc formats the user's web chat as a product brief BMAD
// can ingest at step-02-discovery.
func buildIntakeDoc(idea, conversation string) string {
	var b strings.Builder
	b.WriteString("# Product Brief\n\n")
	b.WriteString("## Idea\n\n")
	b.WriteString(strings.TrimSpace(idea))
	b.WriteString("\n\n")
	if strings.TrimSpace(conversation) != "" {
		b.WriteString("## PM Q&A Transcript\n\n")
		b.WriteString(strings.TrimSpace(conversation))
		b.WriteString("\n")
	}
	return b.String()
}

func readFirst(workdir string, rels ...string) (string, error) {
	for _, rel := range rels {
		abs := filepath.Join(workdir, rel)
		if data, err := os.ReadFile(abs); err == nil && len(data) > 0 {
			return string(data), nil
		}
	}
	return "", fmt.Errorf("no BMAD output at any of %v", rels)
}

// intakeAgent returns the PM agent to drive a conversation. Honours
// HIVE_INTAKE_AGENT=scripted so ops can force determinism when Claude
// CLI presence is unpredictable. Defaults to Claude with scripted
// fallback.
func (s *Server) intakeAgent() intake.Agent {
	if s.intakeAgentOverride != nil {
		return s.intakeAgentOverride
	}
	if getenv := s.envLookup; getenv != nil {
		if getenv("HIVE_INTAKE_AGENT") == "scripted" {
			return intake.NewScriptedAgent()
		}
	}
	return intake.NewClaudeCodeAgent()
}
