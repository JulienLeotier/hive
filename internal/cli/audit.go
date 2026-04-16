package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/JulienLeotier/hive/internal/audit"
	"github.com/JulienLeotier/hive/internal/auth"
	"github.com/JulienLeotier/hive/internal/config"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/spf13/cobra"
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Inspect or export the audit log",
}

var auditListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show recent audit entries",
	RunE: func(cmd *cobra.Command, args []string) error {
		since, _ := cmd.Flags().GetString("since")
		limit, _ := cmd.Flags().GetInt("limit")
		format, _ := cmd.Flags().GetString("format")

		sinceT := time.Now().Add(-24 * time.Hour)
		if since != "" {
			if d, err := time.ParseDuration(since); err == nil {
				sinceT = time.Now().Add(-d)
			}
		}

		cfg, err := config.Load("hive.yaml")
		if err != nil {
			return err
		}
		store, err := storage.Open(cfg.DataDir)
		if err != nil {
			return err
		}
		defer store.Close()

		logger := audit.NewLogger(store.DB)
		entries, err := logger.Query(context.Background(), sinceT, limit)
		if err != nil {
			return err
		}

		switch format {
		case "json":
			data, err := logger.ExportJSON(entries)
			if err != nil {
				return err
			}
			fmt.Println(string(data))
		case "csv":
			fmt.Print(logger.ExportCSV(entries))
		default:
			if len(entries) == 0 {
				fmt.Println("No audit entries.")
				return nil
			}
			fmt.Printf("%-20s %-15s %-15s %s\n", "TIME", "ACTOR", "ACTION", "RESOURCE")
			for _, e := range entries {
				fmt.Printf("%-20s %-15s %-15s %s\n", e.CreatedAt.Format("2006-01-02 15:04:05"), e.Actor, e.Action, e.Resource)
			}
		}
		return nil
	},
}

var auditExportCmd = &cobra.Command{
	Use:   "export [path]",
	Short: "Export audit entries to a file (JSON or CSV by extension, --format override available)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		since, _ := cmd.Flags().GetString("since")
		sinceT := time.Now().Add(-30 * 24 * time.Hour)
		if since != "" {
			if d, err := time.ParseDuration(since); err == nil {
				sinceT = time.Now().Add(-d)
			}
		}

		cfg, err := config.Load("hive.yaml")
		if err != nil {
			return err
		}
		store, err := storage.Open(cfg.DataDir)
		if err != nil {
			return err
		}
		defer store.Close()

		logger := audit.NewLogger(store.DB)
		entries, err := logger.Query(context.Background(), sinceT, 100000)
		if err != nil {
			return err
		}

		// Story 21.3: support --output (preferred) or a positional [path].
		path, _ := cmd.Flags().GetString("output")
		if path == "" && len(args) > 0 {
			path = args[0]
		}
		format, _ := cmd.Flags().GetString("format")
		if format == "" {
			if hasExt(path, ".csv") {
				format = "csv"
			} else {
				format = "json"
			}
		}

		var data []byte
		switch format {
		case "csv":
			data = []byte(logger.ExportCSV(entries))
		case "json":
			data, err = logger.ExportJSON(entries)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown format %q — use json or csv", format)
		}

		if path == "" {
			_, err = cmd.OutOrStdout().Write(data)
			return err
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
		fmt.Printf("Exported %d entries → %s\n", len(entries), path)
		return nil
	},
}

var usersCmd = &cobra.Command{
	Use:   "users",
	Short: "Manage RBAC users",
}

var usersAddCmd = &cobra.Command{
	Use:   "add [subject] [role]",
	Short: "Create or update a user (role: admin|operator|viewer)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		tenant, _ := cmd.Flags().GetString("tenant")
		if tenant == "" {
			tenant = "default"
		}
		if !auth.IsValidRole(args[1]) {
			return fmt.Errorf("invalid role %q", args[1])
		}

		cfg, err := config.Load("hive.yaml")
		if err != nil {
			return err
		}
		store, err := storage.Open(cfg.DataDir)
		if err != nil {
			return err
		}
		defer store.Close()

		us := auth.NewUserStore(store.DB)
		if err := us.Upsert(context.Background(), auth.UserRecord{
			Subject: args[0], Role: auth.Role(args[1]), TenantID: tenant,
		}); err != nil {
			return err
		}
		fmt.Printf("User %s: role=%s tenant=%s\n", args[0], args[1], tenant)
		return nil
	},
}

var usersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List RBAC users",
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

		us := auth.NewUserStore(store.DB)
		users, err := us.List(context.Background())
		if err != nil {
			return err
		}
		if len(users) == 0 {
			fmt.Println("No users registered.")
			return nil
		}
		fmt.Printf("%-30s %-10s %s\n", "SUBJECT", "ROLE", "TENANT")
		for _, u := range users {
			fmt.Printf("%-30s %-10s %s\n", u.Subject, u.Role, u.TenantID)
		}
		return nil
	},
}

func hasExt(path, ext string) bool {
	if len(path) < len(ext) {
		return false
	}
	return path[len(path)-len(ext):] == ext
}

func init() {
	auditListCmd.Flags().String("since", "24h", "time window (e.g., 1h, 7d)")
	auditListCmd.Flags().Int("limit", 50, "max entries to return")
	auditListCmd.Flags().String("format", "table", "output format (table|json|csv)")
	auditExportCmd.Flags().String("since", "30d", "time window (e.g., 1h, 7d, 30d)")
	auditExportCmd.Flags().String("format", "", "output format (json|csv); inferred from --output extension when empty")
	auditExportCmd.Flags().String("output", "", "output file path (prints to stdout when omitted)")

	auditCmd.AddCommand(auditListCmd)
	auditCmd.AddCommand(auditExportCmd)

	usersAddCmd.Flags().String("tenant", "default", "tenant ID")
	usersCmd.AddCommand(usersAddCmd)
	usersCmd.AddCommand(usersListCmd)

	rootCmd.AddCommand(auditCmd)
	rootCmd.AddCommand(usersCmd)
}
