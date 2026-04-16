package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JulienLeotier/hive/internal/config"
	"github.com/JulienLeotier/hive/internal/knowledge"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/spf13/cobra"
)

var knowledgeCmd = &cobra.Command{
	Use:   "knowledge",
	Short: "Manage the shared knowledge layer",
}

var knowledgeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List knowledge entries",
	RunE: func(cmd *cobra.Command, args []string) error {
		taskType, _ := cmd.Flags().GetString("type")
		jsonOutput, _ := cmd.Flags().GetBool("json")

		cfg, _ := config.Load("hive.yaml")
		store, err := storage.Open(cfg.DataDir)
		if err != nil {
			return err
		}
		defer store.Close()

		ks := knowledge.NewStore(store.DB)

		entries, err := ks.ListByType(context.Background(), taskType)
		if err != nil {
			return err
		}

		if jsonOutput {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(entries)
		}

		if len(entries) == 0 {
			fmt.Println("No knowledge entries found.")
			return nil
		}

		for _, e := range entries {
			fmt.Printf("[%s] %s — %s (%s)\n", e.Outcome, e.TaskType, e.Approach, e.CreatedAt.Format("2006-01-02"))
		}
		return nil
	},
}

var knowledgeSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search knowledge entries by keyword similarity",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		jsonOutput, _ := cmd.Flags().GetBool("json")

		cfg, _ := config.Load("hive.yaml")
		store, err := storage.Open(cfg.DataDir)
		if err != nil {
			return err
		}
		defer store.Close()

		// Story 10.4 AC: search returns semantically similar entries — use
		// vector search when an embedder is available, fall back to keyword.
		ks := knowledge.NewStore(store.DB).WithEmbedder(knowledge.NewHashingEmbedder(128))
		results, err := ks.VectorSearch(context.Background(), args[0], limit)
		if err != nil {
			// Fallback to keyword search if embedder unavailable.
			results, err = ks.Search(context.Background(), args[0], limit)
			if err != nil {
				return err
			}
		}

		if jsonOutput {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(results)
		}

		if len(results) == 0 {
			fmt.Println("No matching knowledge entries found.")
			return nil
		}

		for _, e := range results {
			fmt.Printf("[%s] %s — %s\n", e.Outcome, e.TaskType, e.Approach)
		}
		return nil
	},
}

func init() {
	knowledgeListCmd.Flags().String("type", "", "filter by task type")
	knowledgeListCmd.Flags().Bool("json", false, "output as JSON")
	knowledgeSearchCmd.Flags().Int("limit", 5, "max results")
	knowledgeSearchCmd.Flags().Bool("json", false, "output as JSON")

	knowledgeCmd.AddCommand(knowledgeListCmd, knowledgeSearchCmd)
	rootCmd.AddCommand(knowledgeCmd)
}
