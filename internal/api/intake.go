package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

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
	conv, err := s.intakeStore.GetOrStart(r.Context(), p.ID, p.Idea, s.intakeAgentFor(p))
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
	s.handleConversationMessage(w, r, s.intakeAgentFor)
}

// handleConversationMessage is the shared body between the initial
// intake chat and the brownfield iteration chat. agentFn est appelé
// après chargement du projet pour laisser l'appelant picker un agent
// dépendant du contexte (greenfield vs brownfield).
func (s *Server) handleConversationMessage(w http.ResponseWriter, r *http.Request, agentFn func(*project.Project) intake.Agent) {
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
	agent := agentFn(p)
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
	writeJSON(w, map[string]any{"conversation": updated, "done": done})
}

// handleIntakeEdit permet à l'opérateur de corriger un message user
// déjà posté dans la conversation intake. Utilise le conversationID
// courant + messageID passé dans le body. L'agent agent pourra
// relire la conversation modifiée à la prochaine réponse.
func (s *Server) handleIntakeEdit(w http.ResponseWriter, r *http.Request) {
	if s.projectStore == nil || s.intakeStore == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_INTAKE", "")
		return
	}
	id := r.PathValue("id")
	p, err := s.projectStore.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	var body struct {
		MessageID int64  `json:"message_id"`
		Content   string `json:"content"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	conv, err := s.intakeStore.GetOrStart(r.Context(), p.ID, p.Idea, s.intakeAgentFor(p))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTAKE_START_FAILED", err.Error())
		return
	}
	if err := s.intakeStore.EditUserMessage(r.Context(), conv.ID, body.MessageID, body.Content); err != nil {
		writeError(w, http.StatusBadRequest, "EDIT_FAILED", err.Error())
		return
	}
	updated, _ := s.intakeStore.Load(r.Context(), conv.ID)
	writeJSON(w, updated)
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
	agent := s.intakeAgentFor(p)
	conv, err := s.intakeStore.GetOrStart(r.Context(), p.ID, p.Idea, agent)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTAKE_START_FAILED", err.Error())
		return
	}

	// Le PM produit lui-même le Product Brief SCOPE LOCKED, qu'on
	// pré-écrit dans planning-artifacts pour que /bmad-create-prd le
	// consomme tel quel. C'est le fix du bug "BMAD analyse "todolist
	// basique" → sort un concurrent Notion" : l'agent Analyst BMAD
	// élargissait systématiquement la portée. Maintenant il est
	// bypassé, le PM du chat d'intake (qui a discuté avec l'opérateur)
	// est la source de vérité.
	brief, err := s.intakeStore.Finalize(r.Context(), conv.ID, p.Idea, agent)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "BRIEF_BUILD_FAILED", err.Error())
		return
	}

	// Flip to planning + persist le brief comme PRD initial. /bmad-create-prd
	// pourra le reformatter selon ses checklists mais la substance (scope,
	// non-goals, stack) restera inchangée.
	if _, err := s.db().ExecContext(r.Context(),
		`UPDATE projects SET status = ?, prd = ?, updated_at = datetime('now') WHERE id = ?`,
		project.StatusPlanning, brief, p.ID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "PROJECT_UPDATE_FAILED", err.Error())
		return
	}

	// BMAD is the slow path — `npx bmad-method install` + multiple
	// `claude --print` invocations each take minutes. Detach so the
	// HTTP request returns immediately; progress is broadcast via WS
	// (project.architect_started / _done / _failed events).
	go s.runArchitectAsync(p.ID, p.Idea, brief) //nolint:gosec // G118: request ctx dies with the handler

	writeJSON(w, map[string]any{
		"project_id": p.ID,
		"status":     project.StatusPlanning,
	})
}

// handleIterateGet ouvre (ou retourne) la conversation d'itération
// pour un projet déjà livré. Conversation séparée de l'intake
// d'origine — role="pm-iterate" dans project_conversations.
func (s *Server) handleIterateGet(w http.ResponseWriter, r *http.Request) {
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
	conv, err := s.intakeStore.GetOrStart(r.Context(), p.ID, p.Idea, s.iterationAgent())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ITERATION_START_FAILED", err.Error())
		return
	}
	writeJSON(w, conv)
}

// handleIterateMessage poste une réponse utilisateur dans la
// conversation d'itération (brownfield post-shipping). Délègue au
// shared body avec un factory qui ignore le projet (l'itération
// utilise le même IterationAgent quel que soit le state).
func (s *Server) handleIterateMessage(w http.ResponseWriter, r *http.Request) {
	s.handleConversationMessage(w, r, func(*project.Project) intake.Agent { return s.iterationAgent() })
}

// handleIterateFinalize clôture la conversation d'itération et
// déclenche le pipeline brownfield BMAD en background :
// document-project → edit-prd → sprint-planning, etc. Le supervisor
// reprendra automatiquement sur les nouvelles stories pending.
func (s *Server) handleIterateFinalize(w http.ResponseWriter, r *http.Request) {
	if s.projectStore == nil || s.intakeStore == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_INTAKE", "")
		return
	}
	id := r.PathValue("id")
	p, err := s.projectStore.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	agent := s.iterationAgent()
	conv, err := s.intakeStore.GetOrStart(r.Context(), p.ID, p.Idea, agent)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ITERATION_START_FAILED", err.Error())
		return
	}

	// Flip direct en planning pour signaler à l'UI que l'itération
	// démarre. Un shipped project qui itère redevient building à la
	// fin du pipeline brownfield.
	if _, err := s.db().ExecContext(r.Context(),
		`UPDATE projects SET status = ?, updated_at = datetime('now') WHERE id = ?`,
		project.StatusPlanning, p.ID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "PROJECT_UPDATE_FAILED", err.Error())
		return
	}

	go s.runIterationAsync(p.ID, p.Idea, flattenConversation(conv)) //nolint:gosec // G118: request ctx dies with the handler

	writeJSON(w, map[string]any{"project_id": p.ID, "status": project.StatusPlanning})
}

// iterationAgent renvoie le PM agent wrappé pour une conversation
// d'itération (role = pm-iterate).
func (s *Server) iterationAgent() intake.Agent {
	return &intake.IterationAgent{Base: s.intakeAgent()}
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

// intakeAgentFor choisit l'agent adapté au projet : brownfield →
// IterationAgent (greeting "projet existant, qu'est-ce que tu veux
// ajouter ?") tout en conservant le role "pm" pour la conversation
// initiale ; greenfield → agent de base.
func (s *Server) intakeAgentFor(p *project.Project) intake.Agent {
	base := s.intakeAgent()
	if p != nil && p.IsExisting {
		return &intake.IterationAgent{Base: base, RoleOverride: intake.RolePM}
	}
	return base
}
