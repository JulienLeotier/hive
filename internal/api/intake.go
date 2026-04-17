package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

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

	// Run the Architect synchronously. When this succeeds the project
	// flips to `building` with its epic/story/AC tree populated. If the
	// architect fails, the project stays in `planning` so the operator
	// can retry (future endpoint) — the PRD is already saved either way.
	arch := s.architectAgent()
	epics, stories, err := architect.NewDispatcher(s.db(), arch).Run(r.Context(), p.ID, p.Idea, prd)
	if err != nil {
		slog.Warn("architect run failed — project left in planning", "project", p.ID, "error", err)
		writeJSON(w, map[string]any{
			"project_id":       p.ID,
			"status":           project.StatusPlanning,
			"prd_length":       len(prd),
			"architect_error":  err.Error(),
		})
		return
	}
	if _, err := s.db().ExecContext(r.Context(),
		`UPDATE projects SET status = ?, updated_at = datetime('now') WHERE id = ?`,
		project.StatusBuilding, p.ID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "PROJECT_UPDATE_FAILED", err.Error())
		return
	}
	writeJSON(w, map[string]any{
		"project_id": p.ID,
		"status":     project.StatusBuilding,
		"prd_length": len(prd),
		"epics":      epics,
		"stories":    stories,
	})
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
