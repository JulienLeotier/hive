package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/JulienLeotier/hive/internal/bmad"
)

// handleBmadSkills expose le registre de skills BMAD invocables.
// Filtrable par ?scope=project|epic|story pour alimenter un dropdown
// contextuel dans l'UI (pas de /bmad-code-review sur un bouton projet).
func (s *Server) handleBmadSkills(w http.ResponseWriter, r *http.Request) {
	scope := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("scope")))
	out := make([]bmad.Skill, 0, len(bmad.SkillRegistry))
	for _, sk := range bmad.SkillRegistry {
		if scope != "" && string(sk.Scope) != scope {
			continue
		}
		out = append(out, sk)
	}
	writeJSON(w, out)
}

// bmadRunRequest est le payload de POST /api/v1/bmad/run. Un skill
// story-scoped exige story_id ; un skill epic-scoped exige epic_id ;
// un skill project-scoped n'a besoin que de project_id. Validation
// stricte côté handler — on refuse les combinaisons invalides.
type bmadRunRequest struct {
	Skill     string `json:"skill"`
	ProjectID string `json:"project_id"`
	EpicID    string `json:"epic_id,omitempty"`
	StoryID   string `json:"story_id,omitempty"`
}

// handleBmadRun lance un skill BMAD en arrière-plan sur le contexte
// demandé. Idempotent si l'opérateur clique deux fois : la deuxième
// invocation annule la première (cf. registerRun). Répond 202 +
// { project_id, skill, step_id } ; le client polle /phases pour voir
// le résultat et écoute les events WS.
func (s *Server) handleBmadRun(w http.ResponseWriter, r *http.Request) {
	if s.projectStore == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_PROJECT_STORE", "")
		return
	}
	var req bmadRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_JSON", err.Error())
		return
	}
	req.Skill = strings.TrimSpace(req.Skill)
	req.ProjectID = strings.TrimSpace(req.ProjectID)
	if req.Skill == "" || req.ProjectID == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST",
			"skill et project_id sont requis")
		return
	}

	skill := bmad.FindSkill(req.Skill)
	if skill == nil {
		writeError(w, http.StatusBadRequest, "UNKNOWN_SKILL",
			"ce skill n'est pas dans le registre Hive")
		return
	}

	// Validation scope → context id.
	switch skill.Scope {
	case bmad.ScopeProject:
		if req.StoryID != "" || req.EpicID != "" {
			writeError(w, http.StatusBadRequest, "SCOPE_MISMATCH",
				"ce skill est project-scoped — ne pas envoyer story_id/epic_id")
			return
		}
	case bmad.ScopeEpic:
		if req.EpicID == "" {
			writeError(w, http.StatusBadRequest, "SCOPE_MISMATCH",
				"ce skill est epic-scoped — epic_id requis")
			return
		}
	case bmad.ScopeStory:
		if req.StoryID == "" {
			writeError(w, http.StatusBadRequest, "SCOPE_MISMATCH",
				"ce skill est story-scoped — story_id requis")
			return
		}
	}

	proj, err := s.projectStore.GetByID(r.Context(), req.ProjectID)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	if proj.Workdir == "" {
		writeError(w, http.StatusConflict, "NO_WORKDIR",
			"ce projet n'a pas de workdir configuré")
		return
	}

	// Label = commande + context court, pour que /phases distingue
	// deux invocations du même skill sur des stories différentes.
	label := skill.Command
	switch skill.Scope {
	case bmad.ScopeStory:
		label = fmt.Sprintf("%s (story %s)", skill.Command, shortID(req.StoryID))
	case bmad.ScopeEpic:
		label = fmt.Sprintf("%s (epic %s)", skill.Command, shortID(req.EpicID))
	}

	goal := buildSkillGoal(skill, req)

	// Lance en goroutine : la request retourne 202 immédiatement. Le
	// ctx vit de sa propre vie (pas de timeout — match le comportement
	// des autres BMAD calls).
	go func() { //nolint:gosec // G118: request ctx dies with the handler
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		s.registerRun(req.ProjectID, cancel)
		defer s.clearRun(req.ProjectID)

		runner := bmad.NewRunner()
		if runner == nil {
			slog.Warn("bmad skill: runner indisponible", "skill", req.Skill)
			return
		}
		if _, err := s.trackedInvoke(ctx, runner, req.ProjectID, skill.Phase,
			label, proj.Workdir, goal); err != nil {
			slog.Warn("bmad skill failed",
				"skill", req.Skill, "project", req.ProjectID, "error", err)
		}
	}()

	writeJSON(w, map[string]any{
		"project_id": req.ProjectID,
		"skill":      req.Skill,
		"scope":      skill.Scope,
		"status":     "started",
	})
}

// buildSkillGoal construit le prompt d'entrée pour le skill. Pour les
// skills story-scoped, on précise à BMAD sur QUELLE story travailler
// pour éviter la drift observée en mode autonome (BMAD choisit
// lui-même une story différente de celle demandée). Un préfixe strict
// est plus fiable qu'un flag CLI : les skills BMAD lisent juste le
// prompt d'ouverture.
func buildSkillGoal(skill *bmad.Skill, req bmadRunRequest) string {
	switch skill.Scope {
	case bmad.ScopeStory:
		return fmt.Sprintf(
			"Tourne %s UNIQUEMENT sur la story dont l'id Hive est %s. "+
				"Ne touche à aucune autre story de sprint-status.yaml, même si elles sont ready-for-dev.",
			skill.Command, req.StoryID)
	case bmad.ScopeEpic:
		return fmt.Sprintf(
			"Tourne %s pour l'epic %s. Scope ta revue / rétrospective à cet epic.",
			skill.Command, req.EpicID)
	default:
		return skill.Command
	}
}

// shortID tronque un ULID à ses 6 derniers caractères pour affichage.
// Suffisant pour distinguer visuellement dans le dashboard sans
// saturer le label.
func shortID(id string) string {
	if len(id) <= 6 {
		return id
	}
	return id[len(id)-6:]
}

