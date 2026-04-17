package api

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/JulienLeotier/hive/internal/agent"
	"github.com/JulienLeotier/hive/internal/auth"
	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/knowledge"
	"github.com/JulienLeotier/hive/internal/resilience"
)

// knowledgeStoreFromDB builds a Store on demand with the default HashingEmbedder.
// Kept inline so the server stays self-contained without threading yet another
// dependency through NewServer.
func knowledgeStoreFromDB(db *sql.DB) *knowledge.Store {
	return knowledge.NewStore(db).WithEmbedder(knowledge.NewHashingEmbedder(128))
}

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
	//
	// A3 hardening: goes through AuthMiddleware (set at Handler() level) and
	// requires "system:read" — i.e. the peer must present a valid API key or
	// OIDC JWT. Previously unauth, which leaked architecture fingerprint to
	// any network scanner. If a use case emerges for truly anonymous
	// discovery, add a dedicated public sub-endpoint with only the capability
	// names (no counts, no versions).
	s.mux.Handle("GET /api/v1/capabilities", auth.RBACMiddleware("system", "read")(http.HandlerFunc(s.handleListCapabilities)))
	// Write endpoint: viewers get 403.
	s.mux.Handle("POST /api/v1/agents", auth.RBACMiddleware("agents", "write")(http.HandlerFunc(s.handleCreateAgent)))
	// Story 2.1 AC: "agents can emit custom events via the adapter protocol".
	// POST /api/v1/events lets an adapter push a (type, payload) into the bus.
	s.mux.Handle("POST /api/v1/events", auth.RBACMiddleware("events", "write")(http.HandlerFunc(s.handleEmitEvent)))
	// Story 10.2 AC: "agent queries for approaches similar to its current task".
	s.mux.Handle("GET /api/v1/knowledge/search", auth.RBACMiddleware("system", "read")(http.HandlerFunc(s.handleKnowledgeSearch)))

	// Dashboard-backing read-only endpoints (all require at least viewer).
	s.mux.Handle("GET /api/v1/workflows", auth.RBACMiddleware("workflows", "read")(http.HandlerFunc(s.handleListWorkflows)))
	s.mux.Handle("GET /api/v1/knowledge", auth.RBACMiddleware("system", "read")(http.HandlerFunc(s.handleListKnowledge)))
	s.mux.Handle("GET /api/v1/dialogs", auth.RBACMiddleware("system", "read")(http.HandlerFunc(s.handleListDialogs)))
	s.mux.Handle("GET /api/v1/federation", auth.RBACMiddleware("system", "read")(http.HandlerFunc(s.handleListFederation)))
	s.mux.Handle("GET /api/v1/auctions", auth.RBACMiddleware("system", "read")(http.HandlerFunc(s.handleListAuctions)))
	s.mux.Handle("GET /api/v1/optimizations", auth.RBACMiddleware("system", "read")(http.HandlerFunc(s.handleListOptimizations)))
	s.mux.Handle("GET /api/v1/recommendations", auth.RBACMiddleware("system", "read")(http.HandlerFunc(s.handleRecommendations)))
	s.mux.Handle("GET /api/v1/audit", auth.RBACMiddleware("system", "read")(http.HandlerFunc(s.handleListAudit)))
	s.mux.Handle("GET /api/v1/users", auth.RBACMiddleware("system", "read")(http.HandlerFunc(s.handleListUsers)))
	s.mux.Handle("GET /api/v1/tenants", auth.RBACMiddleware("system", "read")(http.HandlerFunc(s.handleListTenants)))
	s.mux.Handle("GET /api/v1/cluster", auth.RBACMiddleware("system", "read")(http.HandlerFunc(s.handleListCluster)))
	s.mux.Handle("GET /api/v1/trust", auth.RBACMiddleware("system", "read")(http.HandlerFunc(s.handleListTrustHistory)))
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
	return AuthMiddlewareWithJWT(s.keyMgr, s.jwtValidator())(s.roleResolver(s.mux))
}

// WSHandler wraps a WebSocket upgrade handler with the same auth policy as
// the REST API, but accepts the token via ?token= query param in addition
// to the Authorization header (browsers can't send headers on WS upgrade).
// Dev mode (no API keys, no OIDC) bypasses auth to keep the local loop easy.
func (s *Server) WSHandler(next http.Handler) http.Handler {
	return WSAuthMiddleware(s.keyMgr, s.jwtValidator())(next)
}

func (s *Server) jwtValidator() JWTValidator {
	if s.oidc == nil {
		return nil
	}
	return s.oidc.ValidateJWT
}

// roleResolver pulls the API key name set by AuthMiddleware and resolves it to
// a role (+tenant) via the UserStore; stashes them in context for RBACMiddleware.
//
// A6 guard: dev-mode callers (no user store, or no API keys configured) are
// given the admin role with an EMPTY tenant string. Combined with
// tenantFilter's policy, this yields cross-tenant visibility in dev without
// requiring a hardcoded tenant name that could collide with a real customer.
func (s *Server) roleResolver(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if s.users == nil {
			ctx = auth.WithRole(ctx, auth.RoleAdmin) // no directory → trust the key
			ctx = auth.WithTenant(ctx, "")           // admin cross-tenant
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		keyName, _ := ctx.Value(ctxKeyName).(string)
		if keyName == "" {
			// AuthMiddleware let a dev-mode request through (no keys configured).
			ctx = auth.WithRole(ctx, auth.RoleAdmin)
			ctx = auth.WithTenant(ctx, "")
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		user, err := s.users.Get(ctx, keyName)
		if err != nil {
			// Key exists but isn't mapped to an RBAC role → viewer + no tenant
			// (which now fails closed instead of opening up the legacy
			// "default" tenant to a stranger).
			ctx = auth.WithRole(ctx, auth.RoleViewer)
			ctx = auth.WithTenant(ctx, "")
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
// The request body is {type, source?, payload}.
//
// Source spoofing guard (A5): the `source` field is always overwritten with
// the caller's authenticated identity, except for admins who may pass an
// explicit source (bridge/proxy use case — e.g. a gateway emitting on behalf
// of a backend service it supervises). Without this, any operator could
// post events claiming to be any other agent, poisoning the audit trail and
// triggering task.* handlers for tasks they don't own.
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
	ctx := r.Context()
	caller, _ := ctx.Value(ctxKeyName).(string)
	role, _ := auth.RoleFromContext(ctx)
	switch {
	case body.Source == "":
		if caller != "" {
			body.Source = caller
		} else {
			body.Source = "adapter"
		}
	case body.Source != caller && role != auth.RoleAdmin:
		writeError(w, http.StatusForbidden, "SOURCE_FORBIDDEN",
			"cannot emit events with a source other than the authenticated identity")
		return
	}
	evt, err := s.eventBus.Publish(ctx, body.Type, body.Source, body.Payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "PUBLISH_FAILED", err.Error())
		return
	}
	writeJSON(w, map[string]any{"id": evt.ID, "accepted_at": evt.CreatedAt})
}

// handleKnowledgeSearch exposes the shared knowledge layer to adapters so they
// can consult prior approaches before acting. Story 10.2.
func (s *Server) handleKnowledgeSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "MISSING_Q", "query parameter q is required")
		return
	}
	limit := 5
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	// Use VectorSearch when an embedder is attached; fall back to keyword.
	store := knowledgeStoreFromDB(s.db())
	results, err := store.VectorSearch(r.Context(), q, limit)
	if err != nil {
		results, err = store.Search(r.Context(), q, limit)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SEARCH_FAILED", err.Error())
		return
	}
	writeJSON(w, results)
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
	ctx := r.Context()
	tenant, ok := requireTenantScope(ctx)
	if !ok {
		writeError(w, http.StatusForbidden, "NO_TENANT", "request has no tenant scope")
		return
	}
	agents, err := s.agentMgr.ListByTenant(ctx, tenant, 1000)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}
	writeJSON(w, agents)
}

func (s *Server) handleListEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenant, ok := requireTenantScope(ctx)
	if !ok {
		writeError(w, http.StatusForbidden, "NO_TENANT", "request has no tenant scope")
		return
	}
	opts := event.QueryOpts{
		Type:     r.URL.Query().Get("type"),
		Source:   r.URL.Query().Get("source"),
		TenantID: tenant,
		Limit:    parseLimit(r, 50, 500),
	}
	if since := r.URL.Query().Get("since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			opts.Since = t
		}
	}

	events, err := s.eventBus.Query(ctx, opts)
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
// Story 8.3 AC1 requires duration + result summary in addition to status/agent.
func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantClause, tenantArgs := tenantFilter(ctx, "t")
	limit := parseLimit(r, 500, 500)
	offset := parseOffset(r)
	args := append([]any{}, tenantArgs...)
	args = append(args, limit, offset)
	rows, err := s.db().QueryContext(ctx,
		`SELECT t.id, t.workflow_id, t.type, t.status,
		        COALESCE(t.agent_id, ''), COALESCE(a.name, ''),
		        t.created_at, COALESCE(t.started_at, ''), COALESCE(t.completed_at, ''),
		        COALESCE(t.output, '')
		 FROM tasks t LEFT JOIN agents a ON a.id = t.agent_id
		 WHERE 1=1`+tenantClause+`
		 ORDER BY t.created_at DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "QUERY_FAILED", err.Error())
		return
	}
	defer rows.Close()

	type taskRow struct {
		ID              string   `json:"id"`
		WorkflowID      string   `json:"workflow_id"`
		Type            string   `json:"type"`
		Status          string   `json:"status"`
		AgentID         string   `json:"agent_id"`
		AgentName       string   `json:"agent_name"`
		CreatedAt       string   `json:"created_at"`
		StartedAt       string   `json:"started_at,omitempty"`
		CompletedAt     string   `json:"completed_at,omitempty"`
		DurationSeconds *float64 `json:"duration_seconds,omitempty"`
		ResultSummary   string   `json:"result_summary,omitempty"`
	}
	var tasks []taskRow
	for rows.Next() {
		var (
			t      taskRow
			output string
		)
		if err := rows.Scan(&t.ID, &t.WorkflowID, &t.Type, &t.Status,
			&t.AgentID, &t.AgentName, &t.CreatedAt,
			&t.StartedAt, &t.CompletedAt, &output); err != nil {
			continue
		}
		if d, ok := taskDurationSeconds(t.StartedAt, t.CompletedAt); ok {
			t.DurationSeconds = &d
		}
		t.ResultSummary = summariseTaskOutput(output)
		tasks = append(tasks, t)
	}
	writeJSON(w, tasks)
}

// taskDurationSeconds returns elapsed time between started_at and completed_at.
// If completed_at is empty but started_at is set, returns elapsed-so-far.
// Returns (0, false) when started_at is missing or unparseable.
func taskDurationSeconds(startedAt, completedAt string) (float64, bool) {
	if startedAt == "" {
		return 0, false
	}
	start, err := parseTaskTime(startedAt)
	if err != nil {
		return 0, false
	}
	end := time.Now().UTC()
	if completedAt != "" {
		if t, err := parseTaskTime(completedAt); err == nil {
			end = t
		}
	}
	d := end.Sub(start).Seconds()
	if d < 0 {
		return 0, false
	}
	return d, true
}

// parseTaskTime accepts both RFC3339 and SQLite's "YYYY-MM-DD HH:MM:SS" format.
func parseTaskTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02 15:04:05", s)
}

// summariseTaskOutput returns a short single-line preview of the task output
// payload for the dashboard. Empty output and unparseable JSON degrade to ''.
func summariseTaskOutput(raw string) string {
	if raw == "" {
		return ""
	}
	// Collapse whitespace and cap at 120 chars.
	const max = 120
	out := strings.Join(strings.Fields(raw), " ")
	if len(out) > max {
		out = out[:max] + "…"
	}
	return out
}

// handleCosts returns per-agent cost summaries and budget alerts.
//
// Runs the four independent queries concurrently. Dashboards poll this every
// 5s; under SQLite WAL or Postgres multiple goroutines can read in parallel,
// so the wall-clock latency drops from Σ(queries) to max(queries). We keep
// a short per-query timeout so a single slow aggregation can't hold the
// response hostage.
func (s *Server) handleCosts(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	db := s.db()
	tClause, tArgs := tenantFilter(ctx, "")

	type summary struct {
		AgentName string  `json:"agent_name"`
		TotalCost float64 `json:"total_cost"`
		TaskCount int     `json:"task_count"`
	}
	type wfSummary struct {
		WorkflowID string  `json:"workflow_id"`
		TotalCost  float64 `json:"total_cost"`
		TaskCount  int     `json:"task_count"`
	}
	type dailyPoint struct {
		Day       string  `json:"day"`
		TotalCost float64 `json:"total_cost"`
	}
	type alert struct {
		AgentName  string  `json:"agent_name"`
		DailyLimit float64 `json:"daily_limit"`
		Spend      float64 `json:"spend"`
		Breached   bool    `json:"breached"`
	}

	var (
		wg          sync.WaitGroup
		summaries   []summary
		perWorkflow []wfSummary
		trend       []dailyPoint
		alerts      []alert
	)

	// Each closure captures its own result slice. Errors are logged but
	// never fail the whole response — a transient DB blip on one section
	// should still render the rest of the dashboard.
	run := func(fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn()
		}()
	}

	run(func() {
		rows, err := db.QueryContext(ctx,
			`SELECT agent_name, SUM(cost), COUNT(*) FROM costs
			 WHERE 1=1`+tClause+`
			 GROUP BY agent_name ORDER BY SUM(cost) DESC`, tArgs...)
		if err != nil {
			slog.Warn("costs: summaries query failed", "error", err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var x summary
			if err := rows.Scan(&x.AgentName, &x.TotalCost, &x.TaskCount); err == nil {
				summaries = append(summaries, x)
			}
		}
	})

	run(func() {
		innerClause, innerArgs := tenantFilter(ctx, "")
		outerClause, outerArgs := tenantFilter(ctx, "b")
		args := append([]any{}, innerArgs...)
		args = append(args, outerArgs...)
		rows, err := db.QueryContext(ctx,
			`SELECT b.agent_name, b.daily_limit,
			        COALESCE((SELECT SUM(cost) FROM costs
			                  WHERE agent_name = b.agent_name
			                    AND date(created_at) = date('now')`+innerClause+`), 0)
			 FROM budget_alerts b WHERE b.enabled = 1`+outerClause, args...)
		if err != nil {
			slog.Warn("costs: alerts query failed", "error", err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var a alert
			if err := rows.Scan(&a.AgentName, &a.DailyLimit, &a.Spend); err == nil {
				a.Breached = a.Spend >= a.DailyLimit
				alerts = append(alerts, a)
			}
		}
	})

	run(func() {
		rows, err := db.QueryContext(ctx,
			`SELECT workflow_id, SUM(cost), COUNT(*) FROM costs
			 WHERE 1=1`+tClause+`
			 GROUP BY workflow_id ORDER BY SUM(cost) DESC LIMIT 50`, tArgs...)
		if err != nil {
			slog.Warn("costs: per-workflow query failed", "error", err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var x wfSummary
			if err := rows.Scan(&x.WorkflowID, &x.TotalCost, &x.TaskCount); err == nil {
				perWorkflow = append(perWorkflow, x)
			}
		}
	})

	run(func() {
		rows, err := db.QueryContext(ctx,
			`SELECT date(created_at), SUM(cost) FROM costs
			 WHERE created_at >= date('now','-14 days')`+tClause+`
			 GROUP BY date(created_at) ORDER BY date(created_at)`, tArgs...)
		if err != nil {
			slog.Warn("costs: trend query failed", "error", err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var p dailyPoint
			if err := rows.Scan(&p.Day, &p.TotalCost); err == nil {
				trend = append(trend, p)
			}
		}
	})

	wg.Wait()

	writeJSON(w, map[string]any{
		"summaries":    summaries,
		"per_workflow": perWorkflow,
		"trend":        trend,
		"alerts":       alerts,
	})
}

// handleMetricsProm renders the same signals in Prometheus text-exposition
// format (Story 6.4 promised for v0.2). Content negotiation in handleMetrics
// delegates here on `?format=prometheus` or Accept: text/plain.
func (s *Server) handleMetricsProm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	agents, _ := s.agentMgr.List(ctx)
	counts := map[string]int{}
	for _, a := range agents {
		counts[a.HealthStatus]++
	}
	fmt.Fprintf(w, "# HELP hive_agents_total Total agents by health status\n# TYPE hive_agents_total gauge\n")
	for status, n := range counts {
		fmt.Fprintf(w, "hive_agents_total{status=%q} %d\n", status, n)
	}

	taskCounts := countRowsByStatus(ctx, s.db(), "tasks")
	fmt.Fprintf(w, "# HELP hive_tasks_total Total tasks by status\n# TYPE hive_tasks_total gauge\n")
	for status, n := range taskCounts {
		fmt.Fprintf(w, "hive_tasks_total{status=%q} %d\n", status, n)
	}

	fmt.Fprintf(w, "# HELP hive_events_last_minute Events published in the last 60s\n# TYPE hive_events_last_minute gauge\n")
	fmt.Fprintf(w, "hive_events_last_minute %d\n", s.countEventsSince(ctx, time.Now().Add(-time.Minute)))

	open := 0
	for _, state := range s.breakers.AllStates() {
		if state == resilience.StateOpen {
			open++
		}
	}
	fmt.Fprintf(w, "# HELP hive_circuit_breakers_open Number of open circuit breakers\n# TYPE hive_circuit_breakers_open gauge\n")
	fmt.Fprintf(w, "hive_circuit_breakers_open %d\n", open)

	var avgDur float64
	_ = s.db().QueryRowContext(ctx,
		`SELECT COALESCE(AVG((JULIANDAY(completed_at) - JULIANDAY(started_at)) * 86400), 0)
		 FROM tasks WHERE status = 'completed' AND started_at IS NOT NULL AND completed_at IS NOT NULL
		 AND created_at >= datetime('now', '-1 day')`).Scan(&avgDur)
	fmt.Fprintf(w, "# HELP hive_avg_task_duration_seconds Average completed-task duration over last 24h\n# TYPE hive_avg_task_duration_seconds gauge\n")
	fmt.Fprintf(w, "hive_avg_task_duration_seconds %f\n", avgDur)
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	// Content negotiation: `?format=prometheus` or Accept: text/plain → Prometheus.
	if r.URL.Query().Get("format") == "prometheus" ||
		strings.Contains(r.Header.Get("Accept"), "text/plain") {
		s.handleMetricsProm(w, r)
		return
	}
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
