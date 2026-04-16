package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JulienLeotier/hive/internal/config"
	"github.com/JulienLeotier/hive/internal/event"
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

		if apply, _ := cmd.Flags().GetBool("apply"); apply {
			// Story 20.3: snapshot baseline before applying so CompareToBaseline
			// can measure the effect on the next run.
			baseline, err := an.SnapshotBaseline(ctx, window, "pre-apply")
			if err != nil {
				return err
			}
			tunings, err := an.AutoTune(ctx)
			if err != nil {
				return err
			}
			bus := event.NewBus(store.DB)
			for _, t := range tunings {
				_, _ = bus.Publish(ctx, "system.optimization.applied", "optimize_cli", map[string]any{
					"setting":   t.Setting,
					"old_value": t.OldValue,
					"new_value": t.NewValue,
					"rationale": t.Rationale,
					"baseline":  baseline,
				})
				fmt.Printf("Applied: %s = %.2f (%s)\n", t.Setting, t.NewValue, t.Rationale)
			}
			if len(tunings) == 0 {
				fmt.Println("Nothing to apply — no tunings suggested.")
			}
			fmt.Printf("Baseline captured at %s; run `hive optimize --compare-baseline` later to measure the effect.\n",
				baseline.TakenAt.Format(time.RFC3339))
			return nil
		}

		if compare, _ := cmd.Flags().GetBool("compare-baseline"); compare {
			// Find the most recent baseline in the event log and compare.
			bus := event.NewBus(store.DB)
			events, err := bus.Query(ctx, event.QueryOpts{Type: "system.optimization.applied", Limit: 1})
			if err != nil || len(events) == 0 {
				fmt.Println("No baseline recorded yet — run `hive optimize --apply` first.")
				return nil
			}
			var payload struct {
				Baseline optimizer.Baseline `json:"baseline"`
			}
			if err := json.Unmarshal([]byte(events[len(events)-1].Payload), &payload); err != nil {
				return fmt.Errorf("parsing baseline payload: %w", err)
			}
			delta, err := an.CompareToBaseline(ctx, payload.Baseline, window)
			if err != nil {
				return err
			}
			if jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(delta)
			}
			improved := "regressed"
			if delta.Improved {
				improved = "improved"
			}
			fmt.Printf("Since baseline (%s): %s\n", delta.Baseline.TakenAt.Format(time.RFC3339), improved)
			fmt.Printf("  Tasks run:    %+d\n", delta.TasksRunDelta)
			fmt.Printf("  Failure rate: %+.2f%%\n", delta.FailureDelta*100)
			fmt.Printf("  Avg duration: %+.2fs\n", delta.DurationDelta)
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
	optimizeCmd.Flags().Bool("apply", false, "emit system.optimization.applied events recording approval of the suggested tunings")
	optimizeCmd.Flags().Bool("compare-baseline", false, "compare the current window against the most recent applied baseline")
	optimizeCmd.Flags().Int("window", 7, "analysis window in days (for --trend / --auto-tune)")
	rootCmd.AddCommand(optimizeCmd)
}
