// Package api hosts the HTTP surface of the BMAD product factory.
//
// Local single-user tool: no auth, no RBAC, no multi-tenant. The
// middleware chain injects an admin-of-"default" context on every
// request so the handful of downstream helpers that still read role /
// tenant (tenantFilter, future RBAC hooks) keep compiling without a
// deep refactor.
package api

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/JulienLeotier/hive/internal/auth"
	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/intake"
	"github.com/JulienLeotier/hive/internal/project"
)

// Response is the standard API response envelope.
type Response struct {
	Data  any    `json:"data"`
	Error *Error `json:"error"`
}

// Error is the standard API error format.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Server holds the dependencies the REST + WS handlers need.
type Server struct {
	eventBus            *event.Bus
	projectStore        *project.Store
	intakeStore         *intake.Store
	intakeAgentOverride intake.Agent
	envLookup           func(string) string
	mux                 *http.ServeMux
}

// NewServer builds the HTTP API. eventBus is the only required dep —
// everything else flows from it or is wired with the With* helpers.
func NewServer(eventBus *event.Bus) *Server {
	s := &Server{
		eventBus:  eventBus,
		mux:       http.NewServeMux(),
		envLookup: os.Getenv,
	}
	s.routes()
	return s
}

// WithProjectStore wires the BMAD project CRUD.
func (s *Server) WithProjectStore(p *project.Store) *Server {
	s.projectStore = p
	return s
}

// WithIntakeStore wires the PM-conversation store so /intake endpoints work.
func (s *Server) WithIntakeStore(i *intake.Store) *Server {
	s.intakeStore = i
	return s
}

// WithIntakeAgent lets tests inject a deterministic PM agent.
func (s *Server) WithIntakeAgent(a intake.Agent) *Server {
	s.intakeAgentOverride = a
	return s
}

// db exposes the underlying *sql.DB. The event bus owns the handle so
// every handler shares the same connection pool.
func (s *Server) db() *sql.DB { return s.eventBus.DB() }

// routes wires every HTTP endpoint. Order doesn't matter, but keeping
// related routes adjacent makes auditing easier.
func (s *Server) routes() {
	// Events (read + emit) and audit are the only "debug" surfaces the
	// BMAD dashboard exposes. Projects + intake + files are the product.
	s.mux.Handle("GET /api/v1/events", http.HandlerFunc(s.handleListEvents))
	s.mux.Handle("POST /api/v1/events", http.HandlerFunc(s.handleEmitEvent))
	s.mux.Handle("GET /api/v1/audit", http.HandlerFunc(s.handleListAudit))

	// BMAD projects.
	s.mux.Handle("GET /api/v1/projects", http.HandlerFunc(s.handleListProjects))
	s.mux.Handle("GET /api/v1/projects/{id}", http.HandlerFunc(s.handleGetProject))
	s.mux.Handle("POST /api/v1/projects", http.HandlerFunc(s.handleCreateProject))
	s.mux.Handle("GET /api/v1/gh/status", http.HandlerFunc(s.handleGhStatus))
	s.mux.Handle("POST /api/v1/gh/login", http.HandlerFunc(s.handleGhLogin))
	s.mux.Handle("POST /api/v1/gh/logout", http.HandlerFunc(s.handleGhLogout))
	s.mux.Handle("DELETE /api/v1/projects/{id}", http.HandlerFunc(s.handleDeleteProject))

	// BMAD PM intake — the Q&A that turns the idea into a PRD.
	s.mux.Handle("GET /api/v1/projects/{id}/intake", http.HandlerFunc(s.handleIntakeGet))
	s.mux.Handle("POST /api/v1/projects/{id}/intake/messages", http.HandlerFunc(s.handleIntakeMessage))
	s.mux.Handle("POST /api/v1/projects/{id}/intake/finalize", http.HandlerFunc(s.handleIntakeFinalize))

	// Itération brownfield : seconde phase sur un projet déjà livré
	// (on ajoute une feature). Conversation séparée, pipeline BMAD
	// edit-prd + solutioning incrémental.
	s.mux.Handle("GET /api/v1/projects/{id}/iterate", http.HandlerFunc(s.handleIterateGet))
	s.mux.Handle("POST /api/v1/projects/{id}/iterate/messages", http.HandlerFunc(s.handleIterateMessage))
	s.mux.Handle("POST /api/v1/projects/{id}/iterate/finalize", http.HandlerFunc(s.handleIterateFinalize))

	// BMAD planning recovery + per-story retry.
	s.mux.Handle("POST /api/v1/projects/{id}/stories/{story_id}/retry", http.HandlerFunc(s.handleRetryStory))
	s.mux.Handle("PATCH /api/v1/projects/{id}/prd", http.HandlerFunc(s.handleUpdatePRD))
	s.mux.Handle("POST /api/v1/projects/{id}/regenerate-plan", http.HandlerFunc(s.handleRegeneratePlan))

	// Workdir / repo_path file viewer — lets the operator inspect what
	// Claude Code has actually written from inside the dashboard.
	s.mux.Handle("GET /api/v1/projects/{id}/files", http.HandlerFunc(s.handleListFiles))
	s.mux.Handle("GET /api/v1/projects/{id}/files/content", http.HandlerFunc(s.handleFileContent))
}

// Handler returns the HTTP handler. Local-mode hive: the middleware
// chain just injects an admin + default-tenant context so any residual
// tenantFilter calls in downstream helpers keep working without a
// refactor.
func (s *Server) Handler() http.Handler {
	return localAdminContext(s.mux)
}

// WSHandler wraps a WebSocket upgrade handler with the same no-op
// middleware so the hub sees admin context.
func (s *Server) WSHandler(next http.Handler) http.Handler {
	return localAdminContext(next)
}

// localAdminContext injects admin + default tenant into every request
// so downstream helpers that still read auth context compile cleanly.
func localAdminContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ctx = auth.WithRole(ctx, auth.RoleAdmin)
		ctx = auth.WithTenant(ctx, "default")
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// handleListEvents returns recent events, newest-first. Supports
// `type`, `source`, `since` (RFC3339), and `limit` query params.
func (s *Server) handleListEvents(w http.ResponseWriter, r *http.Request) {
	opts := event.QueryOpts{
		Type:   r.URL.Query().Get("type"),
		Source: r.URL.Query().Get("source"),
		Limit:  parseLimit(r, 50, 500),
	}
	if since := r.URL.Query().Get("since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			opts.Since = t
		}
	}
	events, err := s.eventBus.Query(r.Context(), opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	// Dashboard expects newest-first.
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}
	writeJSON(w, events)
}

// handleEmitEvent lets an external caller push a {type, payload} into
// the bus. Useful for adapters/cron jobs that want to light up the
// dashboard timeline without going through a BMAD endpoint.
func (s *Server) handleEmitEvent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Type    string `json:"type"`
		Source  string `json:"source"`
		Payload any    `json:"payload"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if body.Type == "" {
		writeError(w, http.StatusBadRequest, "MISSING_TYPE", "type is required")
		return
	}
	if body.Source == "" {
		body.Source = "api"
	}
	evt, err := s.eventBus.Publish(r.Context(), body.Type, body.Source, body.Payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "PUBLISH_FAILED", err.Error())
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, evt)
}

// handleListAudit returns recent audit entries. The audit table is
// written by the event subsystem whenever a sensitive write lands;
// this endpoint is just a read path.
func (s *Server) handleListAudit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	type row struct {
		ID        int64  `json:"id"`
		Action    string `json:"action"`
		Actor     string `json:"actor"`
		Resource  string `json:"resource"`
		Detail    string `json:"detail"`
		CreatedAt string `json:"created_at"`
	}
	rows, err := s.eventBus.DB().QueryContext(ctx,
		`SELECT id, action, COALESCE(actor, ''), COALESCE(resource, ''),
		        COALESCE(detail, ''), created_at
		 FROM audit_log ORDER BY id DESC LIMIT 500`)
	if err != nil {
		// Table may not exist on a very fresh DB — return empty rather than 500.
		if strings.Contains(err.Error(), "no such table") {
			writeJSON(w, []row{})
			return
		}
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	defer rows.Close()
	var out []row
	for rows.Next() {
		var e row
		if err := rows.Scan(&e.ID, &e.Action, &e.Actor, &e.Resource, &e.Detail, &e.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "SCAN_FAILED", err.Error())
			return
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "SCAN_FAILED", err.Error())
		return
	}
	writeJSON(w, out)
}

// writeJSON wraps the payload in the canonical {data, error} envelope.
// Never use json.NewEncoder(w).Encode(payload) directly — dashboards
// and tests depend on the envelope shape.
func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(Response{Data: payload})
}

// writeError wraps an error in the canonical envelope with HTTP status.
func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Response{Error: &Error{Code: code, Message: message}})
}
