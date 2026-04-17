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

	"github.com/JulienLeotier/hive/internal/agent"
	"github.com/JulienLeotier/hive/internal/api"
	"github.com/JulienLeotier/hive/internal/auth"
	"github.com/JulienLeotier/hive/internal/config"
	"github.com/JulienLeotier/hive/internal/cost"
	"github.com/JulienLeotier/hive/internal/dashboard"
	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/federation"
	"github.com/JulienLeotier/hive/internal/knowledge"
	"github.com/JulienLeotier/hive/internal/market"
	"github.com/JulienLeotier/hive/internal/resilience"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/JulienLeotier/hive/internal/task"
	"github.com/JulienLeotier/hive/internal/ws"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the Hive API server and dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load("hive.yaml")
		if err != nil {
			return err
		}

		// Story 22.3: apply cluster routing config.
		if cfg.Cluster != nil {
			if cfg.Cluster.NodeID != "" {
				task.LocalNodeID = cfg.Cluster.NodeID
				agent.LocalNodeID = cfg.Cluster.NodeID
			}
			if cfg.Cluster.Routing != "" {
				task.RoutingMode = cfg.Cluster.Routing
			}
		}

		// Story 22.1: dispatch storage backend from config.
		store, err := storage.Open2(storage.Backend{
			Type:        cfg.Storage,
			DataDir:     cfg.DataDir,
			PostgresURL: cfg.PostgresURL,
		})
		if err != nil {
			return err
		}
		defer store.Close()

		// Story 15.2/22.2: switch to NATS-bridged bus when configured. Local
		// SQLite bus remains the source of truth for durable query while NATS
		// carries the real-time fan-out for peer nodes.
		var bus *event.Bus
		if cfg.EventBus != nil && cfg.EventBus.Backend == "nats" && cfg.EventBus.NATSURL != "" {
			nc, err := event.NewNATSConnFromURL(cfg.EventBus.NATSURL)
			if err != nil {
				return fmt.Errorf("connecting to nats: %w", err)
			}
			subject := cfg.EventBus.Subject
			if subject == "" {
				subject = "hive.events"
			}
			natsBus, err := event.NewNATSBus(nc, event.NATSConfig{Subject: subject, MaxHistory: 1000})
			if err != nil {
				return fmt.Errorf("initialising nats bus: %w", err)
			}
			bus = event.NewBus(store.DB)
			bus.Subscribe("*", func(e event.Event) {
				_, _ = natsBus.Publish(context.Background(), e.Type, e.Source, e.Payload)
			})
			slog.Info("event bus: nats bridged", "url", cfg.EventBus.NATSURL, "subject", subject)
		} else {
			bus = event.NewBus(store.DB)
		}

		// Story 22.2: wire agent manager to publish lifecycle events. When a
		// NATS bus is configured this fan-out reaches peer nodes automatically.
		mgr := agent.NewManager(store.DB).WithPublisher(bus.PublishErr)

		// Story 5.1 AC: breaker threshold + reset are configurable.
		breakerCfg := resilience.DefaultBreakerConfig()
		if cfg.Breaker != nil {
			if cfg.Breaker.Threshold > 0 {
				breakerCfg.Threshold = cfg.Breaker.Threshold
			}
			if cfg.Breaker.ResetTimeoutSeconds > 0 {
				breakerCfg.ResetTimeout = time.Duration(cfg.Breaker.ResetTimeoutSeconds) * time.Second
			}
		}
		breakers := resilience.NewBreakerRegistry(breakerCfg)

		// Story 19.3: federation resolver + proxy. When no local agent is
		// capable, Router.WithFederation hands control to the resolver.
		fedStore := federation.NewStore(store.DB)

		// A3 follow-up: if HIVE_MASTER_KEY is set, warn about any plaintext
		// cert material still sitting in federation_links so the operator
		// can rotate it.
		if n, err := fedStore.AuditEncryptionAtRest(context.Background()); err != nil {
			slog.Warn("federation encryption audit failed", "error", err)
		} else if n > 0 {
			slog.Warn("federation peers have plaintext TLS material at rest", "count", n)
		}

		fedResolver, fedProxy := federation.NewResolver(context.Background(), fedStore)
		router := task.NewRouter(store.DB).WithBus(bus).WithFederation(
			func(ctx context.Context, taskType string) (string, string, bool) {
				return fedResolver(ctx, taskType)
			},
		)
		_ = fedProxy // kept alive as long as serve is running

		// Auto-isolate agents and failover their tasks when the breaker opens.
		watcher := agent.NewHealthWatcher(mgr, router, bus)
		breakers.OnStateChange(watcher.Hook())

		// Periodic checkpoint supervisor — reassigns tasks whose checkpoint has gone stale.
		// Story 2.6 AC: interval is configurable.
		taskStore := task.NewStore(store.DB, bus)
		supervisorCtx, supervisorCancel := context.WithCancel(context.Background())
		defer supervisorCancel()
		interval := 30 * time.Second
		maxAge := 5 * time.Minute
		if cfg.Checkpoint != nil {
			if cfg.Checkpoint.IntervalSeconds > 0 {
				interval = time.Duration(cfg.Checkpoint.IntervalSeconds) * time.Second
			}
			if cfg.Checkpoint.MaxAgeSeconds > 0 {
				maxAge = time.Duration(cfg.Checkpoint.MaxAgeSeconds) * time.Second
			}
		}
		supervisor := task.NewCheckpointSupervisor(taskStore, router, interval, maxAge)
		supervisor.Start(supervisorCtx)
		defer supervisor.Stop()

		// Retention janitor — deletes old rows from append-only tables on a
		// timer. Safe to always start; tables stay within bounds even on
		// fresh deployments, and config.retention can tune or disable each
		// window. Uses supervisorCtx so it stops with the server.
		{
			r := storage.RetentionConfig{Interval: time.Hour}
			if cfg.Retention != nil {
				r = storage.RetentionConfig{
					EventsDays:         cfg.Retention.EventsMaxAgeDays,
					CompletedTasksDays: cfg.Retention.CompletedTasksMaxAgeDays,
					CostsDays:          cfg.Retention.CostsMaxAgeDays,
					AuditDays:          cfg.Retention.AuditMaxAgeDays,
					Interval:           time.Duration(cfg.Retention.IntervalMinutes) * time.Minute,
				}
			}
			storage.RunRetention(supervisorCtx, store.DB, r)
		}

		// Cost tracker with bus so budget breaches emit cost.alert events.
		_ = cost.NewTracker(store.DB).WithBus(bus.PublishErr)

		// Story 10.1 + 10.3: auto-record knowledge, configurable max-age.
		kStore := knowledge.NewStore(store.DB).WithEmbedder(knowledge.NewHashingEmbedder(128))
		if cfg.Knowledge != nil && cfg.Knowledge.MaxAgeDays > 0 {
			kStore.WithMaxAge(time.Duration(cfg.Knowledge.MaxAgeDays) * 24 * time.Hour)
		}
		knowledge.NewAutoRecorder(store.DB, kStore).Attach(bus)

		// Story 18.3: auto-credit tokens to agents on task.completed.
		marketStore := market.NewStore(store.DB).WithBus(bus.PublishErr)
		market.NewAutoCredit(store.DB, marketStore, 1.0).Attach(bus)

		keyMgr := api.NewKeyManager(store.DB)
		users := auth.NewUserStore(store.DB)

		apiSrv := api.NewServer(mgr, bus, breakers, keyMgr).WithUsers(users)

		// Story 19.2: honour the `federation.share:` list so only whitelisted
		// capabilities appear at /api/v1/capabilities.
		if cfg.Federation != nil && len(cfg.Federation.Share) > 0 {
			apiSrv.SetFederationShared(cfg.Federation.Share)
		}

		// Story 21.1: wire OIDC provider if configured.
		if cfg.OIDC != nil && cfg.OIDC.Issuer != "" {
			provider, err := auth.NewOIDCProvider(context.Background(), auth.OIDCConfig{
				Issuer:       cfg.OIDC.Issuer,
				ClientID:     cfg.OIDC.ClientID,
				ClientSecret: cfg.OIDC.ClientSecret,
				RedirectURL:  cfg.OIDC.RedirectURL,
				Scopes:       cfg.OIDC.Scopes,
			})
			if err != nil {
				slog.Warn("oidc disabled — discovery failed", "error", err)
			} else {
				apiSrv.WithOIDC(provider)
				slog.Info("oidc enabled", "issuer", cfg.OIDC.Issuer)
			}
		}

		// WebSocket hub — broadcast events to dashboard clients
		hub := ws.NewHub()
		bus.Subscribe("*", func(e event.Event) {
			hub.Broadcast(e)
		})

		mux := http.NewServeMux()

		// Health endpoints — unauthenticated, container-probe shaped.
		// /healthz: liveness, /readyz: DB reachable within 2s.
		mux.Handle("/healthz", api.HealthHandler())
		mux.Handle("/readyz", api.ReadyHandler(store.DB))

		// Prometheus scrape endpoint — unauthenticated (standard scraper
		// convention). Emits gauges + request counters + latency histogram.
		// If you need auth, terminate TLS at an ingress and restrict by IP.
		mux.Handle("/metrics", apiSrv.PromHandler())

		// WebSocket endpoint — gated by the same auth policy as the REST API
		// (API key or OIDC JWT). Token may ride in the Authorization header or
		// as ?token= query param since browsers can't send headers on upgrade.
		mux.Handle("/ws", api.Instrument("/ws", apiSrv.WSHandler(http.HandlerFunc(hub.HandleWS))))

		// API routes (authenticated). Instrument at the /api/ prefix so every
		// REST call counts without wiring each handler individually.
		mux.Handle("/api/", api.Instrument("/api/", apiSrv.Handler()))

		// Dashboard (static, no auth)
		mux.Handle("/", dashboard.Handler())

		addr := fmt.Sprintf(":%d", cfg.Port)
		httpSrv := &http.Server{
			Addr:    addr,
			Handler: api.SecurityHeaders(mux),
		}

		// Graceful shutdown
		done := make(chan os.Signal, 1)
		signal.Notify(done, os.Interrupt, syscall.SIGTERM)

		scheme := "http"
		if cfg.TLS.Enabled() {
			scheme = "https"
		}

		// A7: SQLite serializes writers, so concurrent supervisor + scheduler +
		// API writes contend for the same lock. This is fine for dev and small
		// single-node deployments; it will fall over under prod load. Surface
		// a loud startup warning so operators see it in the first log line.
		if cfg.Storage != "postgres" {
			if env := os.Getenv("HIVE_ENV"); env == "prod" || env == "production" {
				slog.Warn("SQLite storage in production — concurrent writers will block on SQLITE_BUSY. Set storage=postgres for multi-writer workloads.")
			} else {
				slog.Info("storage backend: sqlite (single-writer, suitable for dev and small single-node deployments)")
			}
		}

		go func() {
			slog.Info("hive server started",
				"addr", addr,
				"dashboard", fmt.Sprintf("%s://localhost:%d", scheme, cfg.Port),
				"storage", storageLabel(cfg),
				"tls", cfg.TLS.Enabled())
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
