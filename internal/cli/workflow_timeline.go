package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/JulienLeotier/hive/internal/config"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/spf13/cobra"
)

// workflowTimelineCmd renders a Gantt-like timeline for a completed workflow.
// Story 6.3 AC: "each task with start/end/duration/agent/status, parallel
// tasks visually indicated, critical path highlighted".
var workflowTimelineCmd = &cobra.Command{
	Use:   "workflow-timeline [workflow-id]",
	Short: "Render a Gantt-like timeline for a workflow's tasks",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		workflowID := args[0]

		cfg, err := config.Load("hive.yaml")
		if err != nil {
			return err
		}
		store, err := storage.Open(cfg.DataDir)
		if err != nil {
			return err
		}
		defer store.Close()

		rows, err := store.DB.QueryContext(context.Background(), `
			SELECT t.id, t.type, COALESCE(a.name,''), t.status,
			       COALESCE(t.started_at,''), COALESCE(t.completed_at,''),
			       COALESCE(t.depends_on,'[]')
			FROM tasks t LEFT JOIN agents a ON a.id = t.agent_id
			WHERE t.workflow_id = ?
			ORDER BY t.started_at`, workflowID)
		if err != nil {
			return err
		}
		defer rows.Close()

		var items []timelineRow
		for rows.Next() {
			var r timelineRow
			var startStr, endStr, deps string
			if err := rows.Scan(&r.id, &r.tType, &r.agent, &r.status, &startStr, &endStr, &deps); err != nil {
				return err
			}
			r.started, _ = time.Parse("2006-01-02 15:04:05", startStr)
			r.completed, _ = time.Parse("2006-01-02 15:04:05", endStr)
			_ = json.Unmarshal([]byte(deps), &r.depends)
			items = append(items, r)
		}
		if len(items) == 0 {
			fmt.Printf("No tasks found for workflow %s\n", workflowID)
			return nil
		}

		// Critical path: longest chain of completed tasks by duration.
		durations := map[string]time.Duration{}
		for _, r := range items {
			if !r.started.IsZero() && !r.completed.IsZero() {
				durations[r.id] = r.completed.Sub(r.started)
			}
		}
		critical := longestDurationPath(items, durations)
		onCritical := map[string]bool{}
		for _, id := range critical {
			onCritical[id] = true
		}

		// Parallel detection: two tasks are "in parallel" if their windows overlap.
		sort.Slice(items, func(i, j int) bool { return items[i].started.Before(items[j].started) })
		overlap := map[string]bool{}
		for i, a := range items {
			for _, b := range items[i+1:] {
				if !a.completed.IsZero() && !b.started.IsZero() && a.completed.After(b.started) {
					overlap[a.id] = true
					overlap[b.id] = true
				}
			}
		}

		fmt.Printf("%-14s %-18s %-14s %-10s %-10s %-8s %s\n",
			"TASK", "TYPE", "AGENT", "START", "END", "DURATION", "FLAGS")
		for _, r := range items {
			start := "—"
			end := "—"
			dur := "—"
			if !r.started.IsZero() {
				start = r.started.Format("15:04:05")
			}
			if !r.completed.IsZero() {
				end = r.completed.Format("15:04:05")
			}
			if d, ok := durations[r.id]; ok {
				dur = d.String()
			}
			flags := r.status
			if onCritical[r.id] {
				flags += " ★critical"
			}
			if overlap[r.id] {
				flags += " ∥parallel"
			}
			fmt.Printf("%-14s %-18s %-14s %-10s %-10s %-8s %s\n",
				r.id[max(0, len(r.id)-12):], r.tType, r.agent, start, end, dur, flags)
		}
		return nil
	},
}

// timelineRow is the row type used by the workflow-timeline command.
type timelineRow struct {
	id, tType, agent, status string
	started, completed       time.Time
	depends                  []string
}

func longestDurationPath(items []timelineRow, durations map[string]time.Duration) []string {
	byID := map[string]timelineRow{}
	for _, r := range items {
		byID[r.id] = r
	}
	memo := map[string][]string{}
	var longest func(id string) []string
	longest = func(id string) []string {
		if c, ok := memo[id]; ok {
			return c
		}
		var best []string
		bestDur := time.Duration(0)
		for _, dep := range byID[id].depends {
			p := longest(dep)
			d := time.Duration(0)
			for _, pid := range p {
				d += durations[pid]
			}
			if d > bestDur || (d == bestDur && len(p) > len(best)) {
				best = p
				bestDur = d
			}
		}
		result := append([]string{}, best...)
		result = append(result, id)
		memo[id] = result
		return result
	}

	var winner []string
	var winnerDur time.Duration
	for _, r := range items {
		p := longest(r.id)
		d := time.Duration(0)
		for _, pid := range p {
			d += durations[pid]
		}
		if d > winnerDur {
			winner = p
			winnerDur = d
		}
	}
	return winner
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func init() {
	rootCmd.AddCommand(workflowTimelineCmd)
}
