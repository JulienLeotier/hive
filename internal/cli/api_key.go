package cli

import (
	"context"
	"fmt"

	"github.com/JulienLeotier/hive/internal/api"
	"github.com/JulienLeotier/hive/internal/config"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/spf13/cobra"
)

var apiKeyCmd = &cobra.Command{
	Use:   "api-key",
	Short: "Manage API keys used by agents and clients",
}

var apiKeyGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a new API key",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return fmt.Errorf("--name is required")
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

		km := api.NewKeyManager(store.DB)
		raw, err := km.Generate(context.Background(), name)
		if err != nil {
			return err
		}
		fmt.Printf("API key generated for %q:\n\n  %s\n\n", name, raw)
		fmt.Println("Save this key now — it cannot be recovered.")
		return nil
	},
}

var apiKeyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List API keys (names + creation timestamps; never the raw key)",
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

		km := api.NewKeyManager(store.DB)
		keys, err := km.List(context.Background())
		if err != nil {
			return err
		}
		if len(keys) == 0 {
			fmt.Println("No API keys. Use 'hive api-key generate --name <name>'.")
			return nil
		}
		fmt.Printf("%-30s %s\n", "NAME", "CREATED")
		for _, k := range keys {
			fmt.Printf("%-30s %s\n", k.Name, k.CreatedAt.Format("2006-01-02 15:04:05"))
		}
		return nil
	},
}

var apiKeyDeleteCmd = &cobra.Command{
	Use:   "delete [name]",
	Short: "Revoke an API key by name",
	Args:  cobra.ExactArgs(1),
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

		km := api.NewKeyManager(store.DB)
		if err := km.Delete(context.Background(), args[0]); err != nil {
			return err
		}
		fmt.Printf("API key revoked: %s\n", args[0])
		return nil
	},
}

func init() {
	apiKeyGenerateCmd.Flags().String("name", "", "label for the key (required)")
	apiKeyCmd.AddCommand(apiKeyGenerateCmd)
	apiKeyCmd.AddCommand(apiKeyListCmd)
	apiKeyCmd.AddCommand(apiKeyDeleteCmd)
	rootCmd.AddCommand(apiKeyCmd)
}
