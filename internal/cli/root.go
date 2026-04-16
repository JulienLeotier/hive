package cli

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

var logLevel string

var rootCmd = &cobra.Command{
	Use:   "hive",
	Short: "Hive — Universal AI agent orchestration platform",
	Long:  "Hive orchestrates AI agents from any framework through a standardized open protocol.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		initLogging(logLevel)
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func initLogging(level string) {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
	slog.SetDefault(slog.New(handler))
}
