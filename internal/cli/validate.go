package cli

import (
	"fmt"

	"github.com/JulienLeotier/hive/internal/workflow"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate [workflow-file]",
	Short: "Validate workflow configuration",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "hive.yaml"
		if len(args) > 0 {
			path = args[0]
		}

		cfg, err := workflow.ParseFile(path)
		if err != nil {
			fmt.Printf("FAIL: %s\n", err)
			return err
		}

		levels, err := workflow.TopologicalSort(cfg.Tasks)
		if err != nil {
			fmt.Printf("FAIL: %s\n", err)
			return err
		}

		fmt.Printf("OK: workflow '%s' is valid\n", cfg.Name)
		fmt.Printf("  Tasks:    %d\n", len(cfg.Tasks))
		fmt.Printf("  Levels:   %d (parallel groups)\n", len(levels))
		if cfg.Trigger != nil {
			fmt.Printf("  Trigger:  %s\n", cfg.Trigger.Type)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
