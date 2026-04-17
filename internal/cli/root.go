package cli

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

var logLevel string

var rootCmd = &cobra.Command{
	Use:   "hive",
	Short: "Hive — Usine à produits BMAD en local",
	Long: `Hive drive la chaîne BMAD-METHOD de bout en bout via Claude Code :
idée → PRD → architecture → stories → implémentation → revue → retrospective.
Single binary, single user, SQLite par défaut.`,
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
