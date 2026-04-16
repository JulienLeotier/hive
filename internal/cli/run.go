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
	"github.com/JulienLeotier/hive/internal/workflow"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run [workflow-file]",
	Short: "Execute a workflow end-to-end",
	RunE: func(cmd *cobra.Command, args []string) error {
		quiet, _ := cmd.Flags().GetBool("quiet")
		jsonOutput, _ := cmd.Flags().GetBool("json")

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

		// Load registered agents and create adapters
		agentMgr := agent.NewManager(store.DB)
		agents, err := agentMgr.List(context.Background())
		if err != nil {
			return fmt.Errorf("loading agents: %w", err)
		}

		for _, a := range agents {
			var agentConfig map[string]string
			json.Unmarshal([]byte(a.Config), &agentConfig)
			baseURL := agentConfig["base_url"]
			if baseURL != "" {
				engine.RegisterAdapter(a.ID, baseURL, nil) // nil adapter = will create HTTPAdapter on-the-fly
			}
		}

		if !quiet {
			fmt.Printf("Agents: %d registered\n", len(agents))
			fmt.Println("---")
		}

		// Execute workflow
		result, err := engine.Run(context.Background(), wfConfig)

		if jsonOutput {
			json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			return err
		}

		if err != nil {
			fmt.Printf("FAILED: %s\n", err)
			return err
		}

		if !quiet {
			fmt.Println("---")
			fmt.Printf("Workflow completed: %s\n", result.WorkflowID)
			fmt.Printf("Tasks completed: %d\n", len(result.TaskResults))
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
	rootCmd.AddCommand(runCmd)
}
