package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/JulienLeotier/hive/internal/config"
	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Query event logs with filtering",
	RunE: func(cmd *cobra.Command, args []string) error {
		eventType, _ := cmd.Flags().GetString("type")
		agentName, _ := cmd.Flags().GetString("agent")
		since, _ := cmd.Flags().GetString("since")
		limit, _ := cmd.Flags().GetInt("limit")
		jsonOutput, _ := cmd.Flags().GetBool("json")

		cfg, err := config.Load("hive.yaml")
		if err != nil {
			return err
		}

		store, err := storage.Open(cfg.DataDir)
		if err != nil {
			return err
		}
		defer store.Close()

		bus := event.NewBus(store.DB)

		opts := event.QueryOpts{
			Type:   eventType,
			Source: agentName,
			Limit:  limit,
		}

		if since != "" {
			d, err := time.ParseDuration(since)
			if err == nil {
				opts.Since = time.Now().Add(-d)
			}
		}

		events, err := bus.Query(context.Background(), opts)
		if err != nil {
			return err
		}

		if jsonOutput {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(events)
		}

		if len(events) == 0 {
			fmt.Println("No events found.")
			return nil
		}

		for _, e := range events {
			fmt.Printf("[%s] %-25s source=%-15s %s\n",
				e.CreatedAt.Format("15:04:05"),
				e.Type,
				e.Source,
				e.Payload,
			)
		}
		return nil
	},
}

func init() {
	logsCmd.Flags().String("type", "", "filter by event type prefix")
	logsCmd.Flags().String("agent", "", "filter by agent/source name")
	logsCmd.Flags().String("since", "", "show events since duration (e.g., 1h, 30m)")
	logsCmd.Flags().Int("limit", 50, "max events to return")
	logsCmd.Flags().Bool("json", false, "output in JSON format")

	rootCmd.AddCommand(logsCmd)
}
