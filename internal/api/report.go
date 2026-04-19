package api

import (
	"fmt"
	"net/http"
	"strings"
)

// handleProjectReport émet un rapport de build complet en Markdown :
// PRD + arbre epics/stories/ACs + historique reviews + résumé coût.
// Utile pour archiver ou partager ce qu'un projet a effectivement
// produit sans avoir à gratter workdir + DB séparément.
func (s *Server) handleProjectReport(w http.ResponseWriter, r *http.Request) {
	if s.projectStore == nil {
		writeError(w, http.StatusServiceUnavailable, "NO_PROJECT_STORE", "")
		return
	}
	id := r.PathValue("id")
	p, err := s.projectStore.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", p.Name)
	fmt.Fprintf(&b, "_Projet `%s` — statut **%s** — %s_\n\n",
		p.ID, p.Status, p.UpdatedAt.Format("2006-01-02 15:04"))

	fmt.Fprintf(&b, "## Idée\n\n> %s\n\n", strings.TrimSpace(p.Idea))

	// Meta.
	b.WriteString("## Méta\n\n")
	fmt.Fprintf(&b, "- **Workdir** : `%s`\n", p.Workdir)
	if p.RepoURL != "" {
		fmt.Fprintf(&b, "- **Repo** : %s\n", p.RepoURL)
	}
	if p.TotalCostUSD > 0 {
		cap := ""
		if p.CostCapUSD > 0 {
			cap = fmt.Sprintf(" / $%.2f (%.0f%%)", p.CostCapUSD, 100*p.TotalCostUSD/p.CostCapUSD)
		}
		fmt.Fprintf(&b, "- **Coût Claude** : $%.4f%s\n", p.TotalCostUSD, cap)
	}
	b.WriteString("\n")

	// PRD complet.
	if strings.TrimSpace(p.PRD) != "" {
		b.WriteString("## PRD\n\n")
		b.WriteString(strings.TrimSpace(p.PRD))
		b.WriteString("\n\n")
	}

	// Arbre epics → stories → ACs avec status par AC + dernière review.
	if len(p.Epics) > 0 {
		b.WriteString("## Plan de build\n\n")
		for _, ep := range p.Epics {
			fmt.Fprintf(&b, "### %s\n\n", ep.Title)
			if ep.Description != "" {
				fmt.Fprintf(&b, "%s\n\n", ep.Description)
			}
			fmt.Fprintf(&b, "_Statut : %s_\n\n", ep.Status)
			for _, st := range ep.Stories {
				marker := "☐"
				switch st.Status {
				case "done":
					marker = "✅"
				case "blocked":
					marker = "❌"
				case "dev", "review":
					marker = "⏳"
				}
				fmt.Fprintf(&b, "#### %s %s\n\n", marker, st.Title)
				if st.Description != "" {
					fmt.Fprintf(&b, "%s\n\n", st.Description)
				}
				fmt.Fprintf(&b, "- Status : `%s` (itérations : %d)\n", st.Status, st.Iterations)
				if st.Branch != "" {
					fmt.Fprintf(&b, "- Branche : `%s`\n", st.Branch)
				}
				if len(st.AcceptanceCriteria) > 0 {
					b.WriteString("\n**ACs** :\n\n")
					for _, ac := range st.AcceptanceCriteria {
						mark := "◯"
						if ac.Passed {
							mark = "●"
						}
						fmt.Fprintf(&b, "- %s %s\n", mark, ac.Text)
					}
					b.WriteString("\n")
				}
				if st.LastReviewFeedback != "" {
					fmt.Fprintf(&b, "**Dernière review** (%s) : %s\n\n",
						st.LastReviewVerdict, strings.TrimSpace(st.LastReviewFeedback))
				}
			}
		}
	}

	// Résumé coût par phase BMAD.
	rows, err := s.db().QueryContext(r.Context(),
		`SELECT phase, SUM(COALESCE(cost_usd, 0)), COUNT(*)
		 FROM bmad_phase_steps
		 WHERE project_id = ? AND status = 'done'
		 GROUP BY phase
		 ORDER BY 2 DESC`, id)
	if err == nil {
		defer func() { _ = rows.Close() }()
		b.WriteString("## Coût par phase\n\n")
		b.WriteString("| Phase | Total | Skills |\n| --- | ---: | ---: |\n")
		for rows.Next() {
			var phase string
			var total float64
			var n int
			if err := rows.Scan(&phase, &total, &n); err == nil {
				fmt.Fprintf(&b, "| `%s` | $%.4f | %d |\n", phase, total, n)
			}
		}
		_ = rows.Err()
		b.WriteString("\n")
	}

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="hive-report-%s.md"`, p.ID))
	_, _ = w.Write([]byte(b.String()))
}
