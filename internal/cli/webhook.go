package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JulienLeotier/hive/internal/config"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/JulienLeotier/hive/internal/webhook"
	"github.com/spf13/cobra"
)

var webhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "Manage webhook notifications",
}

var webhookAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a webhook configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		url, _ := cmd.Flags().GetString("url")
		whType, _ := cmd.Flags().GetString("type")
		events, _ := cmd.Flags().GetString("events")

		if name == "" || url == "" {
			return fmt.Errorf("--name and --url are required")
		}

		cfg, _ := config.Load("hive.yaml")
		store, err := storage.Open(cfg.DataDir)
		if err != nil {
			return err
		}
		defer store.Close()

		d := webhook.NewDispatcher(store.DB)
		wh, err := d.Add(context.Background(), name, url, whType, events)
		if err != nil {
			return err
		}

		fmt.Printf("Webhook added: %s (%s)\n", wh.Name, wh.Type)
		fmt.Printf("  URL: %s\n", wh.URL)
		if events != "" {
			fmt.Printf("  Events: %s\n", events)
		}
		return nil
	},
}

var webhookListCmd = &cobra.Command{
	Use:   "list",
	Short: "List webhook configurations",
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOutput, _ := cmd.Flags().GetBool("json")

		cfg, _ := config.Load("hive.yaml")
		store, err := storage.Open(cfg.DataDir)
		if err != nil {
			return err
		}
		defer store.Close()

		d := webhook.NewDispatcher(store.DB)
		configs, err := d.List(context.Background())
		if err != nil {
			return err
		}

		if jsonOutput {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(configs)
		}

		if len(configs) == 0 {
			fmt.Println("No webhooks configured.")
			return nil
		}

		for _, c := range configs {
			status := "enabled"
			if !c.Enabled {
				status = "disabled"
			}
			fmt.Printf("%-20s %-10s %-8s %s\n", c.Name, c.Type, status, c.URL)
		}
		return nil
	},
}

func init() {
	webhookAddCmd.Flags().String("name", "", "webhook name (required)")
	webhookAddCmd.Flags().String("url", "", "webhook URL (required)")
	webhookAddCmd.Flags().String("type", "generic", "webhook type (slack, github, generic)")
	webhookAddCmd.Flags().String("events", "", "event filter (comma-separated or JSON array)")
	webhookListCmd.Flags().Bool("json", false, "output as JSON")

	webhookCmd.AddCommand(webhookAddCmd, webhookListCmd)
	rootCmd.AddCommand(webhookCmd)
}
