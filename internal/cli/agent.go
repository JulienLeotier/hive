package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/JulienLeotier/hive/internal/agent"
	"github.com/JulienLeotier/hive/internal/config"
	"github.com/JulienLeotier/hive/internal/cost"
	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/JulienLeotier/hive/internal/task"
	"github.com/JulienLeotier/hive/internal/trust"
	"github.com/spf13/cobra"
)

// detectAgentType inspects a local path for well-known files and infers an adapter type.
func detectAgentType(path string) string {
	markers := []struct {
		file string
		kind string
	}{
		{"AGENT.yaml", "claude-code"},
		{".claude/AGENT.yaml", "claude-code"},
		{"CLAUDE.md", "claude-code"},
		{"crewai.yaml", "crewai"},
		{"agents.yaml", "crewai"},
		{"autogen_agent.py", "autogen"},
		{"mcp.json", "mcp"},
		{".mcp.json", "mcp"},
		{"langchain_agent.py", "langchain"},
	}
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(path, m.file)); err == nil {
			return m.kind
		}
	}
	return "http"
}

var addAgentCmd = &cobra.Command{
	Use:   "add-agent",
	Short: "Register an agent with the hive",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		agentType, _ := cmd.Flags().GetString("type")
		url, _ := cmd.Flags().GetString("url")
		path, _ := cmd.Flags().GetString("path")

		if name == "" {
			return fmt.Errorf("--name is required")
		}

		// --path: local agent — auto-detect type from project markers, synthesize URL.
		if path != "" {
			abs, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("resolving path: %w", err)
			}
			if _, err := os.Stat(abs); err != nil {
				return fmt.Errorf("agent path: %w", err)
			}
			if agentType == "" || agentType == "http" {
				agentType = detectAgentType(abs)
			}
			if url == "" {
				url = "file://" + abs
			}
		}

		if url == "" {
			return fmt.Errorf("--url or --path is required")
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

		var a *agent.Agent
		if path != "" {
			a, err = mgr.RegisterLocal(context.Background(), name, agentType, path, nil)
		} else {
			a, err = mgr.Register(context.Background(), name, agentType, url)
		}
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
		ctx := context.Background()
		agents, err := mgr.List(ctx)
		if err != nil {
			return err
		}

		showCosts, _ := cmd.Flags().GetBool("costs")

		if jsonOutput {
			out := map[string]any{"agents": agents}
			if showCosts {
				tracker := cost.NewTracker(store.DB)
				summaries, _ := tracker.ByAgent(ctx)
				alerts, _ := tracker.EvaluateAlerts(ctx)
				out["costs"] = summaries
				out["budget_alerts"] = alerts
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
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

		if showCosts {
			tracker := cost.NewTracker(store.DB)
			summaries, err := tracker.ByAgent(ctx)
			if err != nil {
				return err
			}
			fmt.Println()
			fmt.Printf("%-20s %-12s %-10s\n", "AGENT", "TOTAL COST", "TASKS")
			fmt.Printf("%-20s %-12s %-10s\n", "-----", "----------", "-----")
			var total float64
			for _, s := range summaries {
				fmt.Printf("%-20s $%-11.4f %-10d\n", s.AgentName, s.TotalCost, s.TaskCount)
				total += s.TotalCost
			}
			fmt.Printf("Total spend: $%.4f\n", total)

			alerts, err := tracker.EvaluateAlerts(ctx)
			if err == nil {
				for _, a := range alerts {
					if a.Breached {
						fmt.Printf("  ⚠  %s over budget: $%.4f / $%.4f daily\n", a.AgentName, a.Spend, a.DailyLimit)
					}
				}
			}
		}
		return nil
	},
}

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage agents",
}

var agentTrustCmd = &cobra.Command{
	Use:   "trust",
	Short: "Inspect or set an agent's trust level",
}

var agentTrustGetCmd = &cobra.Command{
	Use:   "get [agent-name]",
	Short: "Show current trust level and stats for an agent",
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

		ctx := context.Background()
		mgr := agent.NewManager(store.DB)
		a, err := mgr.GetByName(ctx, args[0])
		if err != nil {
			return err
		}

		engine := trust.NewEngine(store.DB, trust.DefaultThresholds())
		stats, err := engine.GetStats(ctx, a.ID)
		if err != nil {
			return err
		}

		fmt.Printf("Agent:       %s\n", a.Name)
		fmt.Printf("Trust level: %s\n", a.TrustLevel)
		fmt.Printf("Total tasks: %d (success=%d, failed=%d)\n", stats.TotalTasks, stats.Successes, stats.Failures)
		fmt.Printf("Error rate:  %.2f%%\n", stats.ErrorRate*100)
		return nil
	},
}

var agentTrustSetCmd = &cobra.Command{
	Use:   "set [agent-name] [level]",
	Short: "Manually set an agent's trust level (supervised|guided|autonomous|trusted)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, level := args[0], args[1]
		switch level {
		case trust.LevelSupervised, trust.LevelGuided, trust.LevelAutonomous, trust.LevelTrusted:
		default:
			return fmt.Errorf("unknown trust level %q — use supervised|guided|autonomous|trusted", level)
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

		ctx := context.Background()
		mgr := agent.NewManager(store.DB)
		a, err := mgr.GetByName(ctx, name)
		if err != nil {
			return err
		}

		engine := trust.NewEngine(store.DB, trust.DefaultThresholds())
		if err := engine.SetManual(ctx, a.ID, level); err != nil {
			return err
		}

		fmt.Printf("Trust level for %s set to %s\n", name, level)
		return nil
	},
}

var agentTrustOverrideCmd = &cobra.Command{
	Use:   "override [agent] [task-type] [level]",
	Short: "Set a per-task-type trust override",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		reason, _ := cmd.Flags().GetString("reason")
		if !trust.IsValidLevel(args[2]) {
			return fmt.Errorf("invalid trust level %q", args[2])
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

		ctx := context.Background()
		mgr := agent.NewManager(store.DB)
		a, err := mgr.GetByName(ctx, args[0])
		if err != nil {
			return err
		}

		engine := trust.NewEngine(store.DB, trust.DefaultThresholds())
		if err := engine.SetOverride(ctx, a.ID, args[1], args[2], reason); err != nil {
			return err
		}
		fmt.Printf("Override set: %s[%s] = %s\n", args[0], args[1], args[2])
		return nil
	},
}

var agentTrustClearOverrideCmd = &cobra.Command{
	Use:   "clear-override [agent] [task-type]",
	Short: "Remove a per-task-type trust override",
	Args:  cobra.ExactArgs(2),
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
		mgr := agent.NewManager(store.DB)
		a, err := mgr.GetByName(ctx, args[0])
		if err != nil {
			return err
		}

		engine := trust.NewEngine(store.DB, trust.DefaultThresholds())
		if err := engine.RemoveOverride(ctx, a.ID, args[1]); err != nil {
			return err
		}
		fmt.Printf("Override cleared: %s[%s]\n", args[0], args[1])
		return nil
	},
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
	addAgentCmd.Flags().String("type", "", "agent type (http, claude-code, mcp, crewai, autogen, langchain); auto-detected when --path is given")
	addAgentCmd.Flags().String("url", "", "agent URL (required unless --path is given)")
	addAgentCmd.Flags().String("path", "", "local filesystem path to an agent project (auto-detects type)")

	agentTrustOverrideCmd.Flags().String("reason", "", "why this override is being set")

	statusCmd.Flags().Bool("json", false, "output in JSON format")
	statusCmd.Flags().Bool("costs", false, "include cost rollup and budget alerts")

	agentCmd.AddCommand(agentSwapCmd)
	agentCmd.AddCommand(agentTrustCmd)
	agentTrustCmd.AddCommand(agentTrustGetCmd)
	agentTrustCmd.AddCommand(agentTrustSetCmd)
	agentTrustCmd.AddCommand(agentTrustOverrideCmd)
	agentTrustCmd.AddCommand(agentTrustClearOverrideCmd)

	rootCmd.AddCommand(addAgentCmd)
	rootCmd.AddCommand(removeAgentCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(agentCmd)
}
