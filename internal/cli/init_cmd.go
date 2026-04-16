package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [project-name]",
	Short: "Scaffold a new hive project",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		template, _ := cmd.Flags().GetString("template")

		projectName := "my-hive"
		if len(args) > 0 {
			projectName = args[0]
		}

		if err := os.MkdirAll(projectName, 0755); err != nil {
			return fmt.Errorf("creating project directory: %w", err)
		}

		// Create hive.yaml
		workflowYAML := getTemplate(template, projectName)
		if err := os.WriteFile(filepath.Join(projectName, "hive.yaml"), []byte(workflowYAML), 0644); err != nil {
			return err
		}

		// Create agents directory
		agentsDir := filepath.Join(projectName, "agents")
		os.MkdirAll(agentsDir, 0755)
		os.WriteFile(filepath.Join(agentsDir, ".gitkeep"), []byte(""), 0644)

		// Create README
		readme := fmt.Sprintf("# %s\n\nA Hive project — AI agent orchestration.\n\n## Quick Start\n\n```bash\nhive add-agent --name my-agent --type http --url http://localhost:8080\nhive run\nhive status\n```\n", projectName)
		os.WriteFile(filepath.Join(projectName, "README.md"), []byte(readme), 0644)

		fmt.Printf("Project '%s' created!\n", projectName)
		fmt.Printf("  hive.yaml   — workflow configuration\n")
		fmt.Printf("  agents/     — agent configurations\n")
		fmt.Printf("  README.md   — getting started\n")
		if template != "" {
			fmt.Printf("  template:   %s\n", template)
		}
		fmt.Printf("\nNext: cd %s && hive add-agent --name my-agent --type http --url http://localhost:8080\n", projectName)
		return nil
	},
}

func getTemplate(template, projectName string) string {
	switch template {
	case "code-review":
		return fmt.Sprintf(`name: %s
tasks:
  - name: review
    type: code-review
    input:
      source: pr
  - name: summarize
    type: summarize
    depends_on: [review]
    input:
      format: markdown
`, projectName)
	case "content-pipeline":
		return fmt.Sprintf(`name: %s
tasks:
  - name: write
    type: content-write
    input:
      topic: "{{topic}}"
  - name: edit
    type: content-edit
    depends_on: [write]
  - name: optimize
    type: seo-optimize
    depends_on: [edit]
  - name: publish
    type: publish
    depends_on: [optimize]
`, projectName)
	case "research":
		return fmt.Sprintf(`name: %s
tasks:
  - name: search-a
    type: research
    input:
      query: "{{query}}"
      source: academic
  - name: search-b
    type: research
    input:
      query: "{{query}}"
      source: web
  - name: aggregate
    type: summarize
    depends_on: [search-a, search-b]
  - name: report
    type: report-generate
    depends_on: [aggregate]
`, projectName)
	default:
		return fmt.Sprintf(`name: %s
tasks:
  - name: example-task
    type: example
    input:
      message: "Hello from Hive!"
`, projectName)
	}
}

func init() {
	initCmd.Flags().String("template", "", "project template (code-review, content-pipeline, research)")
	rootCmd.AddCommand(initCmd)
}
