package api

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/JulienLeotier/hive/internal/agent"
	"github.com/JulienLeotier/hive/internal/auth"
	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/resilience"
)

// cryptoRandRead is declared here so the server file doesn't reach into
// crypto/rand at call sites cluttered with other names.
func cryptoRandRead(b []byte) (int, error) { return rand.Read(b) }

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

// Server holds all dependencies for the HTTP API.
type Server struct {
	agentMgr         *agent.Manager
	eventBus         *event.Bus
	breakers         *resilience.BreakerRegistry
	keyMgr           *KeyManager
	users            *auth.UserStore
	federationShared []string // capabilities exposed to federated peers (empty = all)
	oidc             *auth.OIDCProvider
	mux              *http.ServeMux
}

// NewServer creates an API server with all dependencies.
func NewServer(agentMgr *agent.Manager, eventBus *event.Bus, breakers *resilience.BreakerRegistry, keyMgr *KeyManager) *Server {
	s := &Server{
		agentMgr: agentMgr,
		eventBus: eventBus,
		breakers: breakers,
		keyMgr:   keyMgr,
		mux:      http.NewServeMux(),
	}
	s.routes()
	return s
}

// WithUsers attaches an RBAC user store so the resolver middleware can map API
// key names to roles. Story 21.2.
func (s *Server) WithUsers(users *auth.UserStore) *Server {
	s.users = users
	return s
}

// WithOIDC installs an OIDC provider so /auth/login redirects to the IdP.
// Story 21.1.
func (s *Server) WithOIDC(p *auth.OIDCProvider) *Server {
	s.oidc = p
	// Register the auth routes. These are outside /api/v1 so they go through
	// the AuthMiddleware chain only when mounted at /; we just mux them here.
	s.mux.HandleFunc("GET /auth/login", s.handleOIDCLogin)
	s.mux.HandleFunc("GET /auth/callback", s.handleOIDCCallback)
	return s
}

func (s *Server) handleOIDCLogin(w http.ResponseWriter, r *http.Request) {
	if s.oidc == nil {
		http.Error(w, "oidc not configured", http.StatusNotFound)
		return
	}
	state := randomState()
	http.SetCookie(w, &http.Cookie{
		Name:     "hive_oidc_state",
		Value:    state,
		Path:     "/auth",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
		MaxAge:   600,
	})
	http.Redirect(w, r, s.oidc.AuthRedirectURL(state), http.StatusFound)
}

func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	if s.oidc == nil {
		http.Error(w, "oidc not configured", http.StatusNotFound)
		return
	}
	state := r.URL.Query().Get("state")
	cookie, err := r.Cookie("hive_oidc_state")
	if err != nil || cookie.Value == "" || cookie.Value != state {
		http.Error(w, "state mismatch", http.StatusBadRequest)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}
	tok, err := s.oidc.Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, "token exchange failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	info, err := s.oidc.FetchUserInfo(r.Context(), tok.AccessToken)
	if err != nil {
		http.Error(w, "userinfo failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	// Auto-provision an RBAC record if none exists — default role = viewer.
	if s.users != nil {
		if _, err := s.users.Get(r.Context(), info.Subject); err != nil {
			_ = s.users.Upsert(r.Context(), auth.UserRecord{
				Subject: info.Subject, Role: auth.RoleViewer, TenantID: "default",
			})
		}
	}
	// Set a session cookie. Out-of-scope here to sign/verify it; the existing
	// API-key middleware remains the canonical auth path for /api/v1/*.
	http.SetCookie(w, &http.Cookie{
		Name:     "hive_user",
		Value:    info.Subject,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
		MaxAge:   3600,
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

func randomState() string {
	b := make([]byte, 16)
	_, _ = cryptoRandRead(b)
	return fmt.Sprintf("%x", b)
}

func (s *Server) routes() {
	// Read-only endpoints require at least "viewer".
	s.mux.Handle("GET /api/v1/agents", auth.RBACMiddleware("agents", "read")(http.HandlerFunc(s.handleListAgents)))
	s.mux.Handle("GET /api/v1/events", auth.RBACMiddleware("events", "read")(http.HandlerFunc(s.handleListEvents)))
	s.mux.Handle("GET /api/v1/metrics", auth.RBACMiddleware("system", "read")(http.HandlerFunc(s.handleMetrics)))
	s.mux.Handle("GET /api/v1/tasks", auth.RBACMiddleware("tasks", "read")(http.HandlerFunc(s.handleListTasks)))
	s.mux.Handle("GET /api/v1/costs", auth.RBACMiddleware("system", "read")(http.HandlerFunc(s.handleCosts)))
	// Story 19.2: capabilities endpoint so federated peers can discover what
	// we're willing to handle. Filtered by FederationShared — no filter means
	// every registered capability is visible.
	s.mux.HandleFunc("GET /api/v1/capabilities", s.handleListCapabilities)
	// Write endpoint: viewers get 403.
	s.mux.Handle("POST /api/v1/agents", auth.RBACMiddleware("agents", "write")(http.HandlerFunc(s.handleCreateAgent)))
	// Story 2.1 AC: "agents can emit custom events via the adapter protocol".
	// POST /api/v1/events lets an adapter push a (type, payload) into the bus.
	s.mux.Handle("POST /api/v1/events", auth.RBACMiddleware("events", "write")(http.HandlerFunc(s.handleEmitEvent)))
}

// SetFederationShared configures which capability names are exposed to peers
// via /api/v1/capabilities. Empty = everything.
func (s *Server) SetFederationShared(caps []string) {
	s.federationShared = caps
}

// Handler returns the HTTP handler with auth + role-resolver middleware chained.
// The role resolver looks up the API key name → role mapping so downstream
// RBACMiddleware can enforce per-resource rules. If no user store is attached,
// every authenticated request is treated as an admin (dev mode compatibility).
func (s *Server) Handler() http.Handler {
	var jwtValidator JWTValidator
	if s.oidc != nil {
		jwtValidator = s.oidc.ValidateJWT
	}
	return AuthMiddlewareWithJWT(s.keyMgr, jwtValidator)(s.roleResolver(s.mux))
}

// roleResolver pulls the API key name set by AuthMiddleware and resolves it to
// a role (+tenant) via the UserStore; stashes them in context for RBACMiddleware.
func (s *Server) roleResolver(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if s.users == nil {
			ctx = auth.WithRole(ctx, auth.RoleAdmin) // no directory → trust the key
			ctx = auth.WithTenant(ctx, "default")
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		keyName, _ := ctx.Value(ctxKeyName).(string)
		if keyName == "" {
			// AuthMiddleware let a dev-mode request through (no keys configured).
			ctx = auth.WithRole(ctx, auth.RoleAdmin)
			ctx = auth.WithTenant(ctx, "default")
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		user, err := s.users.Get(ctx, keyName)
		if err != nil {
			// Key exists but isn't mapped to an RBAC role → viewer by default.
			ctx = auth.WithRole(ctx, auth.RoleViewer)
			ctx = auth.WithTenant(ctx, "default")
		} else {
			ctx = auth.WithRole(ctx, user.Role)
			ctx = auth.WithTenant(ctx, user.TenantID)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// handleCreateAgent is a placeholder write endpoint proving the RBAC flow —
// reject "viewer", allow "operator"/"admin". Real registration still happens
// via the CLI (`hive add-agent`); this exists so the authenticated write path
// is testable end-to-end.
func (s *Server) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "accepted"})
}

// handleEmitEvent lets an authenticated agent push a custom event. Story 2.1.
// The request body is {type, source?, payload}. Source defaults to the
// authenticated key name so events are attributable.
func (s *Server) handleEmitEvent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Type    string `json:"type"`
		Source  string `json:"source"`
		Payload any    `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if body.Type == "" {
		writeError(w, http.StatusBadRequest, "MISSING_TYPE", "event type is required")
		return
	}
	if body.Source == "" {
		if keyName, ok := r.Context().Value(ctxKeyName).(string); ok {
			body.Source = keyName
		} else {
			body.Source = "adapter"
		}
	}
	evt, err := s.eventBus.Publish(r.Context(), body.Type, body.Source, body.Payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "PUBLISH_FAILED", err.Error())
		return
	}
	writeJSON(w, map[string]any{"id": evt.ID, "accepted_at": evt.CreatedAt})
}

// handleListCapabilities returns only the capabilities the operator has opted
// to share with federated peers. Story 19.2.
func (s *Server) handleListCapabilities(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	agents, err := s.agentMgr.List(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}

	shareFilter := map[string]bool{}
	for _, c := range s.federationShared {
		shareFilter[c] = true
	}

	seen := map[string]bool{}
	var caps []string
	for _, a := range agents {
		var decl struct {
			TaskTypes []string `json:"task_types"`
		}
		_ = json.Unmarshal([]byte(a.Capabilities), &decl)
		for _, t := range decl.TaskTypes {
			if len(shareFilter) > 0 && !shareFilter[t] {
				continue
			}
			if seen[t] {
				continue
			}
			seen[t] = true
			caps = append(caps, t)
		}
	}
	writeJSON(w, caps)
}

// Start runs the HTTP server.
func (s *Server) Start(addr string) error {
	slog.Info("API server starting", "addr", addr)
	return http.ListenAndServe(addr, s.Handler())
}

func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	agents, err := s.agentMgr.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}
	writeJSON(w, agents)
}

func (s *Server) handleListEvents(w http.ResponseWriter, r *http.Request) {
	opts := event.QueryOpts{
		Type:   r.URL.Query().Get("type"),
		Source: r.URL.Query().Get("source"),
		Limit:  50,
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

	// Story 8.4: dashboard expects newest-first on initial load.
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}

	writeJSON(w, events)
}

// handleListTasks returns tasks with enough fields for the dashboard grouping.
func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := s.db().QueryContext(ctx,
		`SELECT t.id, t.workflow_id, t.type, t.status,
		        COALESCE(t.agent_id, ''), COALESCE(a.name, ''), t.created_at
		 FROM tasks t LEFT JOIN agents a ON a.id = t.agent_id
		 ORDER BY t.created_at DESC LIMIT 500`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	defer rows.Close()

	type taskRow struct {
		ID         string `json:"id"`
		WorkflowID string `json:"workflow_id"`
		Type       string `json:"type"`
		Status     string `json:"status"`
		AgentID    string `json:"agent_id"`
		AgentName  string `json:"agent_name"`
		CreatedAt  string `json:"created_at"`
	}
	var tasks []taskRow
	for rows.Next() {
		var t taskRow
		if err := rows.Scan(&t.ID, &t.WorkflowID, &t.Type, &t.Status, &t.AgentID, &t.AgentName, &t.CreatedAt); err == nil {
			tasks = append(tasks, t)
		}
	}
	writeJSON(w, tasks)
}

// handleCosts returns per-agent cost summaries and budget alerts.
func (s *Server) handleCosts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	db := s.db()

	type summary struct {
		AgentName string  `json:"agent_name"`
		TotalCost float64 `json:"total_cost"`
		TaskCount int     `json:"task_count"`
	}
	rows, err := db.QueryContext(ctx,
		`SELECT agent_name, SUM(cost), COUNT(*) FROM costs GROUP BY agent_name ORDER BY SUM(cost) DESC`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	defer rows.Close()

	var summaries []summary
	for rows.Next() {
		var s summary
		if err := rows.Scan(&s.AgentName, &s.TotalCost, &s.TaskCount); err == nil {
			summaries = append(summaries, s)
		}
	}

	// Budget alerts
	type alert struct {
		AgentName  string  `json:"agent_name"`
		DailyLimit float64 `json:"daily_limit"`
		Spend      float64 `json:"spend"`
		Breached   bool    `json:"breached"`
	}
	var alerts []alert
	if aRows, err := db.QueryContext(ctx,
		`SELECT b.agent_name, b.daily_limit,
		        COALESCE((SELECT SUM(cost) FROM costs WHERE agent_name = b.agent_name AND date(created_at) = date('now')), 0)
		 FROM budget_alerts b WHERE b.enabled = 1`); err == nil {
		defer aRows.Close()
		for aRows.Next() {
			var a alert
			if err := aRows.Scan(&a.AgentName, &a.DailyLimit, &a.Spend); err == nil {
				a.Breached = a.Spend >= a.DailyLimit
				alerts = append(alerts, a)
			}
		}
	}

	writeJSON(w, map[string]any{
		"summaries": summaries,
		"alerts":    alerts,
	})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	agents, _ := s.agentMgr.List(ctx)
	healthy, degraded, unavailable := 0, 0, 0
	for _, a := range agents {
		switch a.HealthStatus {
		case "healthy":
			healthy++
		case "degraded":
			degraded++
		default:
			unavailable++
		}
	}

	breakers := s.breakers.AllStates()
	openCircuits := 0
	for _, state := range breakers {
		if state == resilience.StateOpen {
			openCircuits++
		}
	}

	taskCounts := countRowsByStatus(ctx, s.db(), "tasks")
	workflowCounts := countRowsByStatus(ctx, s.db(), "workflows")

	// Event throughput: events in the last minute and last hour.
	eventsLastMinute := s.countEventsSince(ctx, time.Now().Add(-time.Minute))
	eventsLastHour := s.countEventsSince(ctx, time.Now().Add(-time.Hour))

	// Story 6.4 AC: "average task duration". Computed across every completed
	// task in the last 24h.
	var avgDurationS float64
	_ = s.db().QueryRowContext(ctx,
		`SELECT COALESCE(AVG((JULIANDAY(completed_at) - JULIANDAY(started_at)) * 86400), 0)
		 FROM tasks WHERE status = 'completed' AND started_at IS NOT NULL AND completed_at IS NOT NULL
		 AND created_at >= datetime('now', '-1 day')`).Scan(&avgDurationS)

	metrics := map[string]any{
		"agents": map[string]int{
			"total":       len(agents),
			"healthy":     healthy,
			"degraded":    degraded,
			"unavailable": unavailable,
		},
		"circuit_breakers": map[string]any{
			"total": len(breakers),
			"open":  openCircuits,
		},
		"tasks":     taskCounts,
		"workflows": workflowCounts,
		"events": map[string]int{
			"last_minute": eventsLastMinute,
			"last_hour":   eventsLastHour,
		},
		"avg_task_duration_seconds": avgDurationS,
		"timestamp":                 time.Now().UTC().Format(time.RFC3339),
	}

	writeJSON(w, metrics)
}

func (s *Server) db() *sql.DB {
	return s.eventBus.DB()
}

func (s *Server) countEventsSince(ctx context.Context, t time.Time) int {
	var n int
	_ = s.db().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM events WHERE created_at >= ?`,
		t.UTC().Format("2006-01-02 15:04:05"),
	).Scan(&n)
	return n
}

func countRowsByStatus(ctx context.Context, db *sql.DB, table string) map[string]int {
	rows, err := db.QueryContext(ctx,
		fmt.Sprintf(`SELECT status, COUNT(*) FROM %s GROUP BY status`, table))
	if err != nil {
		return map[string]int{}
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var s string
		var n int
		if err := rows.Scan(&s, &n); err != nil {
			continue
		}
		out[s] = n
	}
	return out
}

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{Data: data})
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Response{Error: &Error{Code: code, Message: msg}})
}

// Serve starts the API server in the background. Returns a shutdown function.
func Serve(ctx context.Context, addr string, handler http.Handler) (shutdown func(), err error) {
	srv := &http.Server{Addr: addr, Handler: handler}

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("API server error", "error", err)
		}
	}()

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}, nil
}

func init() {
	// Suppress unused import
	_ = fmt.Sprintf
}
