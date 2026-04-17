package api

import (
	"net/http"

	"github.com/JulienLeotier/hive/internal/optimizer"
)

// These handlers back the Svelte dashboard pages. Each is a read-only list
// that joins the table with its most-useful adjacent data so the frontend
// can render a single fetch per page.

func (s *Server) handleListWorkflows(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tClause, tArgs := tenantFilter(ctx, "")
	rows, err := s.db().QueryContext(ctx,
		`SELECT id, name, status, created_at FROM workflows
		 WHERE 1=1`+tClause+`
		 ORDER BY created_at DESC LIMIT 500`, tArgs...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	defer rows.Close()
	type row struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Status    string `json:"status"`
		CreatedAt string `json:"created_at"`
	}
	var out []row
	for rows.Next() {
		var v row
		if err := rows.Scan(&v.ID, &v.Name, &v.Status, &v.CreatedAt); err == nil {
			out = append(out, v)
		}
	}
	writeJSON(w, out)
}

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
	rows, err := s.db().QueryContext(ctx,
		`SELECT id, task_type, approach, outcome, COALESCE(context,''), created_at
		 FROM knowledge WHERE 1=1`+tClause+typeClause+`
		 ORDER BY created_at DESC LIMIT 200`, args...)
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
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.ID, &e.TaskType, &e.Approach, &e.Outcome, &e.Context, &e.CreatedAt); err == nil {
			out = append(out, e)
		}
	}
	writeJSON(w, out)
}

func (s *Server) handleListDialogs(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db().QueryContext(r.Context(),
		`SELECT t.id, t.initiator_agent_id, t.participant_agent_id, t.topic, t.status,
		        COUNT(m.id) AS message_count, t.created_at
		 FROM dialog_threads t
		 LEFT JOIN dialog_messages m ON m.thread_id = t.id
		 GROUP BY t.id
		 ORDER BY t.created_at DESC LIMIT 100`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	defer rows.Close()
	type thread struct {
		ID           string `json:"id"`
		Initiator    string `json:"initiator"`
		Participant  string `json:"participant"`
		Topic        string `json:"topic"`
		Status       string `json:"status"`
		MessageCount int    `json:"message_count"`
		CreatedAt    string `json:"created_at"`
	}
	var out []thread
	for rows.Next() {
		var t thread
		if err := rows.Scan(&t.ID, &t.Initiator, &t.Participant, &t.Topic, &t.Status, &t.MessageCount, &t.CreatedAt); err == nil {
			out = append(out, t)
		}
	}
	writeJSON(w, out)
}

func (s *Server) handleListFederation(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db().QueryContext(r.Context(),
		`SELECT name, url, status, COALESCE(shared_caps, '[]'), COALESCE(last_heartbeat, '')
		 FROM federation_links ORDER BY name`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	defer rows.Close()
	type link struct {
		Name          string `json:"name"`
		URL           string `json:"url"`
		Status        string `json:"status"`
		SharedCaps    string `json:"shared_caps"`
		LastHeartbeat string `json:"last_heartbeat"`
	}
	var out []link
	for rows.Next() {
		var l link
		if err := rows.Scan(&l.Name, &l.URL, &l.Status, &l.SharedCaps, &l.LastHeartbeat); err == nil {
			out = append(out, l)
		}
	}
	writeJSON(w, out)
}

func (s *Server) handleListAuctions(w http.ResponseWriter, r *http.Request) {
	// Single-pass query: replaces two correlated subqueries per row
	// (winner lookup + bid count) with a LEFT JOIN on the winner and a
	// GROUP BY for the count. Cuts query count on the auctions page from
	// 1+2N down to 1.
	rows, err := s.db().QueryContext(r.Context(),
		`SELECT a.id, a.task_id, a.strategy, a.status,
		        COALESCE(w.agent_name, ''),
		        COALESCE(COUNT(b.id), 0),
		        a.opened_at
		 FROM auctions a
		 LEFT JOIN bids w ON w.id = a.winner_bid_id
		 LEFT JOIN bids b ON b.auction_id = a.id
		 GROUP BY a.id, a.task_id, a.strategy, a.status, w.agent_name, a.opened_at
		 ORDER BY a.opened_at DESC LIMIT 100`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	defer rows.Close()
	type auction struct {
		ID       string `json:"id"`
		TaskID   string `json:"task_id"`
		Strategy string `json:"strategy"`
		Status   string `json:"status"`
		Winner   string `json:"winner"`
		Bids     int    `json:"bids"`
		Opened   string `json:"opened_at"`
	}
	var out []auction
	for rows.Next() {
		var a auction
		if err := rows.Scan(&a.ID, &a.TaskID, &a.Strategy, &a.Status, &a.Winner, &a.Bids, &a.Opened); err == nil {
			out = append(out, a)
		}
	}
	writeJSON(w, out)
}

func (s *Server) handleListOptimizations(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db().QueryContext(r.Context(),
		`SELECT id, setting, COALESCE(old_value, 0), COALESCE(new_value, 0),
		        COALESCE(rationale, ''), applied_at
		 FROM optimizations ORDER BY applied_at DESC LIMIT 100`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	defer rows.Close()
	type opt struct {
		ID        string  `json:"id"`
		Setting   string  `json:"setting"`
		OldValue  float64 `json:"old_value"`
		NewValue  float64 `json:"new_value"`
		Rationale string  `json:"rationale"`
		AppliedAt string  `json:"applied_at"`
	}
	var out []opt
	for rows.Next() {
		var o opt
		if err := rows.Scan(&o.ID, &o.Setting, &o.OldValue, &o.NewValue, &o.Rationale, &o.AppliedAt); err == nil {
			out = append(out, o)
		}
	}
	writeJSON(w, out)
}

// handleRecommendations runs the optimizer on demand and returns current
// recommendations so the dashboard can show them without persistence.
func (s *Server) handleRecommendations(w http.ResponseWriter, r *http.Request) {
	recs, err := optimizer.NewAnalyzer(s.db()).Analyze(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ANALYZE_FAILED", err.Error())
		return
	}
	writeJSON(w, recs)
}

func (s *Server) handleListAudit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tClause, tArgs := tenantFilter(ctx, "")
	rows, err := s.db().QueryContext(ctx,
		`SELECT id, action, actor, resource, COALESCE(detail, ''), created_at
		 FROM audit_log WHERE 1=1`+tClause+`
		 ORDER BY created_at DESC LIMIT 200`, tArgs...)
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
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.ID, &e.Action, &e.Actor, &e.Resource, &e.Detail, &e.CreatedAt); err == nil {
			out = append(out, e)
		}
	}
	writeJSON(w, out)
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	if s.users == nil {
		writeJSON(w, []any{})
		return
	}
	users, err := s.users.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	writeJSON(w, users)
}

func (s *Server) handleListTenants(w http.ResponseWriter, r *http.Request) {
	// Tenants are the distinct tenant_ids across core tables.
	rows, err := s.db().QueryContext(r.Context(),
		`SELECT DISTINCT tenant_id FROM agents
		 UNION SELECT DISTINCT tenant_id FROM tasks
		 UNION SELECT DISTINCT tenant_id FROM workflows`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil && id != "" {
			out = append(out, id)
		}
	}
	writeJSON(w, out)
}

func (s *Server) handleListCluster(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tClause, tArgs := tenantFilter(ctx, "")
	rows, err := s.db().QueryContext(ctx,
		`SELECT node_id, hostname, address, status, last_heartbeat
		 FROM cluster_members WHERE 1=1`+tClause+`
		 ORDER BY last_heartbeat DESC`, tArgs...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	defer rows.Close()
	type member struct {
		NodeID        string `json:"node_id"`
		Hostname      string `json:"hostname"`
		Address       string `json:"address"`
		Status        string `json:"status"`
		LastHeartbeat string `json:"last_heartbeat"`
	}
	var out []member
	for rows.Next() {
		var m member
		if err := rows.Scan(&m.NodeID, &m.Hostname, &m.Address, &m.Status, &m.LastHeartbeat); err == nil {
			out = append(out, m)
		}
	}
	writeJSON(w, out)
}

func (s *Server) handleListTrustHistory(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db().QueryContext(r.Context(),
		`SELECT h.id, COALESCE(a.name, h.agent_id), h.old_level, h.new_level,
		        h.reason, COALESCE(h.criteria, ''), h.created_at
		 FROM trust_history h
		 LEFT JOIN agents a ON a.id = h.agent_id
		 ORDER BY h.created_at DESC LIMIT 200`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	defer rows.Close()
	type entry struct {
		ID        string `json:"id"`
		Agent     string `json:"agent"`
		OldLevel  string `json:"old_level"`
		NewLevel  string `json:"new_level"`
		Reason    string `json:"reason"`
		Criteria  string `json:"criteria"`
		CreatedAt string `json:"created_at"`
	}
	var out []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.ID, &e.Agent, &e.OldLevel, &e.NewLevel, &e.Reason, &e.Criteria, &e.CreatedAt); err == nil {
			out = append(out, e)
		}
	}
	writeJSON(w, out)
}

