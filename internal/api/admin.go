package api

import (
	"net/http"

	"github.com/JulienLeotier/hive/internal/storage"
)

// handleAdminSweep déclenche un passage de retention à la demande.
// Utile depuis l'UI Settings pour vider events/audit_log sans attendre
// le tick périodique. Utilise les defaults si cfg==nil.
func (s *Server) handleAdminSweep(w http.ResponseWriter, r *http.Request) {
	n := storage.SweepNow(r.Context(), s.db(), storage.RetentionConfig{})
	writeJSON(w, map[string]any{
		"status":        "swept",
		"rows_deleted":  n,
	})
}

// handleAdminBulkDeleteFailed supprime en un coup tous les projets
// en status=failed. Idempotent. Utile après une session BMAD qui a
// planté plusieurs projets d'affilée (cost cap atteint, décisions
// dev impossibles, etc.).
func (s *Server) handleAdminBulkDeleteFailed(w http.ResponseWriter, r *http.Request) {
	// SELECT d'abord pour retourner la liste supprimée.
	var ids []string
	func() {
		rows, err := s.db().QueryContext(r.Context(),
			`SELECT id FROM projects WHERE status = 'failed'`)
		if err != nil {
			return
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err == nil {
				ids = append(ids, id)
			}
		}
		_ = rows.Err()
	}()
	// Cascades : epics/stories/ACs/reviews/phase_steps ont ON DELETE
	// CASCADE donc un DELETE sur projects suffit.
	res, err := s.db().ExecContext(r.Context(),
		`DELETE FROM projects WHERE status = 'failed'`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DELETE_FAILED", err.Error())
		return
	}
	n, _ := res.RowsAffected()
	writeJSON(w, map[string]any{
		"deleted":     n,
		"project_ids": ids,
	})
}

// handleAdminUnwedgeStories rewind en `pending` (ou blocked si trop
// d'itérations) toutes les stories coincées en `dev`/`review` hors
// d'un devloop actif. Fix rétroactif pour des courses concurrentes
// historiques ou des goroutines orphelines post-crash.
func (s *Server) handleAdminUnwedgeStories(w http.ResponseWriter, r *http.Request) {
	res, err := s.db().ExecContext(r.Context(),
		`UPDATE stories
		 SET status = CASE WHEN iterations >= 3 THEN 'blocked' ELSE 'pending' END,
		     updated_at = datetime('now')
		 WHERE status IN ('dev', 'review')
		   AND NOT EXISTS (
		     SELECT 1 FROM bmad_phase_steps s
		     WHERE s.status = 'running'
		       AND s.project_id IN (SELECT project_id FROM epics WHERE id = stories.epic_id)
		   )`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
		return
	}
	n, _ := res.RowsAffected()
	writeJSON(w, map[string]any{"unwedged": n})
}

// handleAdminStats renvoie des compteurs utiles pour l'UI admin :
// nb projects / stories / phase_steps / events / audit_log. Sert à
// afficher "la DB pèse X rows" dans Settings.
func (s *Server) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	stats := map[string]int64{}
	tables := []string{"projects", "stories", "epics", "bmad_phase_steps", "events", "audit_log", "reviews"}
	for _, t := range tables {
		var n int64
		// #nosec G201 : t est une constante d'une liste whitelistée ci-dessus.
		err := s.db().QueryRowContext(r.Context(), "SELECT COUNT(*) FROM "+t).Scan(&n)
		if err != nil {
			continue
		}
		stats[t] = n
	}
	writeJSON(w, stats)
}
