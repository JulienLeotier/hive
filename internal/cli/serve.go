package cli

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/JulienLeotier/hive/internal/adapter"
	"github.com/JulienLeotier/hive/internal/agent"
	"github.com/JulienLeotier/hive/internal/api"
	"github.com/JulienLeotier/hive/internal/auth"
	"github.com/JulienLeotier/hive/internal/autonomy"
	"github.com/JulienLeotier/hive/internal/billing"
	"github.com/JulienLeotier/hive/internal/cluster"
	"github.com/JulienLeotier/hive/internal/config"
	"github.com/JulienLeotier/hive/internal/cost"
	"github.com/JulienLeotier/hive/internal/dashboard"
	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/federation"
	"github.com/JulienLeotier/hive/internal/knowledge"
	"github.com/JulienLeotier/hive/internal/market"
	"github.com/JulienLeotier/hive/internal/notify"
	"github.com/JulienLeotier/hive/internal/resilience"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/JulienLeotier/hive/internal/task"
	"github.com/JulienLeotier/hive/internal/tracing"
	"github.com/JulienLeotier/hive/internal/webhook"
	"github.com/JulienLeotier/hive/internal/workflow"
	"github.com/JulienLeotier/hive/internal/ws"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
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

		// OpenTelemetry: initialise before anything else so HTTP/adapter
		// instrumentation has a TracerProvider to register with. No-op when
		// observability.traces is absent and OTEL_EXPORTER_OTLP_ENDPOINT is
		// unset, so dev deployments pay nothing.
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

		// Monthly billing aggregation. Runs once a day (idempotent thanks to
		// the unique (tenant, period) constraint), rolls the previous full
		// calendar month into one invoice per tenant. Gateway wiring
		// (Stripe, etc.) is a separate follow-up — this is the infra that
		// keeps accumulating clean data regardless.
		billingGen := billing.NewGenerator(store.DB, "USD")
		go func() {
			t := time.NewTicker(24 * time.Hour)
			defer t.Stop()
			// Run once at boot so fresh deployments don't wait a day to see
			// last month's invoice. Safe because the generator is idempotent.
			if _, err := billingGen.GenerateLastMonth(supervisorCtx); err != nil {
				slog.Warn("billing: initial generation failed", "error", err)
			}
			for {
				select {
				case <-t.C:
					if _, err := billingGen.GenerateLastMonth(supervisorCtx); err != nil {
						slog.Warn("billing: daily generation failed", "error", err)
					}
				case <-supervisorCtx.Done():
					return
				}
			}
		}()

		// Story 10.1 + 10.3: auto-record knowledge, configurable max-age.
		// Story 16.2: opt-in OpenAI embeddings when knowledge.embedding is set,
		// with HashingEmbedder as the always-present fallback.
		kStore := knowledge.NewStore(store.DB).WithEmbedder(buildEmbedder(cfg.Knowledge))
		if cfg.Knowledge != nil && cfg.Knowledge.MaxAgeDays > 0 {
			kStore.WithMaxAge(time.Duration(cfg.Knowledge.MaxAgeDays) * 24 * time.Hour)
		}
		knowledge.NewAutoRecorder(store.DB, kStore).Attach(bus)

		// Story 18.3: auto-credit tokens to agents on task.completed.
		marketStore := market.NewStore(store.DB).WithBus(bus.PublishErr)
		market.NewAutoCredit(store.DB, marketStore, 1.0).Attach(bus)

		// Story 11.3/11.4: configured webhooks are delivered by subscribing
		// the dispatcher to every event on the bus. Without this, `hive
		// webhook add` stored the config but nothing ever fired.
		webhookDisp := webhook.NewDispatcher(store.DB)
		bus.Subscribe("*", func(e event.Event) {
			webhookDisp.Dispatch(supervisorCtx, e)
		})

		// Email notifier for ops-shaped events. Silent no-op when the config
		// is missing or incomplete; serve keeps booting either way.
		notify.NewNotifier(buildEmailConfig(cfg.Notifications)).Attach(bus)

		// Slack notifier (dedicated ops channel — complements the generic
		// webhook.Dispatcher for users who just want a webhook URL in YAML).
		notify.NewSlackNotifier(buildSlackConfig(cfg.Notifications)).Attach(bus)

		// Epic 4: autonomy. Wake-up cycles drive agent self-assignment of
		// pending tasks, busywork suppression, and decision logging.
		// Without this block, configured agents never pick up pending
		// tasks on their own — they only run when explicitly invoked by a
		// workflow.
		observer := autonomy.NewObserver(store.DB)
		idleTracker := autonomy.NewIdleTracker(3)
		wakeupHandler := autonomy.NewDefaultHandler(observer, router, idleTracker, bus)
		scheduler := autonomy.NewScheduler(wakeupHandler.Handle)
		// Register every currently healthy agent. Future agents added via
		// CLI during runtime will pick up wake-up cycles on the next
		// server restart (a future story can add live subscription).
		{
			agents, err := mgr.List(supervisorCtx)
			if err != nil {
				slog.Warn("autonomy: listing agents for scheduler failed", "error", err)
			} else {
				interval := 30 * time.Second
				if cfg.Autonomy != nil && cfg.Autonomy.HeartbeatSeconds > 0 {
					interval = time.Duration(cfg.Autonomy.HeartbeatSeconds) * time.Second
				}
				for _, a := range agents {
					if a.HealthStatus == "healthy" {
						scheduler.Register(a.Name, interval)
					}
				}
			}
		}
		// Story 4.2 AC: "heartbeats can also be triggered by events".
		// Trigger an immediate wake-up when an agent event arrives so the
		// system reacts faster than the polling interval.
		bus.Subscribe("agent.registered", func(e event.Event) {
			scheduler.TriggerWakeUp(e.Source)
		})
		bus.Subscribe("task.unroutable", func(e event.Event) {
			// Best-effort: nudge every healthy agent to re-evaluate, since
			// we don't know which one might now be capable.
			agents, _ := mgr.List(supervisorCtx)
			for _, a := range agents {
				if a.HealthStatus == "healthy" {
					scheduler.TriggerWakeUp(a.Name)
				}
			}
		})
		defer scheduler.StopAll()

		// Story 22.2/22.3: advertise this node in the cluster roster. Without
		// a periodic heartbeat, the /cluster dashboard is always empty and
		// peer nodes can't detect when this one goes away.
		nodeID := "local"
		if cfg.Cluster != nil && cfg.Cluster.NodeID != "" {
			nodeID = cfg.Cluster.NodeID
		}
		hostname, _ := os.Hostname()
		selfNode := &cluster.Node{
			ID:        nodeID,
			Hostname:  hostname,
			Address:   fmt.Sprintf(":%d", cfg.Port),
			Status:    "active",
			StartedAt: time.Now(),
		}
		roster := cluster.NewRoster(store.DB)
		if err := roster.Heartbeat(supervisorCtx, selfNode); err != nil {
			slog.Warn("cluster heartbeat: initial upsert failed", "error", err)
		}
		go func() {
			t := time.NewTicker(15 * time.Second)
			defer t.Stop()
			for {
				select {
				case <-t.C:
					if err := roster.Heartbeat(supervisorCtx, selfNode); err != nil {
						slog.Warn("cluster heartbeat failed", "error", err)
					}
					// Mark nodes silent for >2 heartbeats as offline so the
					// /cluster view stays honest.
					if _, err := roster.MarkStale(supervisorCtx, 45*time.Second); err != nil {
						slog.Warn("cluster stale-scan failed", "error", err)
					}
				case <-supervisorCtx.Done():
					return
				}
			}
		}()

		// Story 3.4 AC: workflows declared with `trigger: {type: schedule|webhook}`
		// need an active dispatcher. Without this block, scheduled workflows
		// never fire and webhook paths are unknown to the HTTP server. YAML
		// files are discovered under `${data_dir}/workflows/*.yaml`; missing
		// dir = no-op.
		wfStore := workflow.NewStore(store.DB, bus)
		triggerMgr := workflow.NewTriggerManager(func(ctx context.Context, wfCfg *workflow.Config, payload workflow.TriggerPayload) error {
			engine := workflow.NewEngine(wfStore, taskStore, router, bus)
			engine.WithMarketStore(market.NewStore(store.DB).WithBus(bus.PublishErr))
			engine.WithAgentLookup(buildAgentLookup(mgr))
			_, err := engine.Run(ctx, wfCfg)
			return err
		})
		defer triggerMgr.Stop()
		workflowsDir := filepath.Join(cfg.DataDir, "workflows")
		if entries, err := os.ReadDir(workflowsDir); err == nil {
			registered := 0
			for _, e := range entries {
				if e.IsDir() || (filepath.Ext(e.Name()) != ".yaml" && filepath.Ext(e.Name()) != ".yml") {
					continue
				}
				wfPath := filepath.Join(workflowsDir, e.Name())
				wfCfg, err := workflow.ParseFile(wfPath)
				if err != nil {
					slog.Warn("workflow parse failed", "file", wfPath, "error", err)
					continue
				}
				if err := triggerMgr.Register(supervisorCtx, wfCfg); err != nil {
					slog.Warn("workflow trigger register failed", "file", wfPath, "error", err)
					continue
				}
				registered++
			}
			if registered > 0 {
				slog.Info("workflow triggers armed", "dir", workflowsDir, "count", registered)
			}
		}

		keyMgr := api.NewKeyManager(store.DB)
		users := auth.NewUserStore(store.DB)

		apiSrv := api.NewServer(mgr, bus, breakers, keyMgr).
			WithUsers(users).
			WithTriggerManager(triggerMgr).
			WithWebhookDispatcher(webhookDisp).
			WithBillingGenerator(billingGen)

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

		// Webhook triggers — unauthenticated by design (each workflow declares
		// its own HMAC secret via trigger.secret). Path must match the
		// `webhook:` field of a registered workflow trigger.
		mux.Handle("/hooks/", api.Instrument("/hooks/", workflow.WebhookHandler(triggerMgr)))

		// Dashboard (static, no auth)
		mux.Handle("/", dashboard.Handler())

		addr := fmt.Sprintf(":%d", cfg.Port)
		// otelhttp.NewHandler creates a span per incoming request carrying
		// method/route/status so a trace backend can stitch a customer's
		// journey across every Hive call without additional glue.
		tracedMux := otelhttp.NewHandler(mux, "hive.http",
			otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
				return r.Method + " " + r.URL.Path
			}))

		httpSrv := &http.Server{
			Addr:    addr,
			Handler: api.SecurityHeaders(tracedMux),
			// Slowloris guard: cap how long a client has to send its request
			// headers. Without this, an attacker can hold a connection open
			// indefinitely by dribbling out bytes and exhaust the listener.
			ReadHeaderTimeout: 15 * time.Second,
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

// buildEmbedder picks the knowledge embedder based on config. Default is
// HashingEmbedder (no external dependency). When knowledge.embedding is set
// with provider=openai and an API key, switches to OpenAIEmbedder with
// HashingEmbedder as the fallback for transient API failures.
func buildEmbedder(cfg *config.KnowledgeBlock) knowledge.Embedder {
	hashing := knowledge.NewHashingEmbedder(128)
	if cfg == nil || cfg.Embedding == nil {
		return hashing
	}
	emb := cfg.Embedding
	if emb.Provider == "openai" && emb.APIKey != "" {
		slog.Info("knowledge embedder: openai", "model", emb.Model)
		return knowledge.NewOpenAIEmbedder(emb.APIKey, emb.Model, hashing)
	}
	return hashing
}

// buildEmailConfig turns the YAML config into the shape notify.NewNotifier
// expects, resolving PasswordEnv via the process environment. When the block
// is missing or incomplete, the returned EmailConfig.Enabled() is false and
// the notifier becomes a no-op.
func buildEmailConfig(cfg *config.NotificationsBlock) notify.EmailConfig {
	if cfg == nil || cfg.Email == nil {
		return notify.EmailConfig{}
	}
	e := cfg.Email
	password := ""
	if e.PasswordEnv != "" {
		password = os.Getenv(e.PasswordEnv)
	}
	return notify.EmailConfig{
		Host:        e.Host,
		Port:        e.Port,
		From:        e.From,
		To:          e.To,
		Username:    e.Username,
		Password:    password,
		StartTLS:    e.StartTLS,
		SMTPSOnly:   e.SMTPSOnly,
		TimeoutSecs: e.TimeoutSecs,
	}
}

// buildTracingConfig resolves the YAML observability.traces block into the
// shape tracing.Setup expects. Returns a zero Config when the block is
// missing, which makes tracing.Setup a no-op.
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

// buildSlackConfig mirrors buildEmailConfig for the Slack ops channel.
func buildSlackConfig(cfg *config.NotificationsBlock) notify.SlackConfig {
	if cfg == nil || cfg.Slack == nil {
		return notify.SlackConfig{}
	}
	return notify.SlackConfig{
		WebhookURL:  cfg.Slack.WebhookURL,
		TimeoutSecs: cfg.Slack.TimeoutSecs,
	}
}

// buildAgentLookup wraps the agent manager so the workflow engine can resolve
// agent ID → AgentSpec at dispatch time. Same shape as the inline closure in
// `hive run`, factored out because `hive serve` also needs it for triggered
// workflow runs.
func buildAgentLookup(mgr *agent.Manager) workflow.AgentLookup {
	return func(ctx context.Context, agentID string) (adapter.AgentSpec, error) {
		a, err := mgr.GetByID(ctx, agentID)
		if err != nil {
			return adapter.AgentSpec{}, err
		}
		return adapter.AgentSpec{
			Name:         a.Name,
			Type:         a.Type,
			Config:       a.Config,
			Capabilities: a.Capabilities,
		}, nil
	}
}
