package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JulienLeotier/hive/internal/config"
	"github.com/JulienLeotier/hive/internal/optimizer"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/spf13/cobra"
)

var optimizeCmd = &cobra.Command{
	Use:   "optimize",
	Short: "Run optimization analysis on historical execution data",
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOut, _ := cmd.Flags().GetBool("json")
		trend, _ := cmd.Flags().GetBool("trend")
		autoTune, _ := cmd.Flags().GetBool("auto-tune")
		window, _ := cmd.Flags().GetInt("window")

		cfg, err := config.Load("hive.yaml")
		if err != nil {
			return err
		}
		store, err := storage.Open(cfg.DataDir)
		if err != nil {
			return err
		}
		defer store.Close()

		an := optimizer.NewAnalyzer(store.DB)
		ctx := context.Background()

		if trend {
			cur, prev, err := an.Trend(ctx, window)
			if err != nil {
				return err
			}
			if jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"current": cur, "previous": prev})
			}
			fmt.Printf("Window %s\n", cur.Window)
			fmt.Printf("  Current:  %d tasks, %.2f%% failure, %.1fs avg\n", cur.TasksRun, cur.FailureRate*100, cur.AvgDurationS)
			fmt.Printf("  Previous: %d tasks, %.2f%% failure, %.1fs avg\n", prev.TasksRun, prev.FailureRate*100, prev.AvgDurationS)
			return nil
		}

		if autoTune {
			tunings, err := an.AutoTune(ctx)
			if err != nil {
				return err
			}
			if jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(tunings)
			}
			if len(tunings) == 0 {
				fmt.Println("No tuning suggestions — system is within healthy bounds.")
				return nil
			}
			for _, t := range tunings {
				fmt.Printf("• %s: %.2f → %.2f (%s)\n", t.Setting, t.OldValue, t.NewValue, t.Rationale)
			}
			return nil
		}

		recs, err := an.Analyze(ctx)
		if err != nil {
			return err
		}
		if jsonOut {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(recs)
		}
		if len(recs) == 0 {
			fmt.Println("No optimization opportunities found.")
			return nil
		}
		for _, r := range recs {
			fmt.Printf("[%s] %s — impact: %s (confidence %.0f%%)\n", r.Type, r.Description, r.Impact, r.Confidence*100)
		}
		return nil
	},
}

func init() {
	optimizeCmd.Flags().Bool("json", false, "output in JSON format")
	optimizeCmd.Flags().Bool("trend", false, "show trend snapshot (current vs previous window)")
	optimizeCmd.Flags().Bool("auto-tune", false, "suggest configuration tunings based on trends")
	optimizeCmd.Flags().Int("window", 7, "analysis window in days (for --trend / --auto-tune)")
	rootCmd.AddCommand(optimizeCmd)
}
