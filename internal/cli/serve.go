package cli

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JulienLeotier/hive/internal/api"
	"github.com/JulienLeotier/hive/internal/config"
	"github.com/JulienLeotier/hive/internal/dashboard"
	"github.com/JulienLeotier/hive/internal/devloop"
	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/intake"
	"github.com/JulienLeotier/hive/internal/project"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/JulienLeotier/hive/internal/tracing"
	"github.com/JulienLeotier/hive/internal/ws"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// serveCmd starts the BMAD product factory: storage, event bus, devloop
// supervisor, HTTP API + dashboard, WebSocket hub. Single binary, single
// process, single user — no auth, no cluster, no federation.
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Hive API server and dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load("hive.yaml")
		if err != nil {
			return err
		}

		// OpenTelemetry: initialise before the HTTP handler so otelhttp
		// has a TracerProvider to register with. No-op when unconfigured.
		traceShutdown, err := tracing.Setup(context.Background(), buildTracingConfig(cfg.Observability))
		if err != nil {
			slog.Warn("tracing setup failed — continuing without traces", "error", err)
			traceShutdown = func(context.Context) error { return nil }
		}
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = traceShutdown(ctx)
		}()

		store, err := storage.Open2(storage.Backend{
			Type:        cfg.Storage,
			DataDir:     cfg.DataDir,
			PostgresURL: cfg.PostgresURL,
		})
		if err != nil {
			return err
		}
		defer store.Close()

		bus := event.NewBus(store.DB)

		// Background retention sweeps (events, audit) keep the DB bounded.
		supervisorCtx, supervisorCancel := context.WithCancel(context.Background())
		defer supervisorCancel()
		{
			r := storage.RetentionConfig{Interval: time.Hour}
			if cfg.Retention != nil {
				r = storage.RetentionConfig{
					EventsDays: cfg.Retention.EventsMaxAgeDays,
					AuditDays:  cfg.Retention.AuditMaxAgeDays,
					Interval:   time.Duration(cfg.Retention.IntervalMinutes) * time.Minute,
				}
			}
			storage.RunRetention(supervisorCtx, store.DB, r)
		}

		// BMAD dev/review loop. Polls for `building` projects and
		// advances one story per tick until every AC passes. Honours:
		//   HIVE_DEV_AGENT=scripted        — force scripted agents
		//   HIVE_DEVLOOP_INTERVAL=<dur>    — override default 10s tick
		devAgent := devloop.NewClaudeCodeDev()
		reviewerAgent := devloop.NewClaudeCodeReviewer()
		if os.Getenv("HIVE_DEV_AGENT") == "scripted" {
			devAgent = devloop.NewScriptedDev()
			reviewerAgent = devloop.NewScriptedReviewer()
		}
		loopInterval := 10 * time.Second
		if v := os.Getenv("HIVE_DEVLOOP_INTERVAL"); v != "" {
			if d, err := time.ParseDuration(v); err == nil && d > 0 {
				loopInterval = d
			}
		}
		devloop.NewSupervisor(store.DB, devAgent, reviewerAgent, loopInterval).
			WithPublisher(devloop.Publisher(bus.PublishErr)).
			Start(supervisorCtx)
		slog.Info("devloop supervisor armed",
			"dev", devAgent.Name(), "reviewer", reviewerAgent.Name(), "interval", loopInterval)

		apiSrv := api.NewServer(bus).
			WithProjectStore(project.NewStore(store.DB)).
			WithIntakeStore(intake.NewStore(store.DB))

		// Pick up any projects that were mid-architect when the previous
		// server process died. Re-runs runArchitectAsync in a detached
		// goroutine so the UI unblocks without operator intervention.
		if err := apiSrv.RecoverStuckPlanning(supervisorCtx); err != nil {
			slog.Warn("architect crash-recovery sweep failed", "error", err)
		}

		hub := ws.NewHub()
		bus.Subscribe("*", func(e event.Event) {
			hub.Broadcast(e)
		})

		mux := http.NewServeMux()
		mux.Handle("/healthz", api.HealthHandler())
		mux.Handle("/readyz", api.ReadyHandler(store.DB))
		mux.Handle("/ws", apiSrv.WSHandler(http.HandlerFunc(hub.HandleWS)))
		mux.Handle("/api/", apiSrv.Handler())
		mux.Handle("/", dashboard.Handler())

		addr := fmt.Sprintf(":%d", cfg.Port)
		tracedMux := otelhttp.NewHandler(mux, "hive.http",
			otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
				return r.Method + " " + r.URL.Path
			}))

		httpSrv := &http.Server{
			Addr:              addr,
			Handler:           api.SecurityHeaders(tracedMux),
			ReadHeaderTimeout: 15 * time.Second,
		}

		done := make(chan os.Signal, 1)
		signal.Notify(done, os.Interrupt, syscall.SIGTERM)

		scheme := "http"
		if cfg.TLS.Enabled() {
			scheme = "https"
		}

		go func() {
			slog.Info("hive server started",
				"addr", addr,
				"dashboard", fmt.Sprintf("%s://localhost:%d", scheme, cfg.Port),
				"storage", storageLabel(cfg))
			var err error
			if cfg.TLS.Enabled() {
				err = httpSrv.ListenAndServeTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile)
			} else {
				err = httpSrv.ListenAndServe()
			}
			if err != http.ErrServerClosed {
				slog.Error("server error", "error", err)
			}
		}()

		<-done
		slog.Info("shutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpSrv.Shutdown(ctx)
	},
}

func storageLabel(cfg config.Config) string {
	if cfg.Storage == "postgres" {
		return "postgres"
	}
	return "sqlite:" + cfg.DataDir
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

// buildTracingConfig resolves the YAML observability.traces block into
// the shape tracing.Setup expects. Zero Config when unset.
func buildTracingConfig(cfg *config.ObservabilityBlock) tracing.Config {
	if cfg == nil || cfg.Traces == nil {
		return tracing.Config{}
	}
	t := cfg.Traces
	return tracing.Config{
		Enabled:        t.Enabled,
		Endpoint:       t.Endpoint,
		Protocol:       t.Protocol,
		SampleRatio:    t.SampleRatio,
		ServiceVersion: t.Version,
	}
}
