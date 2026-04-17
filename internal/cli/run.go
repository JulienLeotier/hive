package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JulienLeotier/hive/internal/adapter"
	"github.com/JulienLeotier/hive/internal/agent"
	"github.com/JulienLeotier/hive/internal/config"
	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/JulienLeotier/hive/internal/task"
	"github.com/JulienLeotier/hive/internal/workflow"
	"github.com/spf13/cobra"
)

func firstNonZeroInt(v, fallback int) int {
	if v != 0 {
		return v
	}
	return fallback
}

func firstNonZeroFloat(v, fallback float64) float64 {
	if v != 0 {
		return v
	}
	return fallback
}

var runCmd = &cobra.Command{
	Use:   "run [workflow-file]",
	Short: "Execute a workflow end-to-end",
	RunE: func(cmd *cobra.Command, args []string) error {
		quiet, _ := cmd.Flags().GetBool("quiet")
		jsonOutput, _ := cmd.Flags().GetBool("json")
		trigger, _ := cmd.Flags().GetString("trigger")
		_ = trigger // reserved for future trigger-aware engines; currently emitted as an event

		wfPath := "hive.yaml"
		if len(args) > 0 {
			wfPath = args[0]
		}

		// Parse workflow
		wfConfig, err := workflow.ParseFile(wfPath)
		if err != nil {
			return fmt.Errorf("parsing workflow: %w", err)
		}

		if !quiet {
			fmt.Printf("Workflow: %s (%d tasks)\n", wfConfig.Name, len(wfConfig.Tasks))
		}

		// Load config and open storage
		cfg, err := config.Load("hive.yaml")
		if err != nil {
			return err
		}

		store, err := storage.Open(cfg.DataDir)
		if err != nil {
			return err
		}
		defer store.Close()

		// Initialize components
		bus := event.NewBus(store.DB)
		taskStore := task.NewStore(store.DB, bus)
		taskRouter := task.NewRouter(store.DB)
		wfStore := workflow.NewStore(store.DB, bus)

		engine := workflow.NewEngine(wfStore, taskStore, taskRouter, bus)

		// Story 5.5 AC: retry policy configurable from hive.yaml.
		if cfg.Retry != nil {
			policy := &adapter.RetryPolicy{
				MaxAttempts: firstNonZeroInt(cfg.Retry.MaxAttempts, 3),
				InitialWait: time.Duration(firstNonZeroInt(cfg.Retry.InitialWaitMs, 200)) * time.Millisecond,
				MaxWait:     time.Duration(firstNonZeroInt(cfg.Retry.MaxWaitMs, 2000)) * time.Millisecond,
				Multiplier:  firstNonZeroFloat(cfg.Retry.Multiplier, 2.0),
				Jitter:      firstNonZeroFloat(cfg.Retry.Jitter, 0.2),
			}
			engine.WithRetry(policy)
		}

		// Load registered agents and create adapters
		agentMgr := agent.NewManager(store.DB)
		agents, err := agentMgr.List(context.Background())
		if err != nil {
			return fmt.Errorf("loading agents: %w", err)
		}

		for _, a := range agents {
			var agentConfig map[string]string
			_ = json.Unmarshal([]byte(a.Config), &agentConfig)
			baseURL := agentConfig["base_url"]
			if baseURL != "" {
				engine.RegisterAdapter(a.ID, baseURL, nil) // nil adapter = will create HTTPAdapter on-the-fly
			}
		}

		// Story 3.3: streaming progress — JSON mode writes one object per event,
		// text mode pretty-prints each transition in the terminal.
		if jsonOutput {
			enc := json.NewEncoder(cmd.OutOrStdout())
			bus.Subscribe("task", func(e event.Event) {
				_ = enc.Encode(map[string]any{"kind": "event", "type": e.Type, "source": e.Source, "payload": e.Payload})
			})
			bus.Subscribe("workflow", func(e event.Event) {
				_ = enc.Encode(map[string]any{"kind": "event", "type": e.Type, "source": e.Source, "payload": e.Payload})
			})
		} else if !quiet {
			fmt.Printf("Agents: %d registered\n", len(agents))
			fmt.Println("---")
			bus.Subscribe("task", func(e event.Event) {
				fmt.Printf("[%s] %-18s %s\n", e.CreatedAt.Format("15:04:05"), e.Type, e.Payload)
			})
			bus.Subscribe("workflow", func(e event.Event) {
				fmt.Printf("[%s] %-18s %s\n", e.CreatedAt.Format("15:04:05"), e.Type, e.Payload)
			})
		}

		// Execute workflow. When --trigger is set, publish a manual trigger
		// event before the run so timelines show why the workflow fired.
		if trigger != "" {
			_, _ = bus.Publish(context.Background(), "workflow.trigger", "cli",
				map[string]string{"workflow": wfConfig.Name, "trigger": trigger})
		}

		started := time.Now()
		result, err := engine.Run(context.Background(), wfConfig)
		elapsed := time.Since(started)

		if jsonOutput {
			_ = json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
				"kind":        "summary",
				"result":      result,
				"duration_ms": elapsed.Milliseconds(),
			})
			return err
		}

		if err != nil {
			fmt.Printf("FAILED after %s: %s\n", elapsed, err)
			return err
		}

		// Story 7.3 AC: --quiet suppresses progress output but still shows the
		// final result summary. Non-quiet prints the verbose block.
		if quiet {
			fmt.Printf("workflow=%s tasks=%d duration=%s status=%s\n",
				result.WorkflowID, len(result.TaskResults), elapsed, result.Status)
		} else {
			fmt.Println("---")
			fmt.Printf("Workflow completed: %s\n", result.WorkflowID)
			fmt.Printf("Tasks completed:    %d\n", len(result.TaskResults))
			fmt.Printf("Duration:           %s\n", elapsed)
			for name, t := range result.TaskResults {
				fmt.Printf("  %s: %s (agent: %s)\n", name, t.Status, t.AgentID)
			}
		}

		return nil
	},
}

func init() {
	runCmd.Flags().Bool("quiet", false, "suppress progress output, show only final result")
	runCmd.Flags().Bool("json", false, "output results as JSON")
	// Story 3.4 AC: `hive run --trigger manual` instantiates the workflow as if a manual trigger fired.
	runCmd.Flags().String("trigger", "", "trigger source tag recorded on events (e.g., manual, ci)")
	rootCmd.AddCommand(runCmd)
}
