package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/JulienLeotier/hive/internal/workflow"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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
		jsonOut, _ := cmd.Flags().GetBool("json")

		data, err := os.ReadFile(path)
		if err != nil {
			return emit(cmd, jsonOut, []string{err.Error()}, "", 0, 0, nil)
		}

		// Parse without bailing on semantic errors so we can collect them.
		var cfg workflow.Config
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return emit(cmd, jsonOut, []string{err.Error()}, "", 0, 0, nil)
		}

		// Story 3.6 AC: report all issues, not just the first.
		problems := workflow.ValidateAll(&cfg)
		var issues []string
		for _, p := range problems {
			issues = append(issues, p.Error())
		}

		levelCount := 0
		var criticalPath []string
		if len(issues) == 0 {
			if levels, err := workflow.TopologicalSort(cfg.Tasks); err == nil {
				levelCount = len(levels)
			}
			criticalPath = workflow.CriticalPath(cfg.Tasks)
		}

		return emit(cmd, jsonOut, issues, cfg.Name, len(cfg.Tasks), levelCount, criticalPath)
	},
}

func emit(cmd *cobra.Command, jsonOut bool, issues []string, name string, tasks, levels int, critical []string) error {
	if jsonOut {
		payload := map[string]any{
			"ok":            len(issues) == 0,
			"name":          name,
			"tasks":         tasks,
			"levels":        levels,
			"issues":        issues,
			"critical_path": critical,
		}
		_ = json.NewEncoder(cmd.OutOrStdout()).Encode(payload)
		if len(issues) > 0 {
			return fmt.Errorf("%d issue(s)", len(issues))
		}
		return nil
	}
	if len(issues) > 0 {
		for _, i := range issues {
			fmt.Printf("FAIL: %s\n", i)
		}
		return fmt.Errorf("%d issue(s)", len(issues))
	}
	fmt.Printf("OK: workflow '%s' is valid\n", name)
	fmt.Printf("  Tasks:         %d\n", tasks)
	fmt.Printf("  Levels:        %d (parallel groups)\n", levels)
	if len(critical) > 0 {
		fmt.Printf("  Critical path: %v (%d hops)\n", critical, len(critical))
	}
	return nil
}

func init() {
	validateCmd.Flags().Bool("json", false, "output in JSON format for CI pipelines")
	rootCmd.AddCommand(validateCmd)
}
