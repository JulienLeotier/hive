package api

import (
	"net/http"
	"strings"
)

// handleSearch : recherche simple par substring (LIKE '%q%') sur
// projets, epics, stories. Limité à 30 résultats au total pour ne
// pas saturer l'UI. Case-insensitive.
//
// Query string : ?q=<terme>. Moins de 2 caractères → réponse vide
// pour éviter de retourner la base entière.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	type hit struct {
		Type    string `json:"type"` // project | epic | story
		ID      string `json:"id"`
		Title   string `json:"title"`
		Subtitle string `json:"subtitle,omitempty"`
		ProjectID string `json:"project_id,omitempty"`
	}
	results := []hit{}
	if len(q) < 2 {
		writeJSON(w, results)
		return
	}
	like := "%" + strings.ToLower(q) + "%"

	// Projects — match sur name ou idea.
	projRows, err := s.db().QueryContext(r.Context(),
		`SELECT id, name, COALESCE(idea, '')
		 FROM projects
		 WHERE LOWER(name) LIKE ? OR LOWER(COALESCE(idea, '')) LIKE ?
		 ORDER BY updated_at DESC LIMIT 10`, like, like)
	if err == nil {
		defer func() { _ = projRows.Close() }()
		for projRows.Next() {
			var h hit
			h.Type = "project"
			if err := projRows.Scan(&h.ID, &h.Title, &h.Subtitle); err == nil {
				results = append(results, h)
			}
		}
		_ = projRows.Err()
	}

	// Epics — titre + description.
	epicRows, eerr := s.db().QueryContext(r.Context(),
		`SELECT id, project_id, title, COALESCE(description, '')
		 FROM epics
		 WHERE LOWER(title) LIKE ? OR LOWER(COALESCE(description, '')) LIKE ?
		 LIMIT 10`, like, like)
	if eerr == nil {
		defer func() { _ = epicRows.Close() }()
		for epicRows.Next() {
			var h hit
			h.Type = "epic"
			if err := epicRows.Scan(&h.ID, &h.ProjectID, &h.Title, &h.Subtitle); err == nil {
				results = append(results, h)
			}
		}
		_ = epicRows.Err()
	}

	// Stories — titre + description.
	stRows, serr := s.db().QueryContext(r.Context(),
		`SELECT s.id, e.project_id, s.title, COALESCE(s.description, '')
		 FROM stories s JOIN epics e ON e.id = s.epic_id
		 WHERE LOWER(s.title) LIKE ? OR LOWER(COALESCE(s.description, '')) LIKE ?
		 ORDER BY s.updated_at DESC LIMIT 10`, like, like)
	if serr == nil {
		defer func() { _ = stRows.Close() }()
		for stRows.Next() {
			var h hit
			h.Type = "story"
			if err := stRows.Scan(&h.ID, &h.ProjectID, &h.Title, &h.Subtitle); err == nil {
				results = append(results, h)
			}
		}
		_ = stRows.Err()
	}

	writeJSON(w, results)
}
