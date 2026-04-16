package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JulienLeotier/hive/internal/agent"
	"github.com/JulienLeotier/hive/internal/config"
	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/JulienLeotier/hive/internal/task"
	"github.com/spf13/cobra"
)

var addAgentCmd = &cobra.Command{
	Use:   "add-agent",
	Short: "Register an agent with the hive",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		agentType, _ := cmd.Flags().GetString("type")
		url, _ := cmd.Flags().GetString("url")

		if name == "" || url == "" {
			return fmt.Errorf("--name and --url are required")
		}
		if agentType == "" {
			agentType = "http"
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

		mgr := agent.NewManager(store.DB)
		a, err := mgr.Register(context.Background(), name, agentType, url)
		if err != nil {
			return fmt.Errorf("registration failed: %w", err)
		}

		fmt.Printf("Agent registered: %s (%s)\n", a.Name, a.Type)
		fmt.Printf("  ID:     %s\n", a.ID)
		fmt.Printf("  Health: %s\n", a.HealthStatus)
		fmt.Printf("  Caps:   %s\n", a.Capabilities)
		return nil
	},
}

var removeAgentCmd = &cobra.Command{
	Use:   "remove-agent [name]",
	Short: "Remove an agent from the hive",
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

		mgr := agent.NewManager(store.DB)
		if err := mgr.Remove(context.Background(), args[0]); err != nil {
			return err
		}

		fmt.Printf("Agent removed: %s\n", args[0])
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show hive status — agents, health, and active tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOutput, _ := cmd.Flags().GetBool("json")

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
		agents, err := mgr.List(context.Background())
		if err != nil {
			return err
		}

		if jsonOutput {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(agents)
		}

		if len(agents) == 0 {
			fmt.Println("No agents registered. Use 'hive add-agent' to register one.")
			return nil
		}

		fmt.Printf("%-20s %-10s %-12s %-10s\n", "NAME", "TYPE", "HEALTH", "TRUST")
		fmt.Printf("%-20s %-10s %-12s %-10s\n", "----", "----", "------", "-----")
		for _, a := range agents {
			fmt.Printf("%-20s %-10s %-12s %-10s\n", a.Name, a.Type, a.HealthStatus, a.TrustLevel)
		}
		fmt.Printf("\nTotal: %d agents\n", len(agents))
		return nil
	},
}

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage agents",
}

var agentSwapCmd = &cobra.Command{
	Use:   "swap [old-name] [new-name]",
	Short: "Swap a failing agent for a replacement — reassigns in-flight tasks",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		oldName, newName := args[0], args[1]

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
		ctx := context.Background()

		if _, err := mgr.GetByName(ctx, oldName); err != nil {
			return fmt.Errorf("old agent: %w", err)
		}
		replacement, err := mgr.GetByName(ctx, newName)
		if err != nil {
			return fmt.Errorf("replacement agent: %w", err)
		}
		if replacement.HealthStatus != "healthy" {
			return fmt.Errorf("replacement agent %s is not healthy (%s)", newName, replacement.HealthStatus)
		}

		bus := event.NewBus(store.DB)
		router := task.NewRouter(store.DB).WithBus(bus)

		n, err := router.ReassignAgentTasks(ctx, oldName, "agent swap → "+newName)
		if err != nil {
			return fmt.Errorf("reassigning tasks: %w", err)
		}

		if err := mgr.UpdateHealth(ctx, oldName, "unavailable"); err != nil {
			return fmt.Errorf("marking old agent unavailable: %w", err)
		}

		_, _ = bus.Publish(ctx, "agent.swapped", "cli", map[string]string{
			"from": oldName, "to": newName,
		})

		fmt.Printf("Swapped %s → %s (%d tasks reassigned)\n", oldName, newName, n)
		return nil
	},
}

func init() {
	addAgentCmd.Flags().String("name", "", "agent name (required)")
	addAgentCmd.Flags().String("type", "http", "agent type (http, claude-code, mcp)")
	addAgentCmd.Flags().String("url", "", "agent URL (required)")

	statusCmd.Flags().Bool("json", false, "output in JSON format")

	agentCmd.AddCommand(agentSwapCmd)

	rootCmd.AddCommand(addAgentCmd)
	rootCmd.AddCommand(removeAgentCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(agentCmd)
}
