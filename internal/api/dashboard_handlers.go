package api

import (
	"net/http"
)

// Remaining list handlers after the BMAD cleanup. Most of the old dashboard
// (workflows, dialogs, federation, auctions, users, tenants, cluster, trust
// history) was dropped with the enterprise-feature purge; only knowledge
// and audit survive because the BMAD agents consult them during a build.
//
// scanAll funnel: row iteration goes through scanAll so a bad Scan is
// logged instead of silently skipped — matches the invariant the rest of
// the API relies on.

func (s *Server) handleListKnowledge(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	taskType := r.URL.Query().Get("type")
	tClause, tArgs := tenantFilter(ctx, "")
	args := append([]any{}, tArgs...)
	typeClause := ""
	if taskType != "" {
		typeClause = ` AND task_type = ?`
		args = append(args, taskType)
	}
	args = append(args, parseLimit(r, 200, 500), parseOffset(r))
	rows, err := s.db().QueryContext(ctx,
		`SELECT id, task_type, approach, outcome, COALESCE(context,''), created_at
		 FROM knowledge WHERE 1=1`+tClause+typeClause+`
		 ORDER BY created_at DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	defer rows.Close()
	type entry struct {
		ID        int64  `json:"id"`
		TaskType  string `json:"task_type"`
		Approach  string `json:"approach"`
		Outcome   string `json:"outcome"`
		Context   string `json:"context"`
		CreatedAt string `json:"created_at"`
	}
	var out []entry
	scanAll(rows, "knowledge", func() error {
		var e entry
		if err := rows.Scan(&e.ID, &e.TaskType, &e.Approach, &e.Outcome, &e.Context, &e.CreatedAt); err != nil {
			return err
		}
		out = append(out, e)
		return nil
	})
	writeJSON(w, out)
}

func (s *Server) handleListAudit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tClause, tArgs := tenantFilter(ctx, "")
	args := append([]any{}, tArgs...)
	args = append(args, parseLimit(r, 200, 500), parseOffset(r))
	rows, err := s.db().QueryContext(ctx,
		`SELECT id, action, actor, resource, COALESCE(detail, ''), created_at
		 FROM audit_log WHERE 1=1`+tClause+`
		 ORDER BY created_at DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	defer rows.Close()
	type entry struct {
		ID        int64  `json:"id"`
		Action    string `json:"action"`
		Actor     string `json:"actor"`
		Resource  string `json:"resource"`
		Detail    string `json:"detail"`
		CreatedAt string `json:"created_at"`
	}
	var out []entry
	scanAll(rows, "audit_log", func() error {
		var e entry
		if err := rows.Scan(&e.ID, &e.Action, &e.Actor, &e.Resource, &e.Detail, &e.CreatedAt); err != nil {
			return err
		}
		out = append(out, e)
		return nil
	})
	writeJSON(w, out)
}
