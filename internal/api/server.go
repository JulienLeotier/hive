package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/JulienLeotier/hive/internal/agent"
	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/resilience"
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

// Server holds all dependencies for the HTTP API.
type Server struct {
	agentMgr *agent.Manager
	eventBus *event.Bus
	breakers *resilience.BreakerRegistry
	keyMgr   *KeyManager
	mux      *http.ServeMux
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

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/v1/agents", s.handleListAgents)
	s.mux.HandleFunc("GET /api/v1/events", s.handleListEvents)
	s.mux.HandleFunc("GET /api/v1/metrics", s.handleMetrics)
}

// Handler returns the HTTP handler with auth middleware.
func (s *Server) Handler() http.Handler {
	return AuthMiddleware(s.keyMgr)(s.mux)
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
	writeJSON(w, events)
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
		"timestamp": time.Now().UTC().Format(time.RFC3339),
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
