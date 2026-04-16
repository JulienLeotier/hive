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
	"github.com/JulienLeotier/hive/internal/config"
	"github.com/JulienLeotier/hive/internal/dashboard"
	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/resilience"
	"github.com/JulienLeotier/hive/internal/storage"
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

		store, err := storage.Open(cfg.DataDir)
		if err != nil {
			return err
		}
		defer store.Close()

		mgr := agent.NewManager(store.DB)
		bus := event.NewBus(store.DB)
		breakers := resilience.NewBreakerRegistry(resilience.DefaultBreakerConfig())
		keyMgr := api.NewKeyManager(store.DB)

		srv := api.NewServer(mgr, bus, breakers, keyMgr)

		mux := http.NewServeMux()

		// API routes (authenticated)
		mux.Handle("/api/", srv.Handler())

		// Dashboard (static, no auth)
		mux.Handle("/", dashboard.Handler())

		addr := fmt.Sprintf(":%d", cfg.Port)
		httpSrv := &http.Server{Addr: addr, Handler: mux}

		// Graceful shutdown
		done := make(chan os.Signal, 1)
		signal.Notify(done, os.Interrupt, syscall.SIGTERM)

		go func() {
			slog.Info("hive server started", "addr", addr, "dashboard", fmt.Sprintf("http://localhost:%d", cfg.Port))
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

func init() {
	rootCmd.AddCommand(serveCmd)
}
