package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/JulienLeotier/hive/internal/architect"
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
	agent := s.intakeAgent()
	conv, err := s.intakeStore.GetOrStart(r.Context(), p.ID, p.Idea, agent)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTAKE_START_FAILED", err.Error())
		return
	}
	prd, err := s.intakeStore.Finalize(r.Context(), conv.ID, p.Idea, agent)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "FINALIZE_FAILED", err.Error())
		return
	}
	if _, err := s.db().ExecContext(r.Context(),
		`UPDATE projects SET prd = ?, status = ?, updated_at = datetime('now') WHERE id = ?`,
		prd, project.StatusPlanning, p.ID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "PROJECT_UPDATE_FAILED", err.Error())
		return
	}

	// Architect is the slow path — Claude Code can take minutes to emit
	// the epic/story/AC tree. Blocking the HTTP request would mean the
	// dashboard sits on a spinner until then, which also exceeds any
	// reasonable HTTP timeout. Kick the Architect off in the background
	// and return the updated project immediately. The frontend reacts to
	// project.architect_started / project.architect_done / project.architect_failed
	// events and polls the project tree when one arrives.
	go s.runArchitectAsync(p.ID, p.Idea, prd) //nolint:gosec // G118: the request ctx dies with the handler; runArchitectAsync deliberately uses its own 10-min background ctx

	writeJSON(w, map[string]any{
		"project_id": p.ID,
		"status":     project.StatusPlanning,
		"prd_length": len(prd),
	})
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

// runArchitectAsync executes Architect.Run detached from the caller and
// emits dashboard events so the UI can track progress. It uses a fresh
// 10-minute background context — the request context is already gone by
// the time this runs, and the caller has returned to the browser.
func (s *Server) runArchitectAsync(projectID, idea, prd string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if s.eventBus != nil {
		_, _ = s.eventBus.Publish(ctx, "project.architect_started", "api",
			map[string]string{"project_id": projectID})
	}

	arch := s.architectAgent()
	epics, stories, err := architect.NewDispatcher(s.db(), arch).Run(ctx, projectID, idea, prd)
	if err != nil {
		slog.Warn("architect run failed — project left in planning", "project", projectID, "error", err)
		if s.eventBus != nil {
			_, _ = s.eventBus.Publish(ctx, "project.architect_failed", "api", map[string]any{
				"project_id": projectID,
				"error":      err.Error(),
			})
		}
		return
	}
	if _, err := s.db().ExecContext(ctx,
		`UPDATE projects SET status = ?, updated_at = datetime('now') WHERE id = ?`,
		project.StatusBuilding, projectID,
	); err != nil {
		slog.Warn("architect done but project update failed", "project", projectID, "error", err)
		return
	}
	slog.Info("architect done", "project", projectID, "epics", epics, "stories", stories)
	if s.eventBus != nil {
		_, _ = s.eventBus.Publish(ctx, "project.architect_done", "api", map[string]any{
			"project_id": projectID,
			"epics":      epics,
			"stories":    stories,
		})
	}
}

// architectAgent returns the architect driver, honouring HIVE_ARCHITECT=scripted.
func (s *Server) architectAgent() architect.Agent {
	if s.architectAgentOverride != nil {
		return s.architectAgentOverride
	}
	if getenv := s.envLookup; getenv != nil {
		if getenv("HIVE_ARCHITECT") == "scripted" {
			return architect.NewScripted()
		}
	}
	return architect.NewClaudeCodeAgent()
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
