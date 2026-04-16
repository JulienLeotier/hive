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

		bus := event.NewBus(store.DB)

		// Story 22.2: wire agent manager to publish lifecycle events. When a
		// NATS bus is configured this fan-out reaches peer nodes automatically.
		mgr := agent.NewManager(store.DB).WithPublisher(bus.PublishErr)

		breakers := resilience.NewBreakerRegistry(resilience.DefaultBreakerConfig())

		// Story 19.3: federation resolver + proxy. When no local agent is
		// capable, Router.WithFederation hands control to the resolver.
		fedStore := federation.NewStore(store.DB)
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
		taskStore := task.NewStore(store.DB, bus)
		supervisorCtx, supervisorCancel := context.WithCancel(context.Background())
		defer supervisorCancel()
		supervisor := task.NewCheckpointSupervisor(taskStore, router, 30*time.Second, 5*time.Minute)
		supervisor.Start(supervisorCtx)
		defer supervisor.Stop()

		// Cost tracker with bus so budget breaches emit cost.alert events.
		_ = cost.NewTracker(store.DB).WithBus(bus.PublishErr)

		// Story 10.1: auto-record knowledge on every task completion/failure.
		kStore := knowledge.NewStore(store.DB).WithEmbedder(knowledge.NewHashingEmbedder(128))
		knowledge.NewAutoRecorder(store.DB, kStore).Attach(bus)

		keyMgr := api.NewKeyManager(store.DB)
		users := auth.NewUserStore(store.DB)

		apiSrv := api.NewServer(mgr, bus, breakers, keyMgr).WithUsers(users)

		// WebSocket hub — broadcast events to dashboard clients
		hub := ws.NewHub()
		bus.Subscribe("*", func(e event.Event) {
			hub.Broadcast(e)
		})

		mux := http.NewServeMux()

		// WebSocket endpoint
		mux.HandleFunc("/ws", hub.HandleWS)

		// API routes (authenticated)
		mux.Handle("/api/", apiSrv.Handler())

		// Dashboard (static, no auth)
		mux.Handle("/", dashboard.Handler())

		addr := fmt.Sprintf(":%d", cfg.Port)
		httpSrv := &http.Server{Addr: addr, Handler: mux}

		// Graceful shutdown
		done := make(chan os.Signal, 1)
		signal.Notify(done, os.Interrupt, syscall.SIGTERM)

		go func() {
			slog.Info("hive server started",
				"addr", addr,
				"dashboard", fmt.Sprintf("http://localhost:%d", cfg.Port),
				"storage", storageLabel(cfg))
			if err := httpSrv.ListenAndServe(); err != http.ErrServerClosed {
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
