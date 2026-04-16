package cli

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/JulienLeotier/hive/internal/config"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/oklog/ulid/v2"
	"github.com/spf13/cobra"
)

var tenantCmd = &cobra.Command{
	Use:   "tenant",
	Short: "Manage tenants (Story 21.4 multi-tenant support)",
}

var tenantCreateCmd = &cobra.Command{
	Use:   "create [tenant-id]",
	Short: "Create a new tenant",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tenantID := args[0]
		if tenantID == "" {
			return fmt.Errorf("tenant id is required")
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

		// Ensure the tenants bookkeeping table exists (self-heal — we didn't
		// carve one out in migration 006 since tenant_id is just a column).
		if _, err := store.DB.Exec(`CREATE TABLE IF NOT EXISTS tenants (
			id TEXT PRIMARY KEY,
			created_at TEXT DEFAULT (datetime('now'))
		)`); err != nil {
			return err
		}

		_, err = store.DB.ExecContext(context.Background(),
			`INSERT INTO tenants (id) VALUES (?)`, tenantID)
		if err != nil {
			return fmt.Errorf("creating tenant %q: %w", tenantID, err)
		}
		// Opaque ULID just to stamp creation sequence.
		_ = ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader)

		fmt.Printf("Tenant created: %s\n", tenantID)
		fmt.Printf("  Scope agent/task/workflow operations to this tenant by setting tenant_id on those rows.\n")
		return nil
	},
}

var tenantListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tenants",
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

		ctx := context.Background()

		// Fall back to distinct tenant_id values if the tenants table hasn't
		// been created yet (fresh DB or legacy install).
		rows, err := store.DB.QueryContext(ctx,
			`SELECT DISTINCT tenant_id FROM agents
			 UNION
			 SELECT DISTINCT tenant_id FROM tasks
			 UNION
			 SELECT DISTINCT tenant_id FROM workflows`)
		if err != nil {
			return err
		}
		defer rows.Close()

		var tenants []string
		for rows.Next() {
			var t string
			if err := rows.Scan(&t); err == nil && t != "" {
				tenants = append(tenants, t)
			}
		}
		if len(tenants) == 0 {
			fmt.Println("No tenants. Use 'hive tenant create <id>' to create one.")
			return nil
		}
		for _, t := range tenants {
			fmt.Println(t)
		}
		return nil
	},
}

func init() {
	tenantCmd.AddCommand(tenantCreateCmd)
	tenantCmd.AddCommand(tenantListCmd)
	rootCmd.AddCommand(tenantCmd)
}
