package cli

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// availableTemplates is kept in sync with the embedded tree.
var availableTemplates = []string{"code-review", "content-pipeline", "research"}

// promptTemplate offers an interactive selection when stdin is a TTY.
// Falls back to empty (minimal starter) on any error so `hive init` stays
// scriptable in CI.
func promptTemplate(in *os.File, out *os.File) string {
	info, err := in.Stat()
	if err != nil || (info.Mode()&os.ModeCharDevice) == 0 {
		return "" // not a TTY — keep non-interactive
	}

	fmt.Fprintln(out, "Choose a starter template:")
	fmt.Fprintln(out, "  0) (none — minimal starter)")
	for i, name := range availableTemplates {
		fmt.Fprintf(out, "  %d) %s\n", i+1, name)
	}
	fmt.Fprint(out, "Selection [0]: ")

	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil {
		return ""
	}
	choice := strings.TrimSpace(line)
	if choice == "" || choice == "0" {
		return ""
	}
	for i, name := range availableTemplates {
		if choice == fmt.Sprint(i+1) || choice == name {
			return name
		}
	}
	return ""
}

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

		// Story 7.1 AC: interactive selection when no --template is given.
		if template == "" {
			template = promptTemplate(os.Stdin, os.Stdout)
		}

		if err := os.MkdirAll(projectName, 0o755); err != nil {
			return fmt.Errorf("creating project directory: %w", err)
		}

		// Story 7.5/7.6/7.7: templates copy the full tree (workflow + agent configs + README).
		if template != "" {
			if err := copyTemplate(template, projectName); err != nil {
				return err
			}
			fmt.Printf("Project '%s' created from template '%s'\n", projectName, template)
			fmt.Printf("  hive.yaml      — workflow configuration\n")
			fmt.Printf("  agents/*.yaml  — agent personas (one per task)\n")
			fmt.Printf("  README.md      — template-specific setup instructions\n")
			return nil
		}

		// No template: emit a minimal starter.
		if err := os.WriteFile(filepath.Join(projectName, "hive.yaml"),
			[]byte(fmt.Sprintf("name: %s\ntasks:\n  - name: example-task\n    type: example\n    input:\n      message: \"Hello from Hive!\"\n", projectName)),
			0o644); err != nil {
			return err
		}
		agentsDir := filepath.Join(projectName, "agents")
		if err := os.MkdirAll(agentsDir, 0o755); err != nil {
			return fmt.Errorf("creating agents directory: %w", err)
		}
		_ = os.WriteFile(filepath.Join(agentsDir, ".gitkeep"), []byte(""), 0o644)
		_ = os.WriteFile(filepath.Join(projectName, "README.md"),
			[]byte(fmt.Sprintf("# %s\n\nA Hive project — AI agent orchestration.\n\n## Quick Start\n\n```bash\nhive add-agent --name my-agent --type http --url http://localhost:8080\nhive run\nhive status\n```\n", projectName)),
			0o644)

		fmt.Printf("Project '%s' created!\n", projectName)
		fmt.Printf("Next: cd %s && hive add-agent --name my-agent --type http --url http://localhost:8080\n", projectName)
		return nil
	},
}

// copyTemplate walks the embedded template tree and materialises it on disk.
func copyTemplate(template, dest string) error {
	root := "templates/" + template
	if _, err := fs.Stat(templatesFS, root); err != nil {
		return fmt.Errorf("unknown template %q (available: code-review, content-pipeline, research)", template)
	}
	return fs.WalkDir(templatesFS, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(path, root)
		rel = strings.TrimPrefix(rel, "/")
		outPath := dest
		if rel != "" {
			outPath = filepath.Join(dest, rel)
		}
		if d.IsDir() {
			return os.MkdirAll(outPath, 0o755)
		}
		data, err := fs.ReadFile(templatesFS, path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(outPath, data, 0o644)
	})
}

func init() {
	initCmd.Flags().String("template", "", "project template (code-review, content-pipeline, research)")
	rootCmd.AddCommand(initCmd)
}
