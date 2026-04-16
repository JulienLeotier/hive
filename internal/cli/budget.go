package cli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/JulienLeotier/hive/internal/config"
	"github.com/JulienLeotier/hive/internal/cost"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/spf13/cobra"
)

var budgetCmd = &cobra.Command{
	Use:   "budget",
	Short: "Manage per-agent daily spend budgets",
}

var budgetSetCmd = &cobra.Command{
	Use:   "set [agent-name] [daily-limit-usd]",
	Short: "Set a daily USD budget for an agent (also accepts --agent + --daily-limit flags)",
	Args:  cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Story 16.4 AC uses the flag form: --agent NAME --daily-limit N.
		// Accept both positional and flag forms.
		name, _ := cmd.Flags().GetString("agent")
		limit, _ := cmd.Flags().GetFloat64("daily-limit")
		if name == "" && len(args) >= 1 {
			name = args[0]
		}
		if limit == 0 && len(args) >= 2 {
			var err error
			limit, err = strconv.ParseFloat(args[1], 64)
			if err != nil {
				return fmt.Errorf("parsing daily limit: %w", err)
			}
		}
		if name == "" {
			return fmt.Errorf("agent name is required (--agent or positional)")
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

		tracker := cost.NewTracker(store.DB)
		if err := tracker.SetBudget(context.Background(), name, limit); err != nil {
			return err
		}
		fmt.Printf("Budget set: %s = $%.2f/day\n", name, limit)
		return nil
	},
}

var budgetListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured budgets with today's spend",
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

		tracker := cost.NewTracker(store.DB)
		alerts, err := tracker.EvaluateAlerts(context.Background())
		if err != nil {
			return err
		}
		if len(alerts) == 0 {
			fmt.Println("No budgets configured. Use 'hive budget set <agent> <limit>' to create one.")
			return nil
		}
		fmt.Printf("%-20s %-12s %-12s %-10s\n", "AGENT", "LIMIT", "SPEND", "STATUS")
		fmt.Printf("%-20s %-12s %-12s %-10s\n", "-----", "-----", "-----", "------")
		for _, a := range alerts {
			status := "ok"
			if a.Breached {
				status = "BREACHED"
			}
			fmt.Printf("%-20s $%-11.2f $%-11.4f %-10s\n", a.AgentName, a.DailyLimit, a.Spend, status)
		}
		return nil
	},
}

var budgetRemoveCmd = &cobra.Command{
	Use:   "remove [agent-name]",
	Short: "Remove a budget",
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

		tracker := cost.NewTracker(store.DB)
		if err := tracker.DeleteBudget(context.Background(), args[0]); err != nil {
			return err
		}
		fmt.Printf("Budget removed: %s\n", args[0])
		return nil
	},
}

func init() {
	budgetSetCmd.Flags().String("agent", "", "agent name (alternative to positional arg)")
	budgetSetCmd.Flags().Float64("daily-limit", 0, "daily USD limit (alternative to positional arg)")
	budgetCmd.AddCommand(budgetSetCmd)
	budgetCmd.AddCommand(budgetListCmd)
	budgetCmd.AddCommand(budgetRemoveCmd)
	rootCmd.AddCommand(budgetCmd)
}
