package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JulienLeotier/hive/internal/hivehub"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search HiveHub for workflow templates",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := ""
		if len(args) == 1 {
			query = args[0]
		}
		registry := hivehub.NewRegistry()
		if url, _ := cmd.Flags().GetString("registry"); url != "" {
			registry.IndexURL = url
		}

		results, err := registry.Search(query)
		if err != nil {
			return err
		}
		if len(results) == 0 {
			fmt.Println("No matching templates in HiveHub.")
			return nil
		}
		fmt.Printf("%-25s %-12s %-10s %s\n", "NAME", "VERSION", "CATEGORY", "DESCRIPTION")
		fmt.Printf("%-25s %-12s %-10s %s\n", "----", "-------", "--------", "-----------")
		for _, t := range results {
			desc := t.Description
			if len(desc) > 60 {
				desc = desc[:57] + "…"
			}
			fmt.Printf("%-25s %-12s %-10s %s\n", t.Name, t.Version, t.Category, desc)
		}
		return nil
	},
}

var installCmd = &cobra.Command{
	Use:   "install [template-name]",
	Short: "Install a HiveHub template into the current directory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		registry := hivehub.NewRegistry()
		if url, _ := cmd.Flags().GetString("registry"); url != "" {
			registry.IndexURL = url
		}

		dest, _ := cmd.Flags().GetString("dest")
		if dest == "" {
			dest = args[0]
		}

		force, _ := cmd.Flags().GetBool("force")
		tmpl, files, err := registry.InstallWith(args[0], dest, hivehub.InstallOptions{
			Force: force,
			Confirm: func(path string) bool {
				// Story 14.3 AC: don't overwrite without confirmation.
				fmt.Fprintf(os.Stderr, "File %s already exists. Overwrite? [y/N] ", path)
				reader := bufio.NewReader(os.Stdin)
				line, err := reader.ReadString('\n')
				if err != nil {
					return false
				}
				return strings.EqualFold(strings.TrimSpace(line), "y")
			},
		})
		if err != nil {
			return err
		}
		fmt.Printf("Installed %s@%s into %s/ (%d files)\n", tmpl.Name, tmpl.Version, dest, len(files))
		for _, f := range files {
			fmt.Printf("  %s\n", f)
		}
		return nil
	},
}

var publishCmd = &cobra.Command{
	Use:   "publish [path]",
	Short: "Package a workflow template into a HiveHub submission manifest",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) == 1 {
			dir = args[0]
		}
		abs, err := filepath.Abs(dir)
		if err != nil {
			return err
		}

		name, _ := cmd.Flags().GetString("name")
		description, _ := cmd.Flags().GetString("description")
		version, _ := cmd.Flags().GetString("version")
		author, _ := cmd.Flags().GetString("author")
		category, _ := cmd.Flags().GetString("category")
		outFile, _ := cmd.Flags().GetString("out")

		if name == "" {
			return fmt.Errorf("--name is required")
		}
		if version == "" {
			version = "0.1.0"
		}

		registry := hivehub.NewRegistry()
		data, err := registry.PublishDir(abs, hivehub.Template{
			Name:        name,
			Description: description,
			Version:     version,
			Author:      author,
			Category:    category,
		})
		if err != nil {
			return err
		}

		if outFile == "" {
			outFile = fmt.Sprintf("%s-%s.json", name, version)
		}
		if err := os.WriteFile(outFile, data, 0o644); err != nil {
			return err
		}
		fmt.Printf("Wrote publication manifest: %s\n", outFile)
		fmt.Printf("  Open a PR against the HiveHub index with this file.\n")
		return nil
	},
}

func init() {
	searchCmd.Flags().String("registry", "", "override HiveHub registry index URL")
	installCmd.Flags().String("registry", "", "override HiveHub registry index URL")
	installCmd.Flags().String("dest", "", "destination directory (defaults to template name)")
	installCmd.Flags().Bool("force", false, "overwrite existing files without prompting")

	publishCmd.Flags().String("name", "", "template name (required)")
	publishCmd.Flags().String("description", "", "short description")
	publishCmd.Flags().String("version", "0.1.0", "template version")
	publishCmd.Flags().String("author", "", "author handle")
	publishCmd.Flags().String("category", "", "category (e.g., review, pipeline, research)")
	publishCmd.Flags().String("out", "", "output manifest file path")

	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(publishCmd)
}
