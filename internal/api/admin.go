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
