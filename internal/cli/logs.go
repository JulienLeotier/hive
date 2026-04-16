package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
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
		workflowID, _ := cmd.Flags().GetString("workflow")
		since, _ := cmd.Flags().GetString("since")
		limit, _ := cmd.Flags().GetInt("limit")
		jsonOutput, _ := cmd.Flags().GetBool("json")
		follow, _ := cmd.Flags().GetBool("follow")
		timeline, _ := cmd.Flags().GetBool("timeline")
		// Story 4.6 / 6.5: --decisions shortcut for decision.* events.
		if dec, _ := cmd.Flags().GetBool("decisions"); dec && eventType == "" {
			eventType = "decision"
		}

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
			if d, err := time.ParseDuration(since); err == nil {
				opts.Since = time.Now().Add(-d)
			}
		}

		filter := func(e event.Event) bool {
			if workflowID != "" {
				return strings.Contains(e.Payload, workflowID)
			}
			return true
		}

		// Seed with initial query
		events, err := bus.Query(context.Background(), opts)
		if err != nil {
			return err
		}

		filtered := events[:0]
		for _, e := range events {
			if filter(e) {
				filtered = append(filtered, e)
			}
		}

		if jsonOutput && !follow {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(filtered)
		}
		if len(filtered) == 0 && !follow {
			fmt.Println("No events found.")
			return nil
		}

		for _, e := range filtered {
			printEvent(cmd, e, timeline, jsonOutput)
		}

		if !follow {
			return nil
		}

		// Follow mode — poll for new events every second until Ctrl-C.
		lastID := int64(0)
		if len(filtered) > 0 {
			lastID = filtered[len(filtered)-1].ID
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		go func() { <-sigs; cancel() }()

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				fresh, err := bus.Query(ctx, event.QueryOpts{
					Type:   eventType,
					Source: agentName,
					Limit:  500,
					Since:  time.Now().Add(-5 * time.Second),
				})
				if err != nil {
					continue
				}
				for _, e := range fresh {
					if e.ID <= lastID || !filter(e) {
						continue
					}
					lastID = e.ID
					printEvent(cmd, e, timeline, jsonOutput)
				}
			}
		}
	},
}

func printEvent(cmd *cobra.Command, e event.Event, timeline, jsonOut bool) {
	if jsonOut {
		_ = json.NewEncoder(cmd.OutOrStdout()).Encode(e)
		return
	}
	if timeline {
		// Vertical timeline-style layout with wall-clock gap hints.
		fmt.Fprintf(cmd.OutOrStdout(), "│\n├─ %s  %s\n│  source=%s  %s\n",
			e.CreatedAt.Format("2006-01-02 15:04:05"), e.Type, e.Source, trunc(e.Payload, 120))
		return
	}
	fmt.Fprintf(cmd.OutOrStdout(), "[%s] %-25s source=%-15s %s\n",
		e.CreatedAt.Format("15:04:05"), e.Type, e.Source, e.Payload)
}

func trunc(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func init() {
	logsCmd.Flags().String("type", "", "filter by event type prefix")
	logsCmd.Flags().String("agent", "", "filter by agent/source name")
	logsCmd.Flags().String("workflow", "", "filter to events mentioning this workflow ID")
	logsCmd.Flags().String("since", "", "show events since duration (e.g., 1h, 30m)")
	logsCmd.Flags().Int("limit", 50, "max events to return")
	logsCmd.Flags().Bool("json", false, "output in JSON format")
	logsCmd.Flags().BoolP("follow", "f", false, "follow new events in real time (Ctrl-C to stop)")
	logsCmd.Flags().Bool("timeline", false, "render as vertical timeline")
	logsCmd.Flags().Bool("decisions", false, "shortcut for --type decision (orchestration decision events)")

	rootCmd.AddCommand(logsCmd)
}
