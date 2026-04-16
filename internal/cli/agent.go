package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/JulienLeotier/hive/internal/adapter"
	"github.com/JulienLeotier/hive/internal/agent"
	"github.com/JulienLeotier/hive/internal/config"
	"github.com/JulienLeotier/hive/internal/cost"
	"github.com/JulienLeotier/hive/internal/event"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/JulienLeotier/hive/internal/task"
	"github.com/JulienLeotier/hive/internal/trust"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func countTasksByStatus(ctx context.Context, db *sql.DB) (map[string]int, error) {
	return countByStatus(ctx, db, "tasks")
}

func countWorkflowsByStatus(ctx context.Context, db *sql.DB) (map[string]int, error) {
	return countByStatus(ctx, db, "workflows")
}

func countByStatus(ctx context.Context, db *sql.DB, table string) (map[string]int, error) {
	// table is caller-controlled (constants above) — safe to interpolate.
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`SELECT status, COUNT(*) FROM %s GROUP BY status`, table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int)
	for rows.Next() {
		var s string
		var n int
		if err := rows.Scan(&s, &n); err != nil {
			return nil, err
		}
		out[s] = n
	}
	return out, rows.Err()
}

// summariseCapabilities renders the stored capability JSON as a compact list
// for the status table. Truncates to keep the row readable.
func summariseCapabilities(capsJSON string) string {
	if capsJSON == "" {
		return "—"
	}
	var parsed struct {
		TaskTypes []string `json:"task_types"`
	}
	if err := json.Unmarshal([]byte(capsJSON), &parsed); err != nil {
		return capsJSON
	}
	if len(parsed.TaskTypes) == 0 {
		return "—"
	}
	joined := strings.Join(parsed.TaskTypes, ",")
	if len(joined) > 60 {
		joined = joined[:57] + "…"
	}
	return joined
}

// refreshHealth concurrently pokes each agent's /health endpoint and updates
// the stored status. Story 1.4 AC. Keeps a short timeout so `hive status`
// still responds within 500ms even when some agents are unreachable.
func refreshHealth(ctx context.Context, mgr *agent.Manager, agents []agent.Agent) []agent.Agent {
	type update struct {
		name   string
		status string
	}
	updates := make(chan update, len(agents))

	for _, a := range agents {
		go func(a agent.Agent) {
			// Pull base_url from stored config.
			var agentCfg map[string]string
			_ = json.Unmarshal([]byte(a.Config), &agentCfg)
			baseURL := agentCfg["base_url"]
			if baseURL == "" {
				// Local / subprocess agent: trust the stored status.
				updates <- update{name: a.Name, status: a.HealthStatus}
				return
			}
			hCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			h, err := adapter.NewHTTPAdapter(baseURL).Health(hCtx)
			if err != nil {
				updates <- update{name: a.Name, status: "unavailable"}
				return
			}
			updates <- update{name: a.Name, status: h.Status}
		}(a)
	}

	result := make([]agent.Agent, 0, len(agents))
	byName := map[string]string{}
	for range agents {
		u := <-updates
		byName[u.name] = u.status
	}
	for _, a := range agents {
		if newStatus, ok := byName[a.Name]; ok && newStatus != a.HealthStatus {
			_ = mgr.UpdateHealth(ctx, a.Name, newStatus)
			a.HealthStatus = newStatus
		}
		a.UpdatedAt = time.Now()
		result = append(result, a)
	}
	return result
}

// confirmDetectedType asks the user to accept or change the auto-detected type
// (Story 7.2 AC). Non-TTY stdin = auto-accept so CI doesn't stall.
func confirmDetectedType(detected string) string {
	info, err := os.Stdin.Stat()
	if err != nil || (info.Mode()&os.ModeCharDevice) == 0 {
		return detected
	}
	fmt.Printf("Detected agent type: %s — accept? [Y/n/<override>] ", detected)
	var answer string
	fmt.Scanln(&answer)
	answer = strings.TrimSpace(answer)
	if answer == "" || answer == "y" || answer == "Y" {
		return detected
	}
	if answer == "n" || answer == "N" {
		return ""
	}
	return answer
}

// detectAgentType inspects a local path for well-known files and infers an adapter type.
func detectAgentType(path string) string {
	markers := []struct {
		file string
		kind string
	}{
		{"AGENT.yaml", "claude-code"},
		{".claude/AGENT.yaml", "claude-code"},
		{"CLAUDE.md", "claude-code"},
		{"crewai.yaml", "crewai"},
		{"agents.yaml", "crewai"},
		{"autogen_agent.py", "autogen"},
		{"mcp.json", "mcp"},
		{".mcp.json", "mcp"},
		{"langchain_agent.py", "langchain"},
	}
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(path, m.file)); err == nil {
			return m.kind
		}
	}
	return "http"
}

var addAgentCmd = &cobra.Command{
	Use:   "add-agent",
	Short: "Register an agent with the hive",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		agentType, _ := cmd.Flags().GetString("type")
		url, _ := cmd.Flags().GetString("url")
		path, _ := cmd.Flags().GetString("path")
		configFile, _ := cmd.Flags().GetString("config")

		// Story 1.3 AC: `hive add-agent --name reviewer --config ./agent.yaml` reads
		// capabilities (and other fields) from a YAML config file.
		var fileCapabilities []string
		if configFile != "" {
			data, err := os.ReadFile(configFile)
			if err != nil {
				return fmt.Errorf("reading --config: %w", err)
			}
			var fileCfg struct {
				Name         string   `yaml:"name"`
				Type         string   `yaml:"type"`
				URL          string   `yaml:"url"`
				Path         string   `yaml:"path"`
				Capabilities []string `yaml:"capabilities"`
			}
			if err := yaml.Unmarshal(data, &fileCfg); err != nil {
				return fmt.Errorf("parsing --config: %w", err)
			}
			if name == "" {
				name = fileCfg.Name
			}
			if agentType == "" {
				agentType = fileCfg.Type
			}
			if url == "" {
				url = fileCfg.URL
			}
			if path == "" {
				path = fileCfg.Path
			}
			fileCapabilities = fileCfg.Capabilities
		}

		if name == "" {
			return fmt.Errorf("--name is required")
		}

		// --path: local agent — auto-detect type from project markers, synthesize URL.
		if path != "" {
			abs, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("resolving path: %w", err)
			}
			if _, err := os.Stat(abs); err != nil {
				return fmt.Errorf("agent path: %w", err)
			}
			if agentType == "" || agentType == "http" {
				detected := detectAgentType(abs)
				// Story 7.2 AC: confirm detected type with user before registering.
				// --yes skips the prompt for scripted callers (CI etc.).
				if yes, _ := cmd.Flags().GetBool("yes"); yes {
					agentType = detected
				} else {
					agentType = confirmDetectedType(detected)
				}
			}
			if url == "" {
				url = "file://" + abs
			}
		}

		if url == "" {
			return fmt.Errorf("--url or --path is required")
		}
		if agentType == "" {
			agentType = "http"
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

		mgr := agent.NewManager(store.DB)

		var a *agent.Agent
		ctx := context.Background()

		// Stories 13.1-13.4: build the right adapter per --type so Declare()
		// returns framework-native capabilities rather than generic defaults.
		switch {
		case path != "" && (agentType == "crewai" || agentType == "autogen" || agentType == "langchain"):
			// Subprocess-backed local agents — declare via adapter, register as local.
			var caps adapter.AgentCapabilities
			switch agentType {
			case "crewai":
				caps, _ = adapter.NewCrewAIAdapter(path, name).Declare(ctx)
			case "autogen":
				caps, _ = adapter.NewAutoGenAdapter("file://"+path, name).Declare(ctx)
			case "langchain":
				caps, _ = adapter.NewLangChainAdapter("file://"+path, name).Declare(ctx)
			}
			capsMap := map[string]any{"name": caps.Name, "task_types": caps.TaskTypes}
			a, err = mgr.RegisterLocal(ctx, name, agentType, path, capsMap)

		case agentType == "openai":
			assistantID, _ := cmd.Flags().GetString("assistant-id")
			apiKey, _ := cmd.Flags().GetString("api-key")
			if assistantID == "" {
				return fmt.Errorf("--assistant-id is required for openai adapters")
			}
			if apiKey == "" {
				apiKey = os.Getenv("OPENAI_API_KEY")
			}
			if apiKey == "" {
				return fmt.Errorf("--api-key or OPENAI_API_KEY env var is required")
			}
			oa := adapter.NewOpenAIAdapter(assistantID, apiKey, name)
			caps, _ := oa.Declare(ctx)
			capsMap := map[string]any{"name": caps.Name, "task_types": caps.TaskTypes, "assistant_id": assistantID}
			a, err = mgr.RegisterLocal(ctx, name, "openai", assistantID, capsMap)

		case path != "":
			// Story 1.3: if --config declared capabilities, feed them into RegisterLocal.
			var capsMap map[string]any
			if len(fileCapabilities) > 0 {
				capsMap = map[string]any{"name": name, "task_types": fileCapabilities}
			}
			a, err = mgr.RegisterLocal(ctx, name, agentType, path, capsMap)

		default:
			a, err = mgr.Register(ctx, name, agentType, url)
		}

		if err != nil {
			return fmt.Errorf("registration failed: %w", err)
		}

		fmt.Printf("Agent registered: %s (%s)\n", a.Name, a.Type)
		fmt.Printf("  ID:     %s\n", a.ID)
		fmt.Printf("  Health: %s\n", a.HealthStatus)
		fmt.Printf("  Caps:   %s\n", a.Capabilities)
		return nil
	},
}

var removeAgentCmd = &cobra.Command{
	Use:   "remove-agent [name]",
	Short: "Remove an agent from the hive",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load("hive.yaml")
		if err != nil {
			return err
		}

		store, err := storage.Open(cfg.DataDir)
		if err != nil {
			return err
		}
		defer store.Close()

		mgr := agent.NewManager(store.DB)
		if err := mgr.Remove(context.Background(), args[0]); err != nil {
			return err
		}

		fmt.Printf("Agent removed: %s\n", args[0])
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show hive status — agents, health, and active tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
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

		mgr := agent.NewManager(store.DB)
		ctx := context.Background()
		agents, err := mgr.List(ctx)
		if err != nil {
			return err
		}

		// Story 1.4 AC: "health is refreshed by calling each agent's /health
		// endpoint". Skip when --no-refresh is set or when the agent was
		// registered with a local path (subprocess adapters are always "fresh").
		noRefresh, _ := cmd.Flags().GetBool("no-refresh")
		if !noRefresh {
			agents = refreshHealth(ctx, mgr, agents)
		}

		showCosts, _ := cmd.Flags().GetBool("costs")

		// Tasks and workflows rollup (Story 6.1)
		taskCounts, _ := countTasksByStatus(ctx, store.DB)
		workflowCounts, _ := countWorkflowsByStatus(ctx, store.DB)
		recentEvents, _ := event.NewBus(store.DB).Query(ctx, event.QueryOpts{Limit: 5, Since: time.Now().Add(-15 * time.Minute)})

		if jsonOutput {
			out := map[string]any{
				"agents":    agents,
				"tasks":     taskCounts,
				"workflows": workflowCounts,
				"events":    recentEvents,
			}
			if showCosts {
				tracker := cost.NewTracker(store.DB)
				summaries, _ := tracker.ByAgent(ctx)
				alerts, _ := tracker.EvaluateAlerts(ctx)
				out["costs"] = summaries
				out["budget_alerts"] = alerts
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
		}

		if len(agents) == 0 {
			fmt.Println("No agents registered. Use 'hive add-agent' to register one.")
			return nil
		}

		fmt.Printf("%-20s %-10s %-12s %-10s %-20s %s\n", "NAME", "TYPE", "HEALTH", "TRUST", "LAST CHECK", "CAPABILITIES")
		fmt.Printf("%-20s %-10s %-12s %-10s %-20s %s\n", "----", "----", "------", "-----", "----------", "------------")
		for _, a := range agents {
			lastCheck := a.UpdatedAt.Format("2006-01-02 15:04:05")
			if a.UpdatedAt.IsZero() {
				lastCheck = "—"
			}
			fmt.Printf("%-20s %-10s %-12s %-10s %-20s %s\n",
				a.Name, a.Type, a.HealthStatus, a.TrustLevel, lastCheck, summariseCapabilities(a.Capabilities))
		}
		fmt.Printf("\nTotal: %d agents\n", len(agents))

		if len(taskCounts) > 0 {
			fmt.Println("\nTasks:")
			for _, s := range []string{"pending", "assigned", "running", "completed", "failed"} {
				if n := taskCounts[s]; n > 0 {
					fmt.Printf("  %-12s %d\n", s, n)
				}
			}
		}
		if len(workflowCounts) > 0 {
			fmt.Println("\nWorkflows:")
			for _, s := range []string{"idle", "running", "completed", "failed"} {
				if n := workflowCounts[s]; n > 0 {
					fmt.Printf("  %-12s %d\n", s, n)
				}
			}
		}
		if len(recentEvents) > 0 {
			fmt.Println("\nRecent events (last 15m):")
			for _, e := range recentEvents {
				fmt.Printf("  [%s] %-25s source=%s\n", e.CreatedAt.Format("15:04:05"), e.Type, e.Source)
			}
		}

		if showCosts {
			tracker := cost.NewTracker(store.DB)
			summaries, err := tracker.ByAgent(ctx)
			if err != nil {
				return err
			}
			fmt.Println()
			fmt.Printf("%-20s %-12s %-10s\n", "AGENT", "TOTAL COST", "TASKS")
			fmt.Printf("%-20s %-12s %-10s\n", "-----", "----------", "-----")
			var total float64
			for _, s := range summaries {
				fmt.Printf("%-20s $%-11.4f %-10d\n", s.AgentName, s.TotalCost, s.TaskCount)
				total += s.TotalCost
			}
			fmt.Printf("Total spend: $%.4f\n", total)

			alerts, err := tracker.EvaluateAlerts(ctx)
			if err == nil {
				for _, a := range alerts {
					if a.Breached {
						fmt.Printf("  ⚠  %s over budget: $%.4f / $%.4f daily\n", a.AgentName, a.Spend, a.DailyLimit)
					}
				}
			}
		}
		return nil
	},
}

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage agents",
}

var agentTrustCmd = &cobra.Command{
	Use:   "trust",
	Short: "Inspect or set an agent's trust level",
}

var agentTrustGetCmd = &cobra.Command{
	Use:   "get [agent-name]",
	Short: "Show current trust level and stats for an agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load("hive.yaml")
		if err != nil {
			return err
		}
		store, err := storage.Open(cfg.DataDir)
		if err != nil {
			return err
		}
		defer store.Close()

		ctx := context.Background()
		mgr := agent.NewManager(store.DB)
		a, err := mgr.GetByName(ctx, args[0])
		if err != nil {
			return err
		}

		engine := trust.NewEngine(store.DB, trust.DefaultThresholds())
		stats, err := engine.GetStats(ctx, a.ID)
		if err != nil {
			return err
		}

		fmt.Printf("Agent:       %s\n", a.Name)
		fmt.Printf("Trust level: %s\n", a.TrustLevel)
		fmt.Printf("Total tasks: %d (success=%d, failed=%d)\n", stats.TotalTasks, stats.Successes, stats.Failures)
		fmt.Printf("Error rate:  %.2f%%\n", stats.ErrorRate*100)
		return nil
	},
}

var agentTrustSetCmd = &cobra.Command{
	Use:   "set [agent-name] [level]",
	Short: "Manually set an agent's trust level (supervised|guided|autonomous|trusted)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, level := args[0], args[1]
		switch level {
		case trust.LevelSupervised, trust.LevelGuided, trust.LevelAutonomous, trust.LevelTrusted:
		default:
			return fmt.Errorf("unknown trust level %q — use supervised|guided|autonomous|trusted", level)
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

		ctx := context.Background()
		mgr := agent.NewManager(store.DB)
		a, err := mgr.GetByName(ctx, name)
		if err != nil {
			return err
		}

		engine := trust.NewEngine(store.DB, trust.DefaultThresholds())
		if err := engine.SetManual(ctx, a.ID, level); err != nil {
			return err
		}

		fmt.Printf("Trust level for %s set to %s\n", name, level)
		return nil
	},
}

var agentStatsCmd = &cobra.Command{
	Use:   "stats [agent-name]",
	Short: "Show task stats + bid history + token balance for an agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load("hive.yaml")
		if err != nil {
			return err
		}
		store, err := storage.Open(cfg.DataDir)
		if err != nil {
			return err
		}
		defer store.Close()

		ctx := context.Background()
		mgr := agent.NewManager(store.DB)
		a, err := mgr.GetByName(ctx, args[0])
		if err != nil {
			return err
		}

		// Trust stats
		engine := trust.NewEngine(store.DB, trust.DefaultThresholds())
		stats, err := engine.GetStats(ctx, a.ID)
		if err != nil {
			return err
		}

		fmt.Printf("Agent: %s (id=%s)\n", a.Name, a.ID)
		fmt.Printf("  Health:       %s\n", a.HealthStatus)
		fmt.Printf("  Trust level:  %s\n", a.TrustLevel)
		fmt.Printf("  Total tasks:  %d (success=%d, failed=%d)\n", stats.TotalTasks, stats.Successes, stats.Failures)
		fmt.Printf("  Error rate:   %.2f%%\n", stats.ErrorRate*100)

		// Bid stats (if any)
		var bidCount, winCount int
		_ = store.DB.QueryRowContext(ctx,
			`SELECT COUNT(*), COALESCE(SUM(won), 0) FROM bids WHERE agent_name = ?`,
			args[0]).Scan(&bidCount, &winCount)
		if bidCount > 0 {
			fmt.Printf("  Bids:         %d (won=%d, rate=%.1f%%)\n", bidCount, winCount, float64(winCount)*100/float64(bidCount))
		}

		// Token balance (if any)
		var balance float64
		_ = store.DB.QueryRowContext(ctx,
			`SELECT COALESCE(balance, 0) FROM agent_tokens WHERE agent_name = ?`, args[0]).Scan(&balance)
		if balance > 0 {
			fmt.Printf("  Token balance: %.2f\n", balance)
		}
		return nil
	},
}

var agentTrustOverrideCmd = &cobra.Command{
	Use:   "override [agent] [task-type] [level]",
	Short: "Set a per-task-type trust override",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		reason, _ := cmd.Flags().GetString("reason")
		if !trust.IsValidLevel(args[2]) {
			return fmt.Errorf("invalid trust level %q", args[2])
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

		ctx := context.Background()
		mgr := agent.NewManager(store.DB)
		a, err := mgr.GetByName(ctx, args[0])
		if err != nil {
			return err
		}

		engine := trust.NewEngine(store.DB, trust.DefaultThresholds())
		if err := engine.SetOverride(ctx, a.ID, args[1], args[2], reason); err != nil {
			return err
		}
		fmt.Printf("Override set: %s[%s] = %s\n", args[0], args[1], args[2])
		return nil
	},
}

var agentTrustClearOverrideCmd = &cobra.Command{
	Use:   "clear-override [agent] [task-type]",
	Short: "Remove a per-task-type trust override",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load("hive.yaml")
		if err != nil {
			return err
		}
		store, err := storage.Open(cfg.DataDir)
		if err != nil {
			return err
		}
		defer store.Close()

		ctx := context.Background()
		mgr := agent.NewManager(store.DB)
		a, err := mgr.GetByName(ctx, args[0])
		if err != nil {
			return err
		}

		engine := trust.NewEngine(store.DB, trust.DefaultThresholds())
		if err := engine.RemoveOverride(ctx, a.ID, args[1]); err != nil {
			return err
		}
		fmt.Printf("Override cleared: %s[%s]\n", args[0], args[1])
		return nil
	},
}

var agentSwapCmd = &cobra.Command{
	Use:   "swap [old-name] [new-name]",
	Short: "Swap a failing agent for a replacement — reassigns in-flight tasks",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		oldName, newName := args[0], args[1]

		cfg, err := config.Load("hive.yaml")
		if err != nil {
			return err
		}
		store, err := storage.Open(cfg.DataDir)
		if err != nil {
			return err
		}
		defer store.Close()

		mgr := agent.NewManager(store.DB)
		ctx := context.Background()

		if _, err := mgr.GetByName(ctx, oldName); err != nil {
			return fmt.Errorf("old agent: %w", err)
		}
		replacement, err := mgr.GetByName(ctx, newName)
		if err != nil {
			return fmt.Errorf("replacement agent: %w", err)
		}
		if replacement.HealthStatus != "healthy" {
			return fmt.Errorf("replacement agent %s is not healthy (%s)", newName, replacement.HealthStatus)
		}

		bus := event.NewBus(store.DB)
		router := task.NewRouter(store.DB).WithBus(bus)
		taskStore := task.NewStore(store.DB, bus)
		supervisor := task.NewCheckpointSupervisor(taskStore, router, 30*time.Second, 5*time.Minute)

		// Story 5.4 AC: "in-progress tasks are checkpointed" before swap.
		// Poll every running task owned by the old agent so we have a fresh
		// checkpoint row, then reassign.
		var agentCfg map[string]string
		old, err := mgr.GetByName(ctx, oldName)
		if err == nil {
			_ = json.Unmarshal([]byte(old.Config), &agentCfg)
		}
		baseURL := agentCfg["base_url"]
		if baseURL != "" {
			a := adapter.NewHTTPAdapter(baseURL)
			supervisor.WithAdapterResolver(func(agentID string) adapter.Adapter { return a })
			_ = supervisor.Poll(ctx)
		}

		n, err := router.ReassignAgentTasks(ctx, oldName, "agent swap → "+newName)
		if err != nil {
			return fmt.Errorf("reassigning tasks: %w", err)
		}

		if err := mgr.UpdateHealth(ctx, oldName, "unavailable"); err != nil {
			return fmt.Errorf("marking old agent unavailable: %w", err)
		}

		_, _ = bus.Publish(ctx, "agent.swapped", "cli", map[string]any{
			"from": oldName, "to": newName, "reassigned": n,
		})

		fmt.Printf("Swapped %s → %s (%d tasks checkpointed + reassigned)\n", oldName, newName, n)
		return nil
	},
}

func init() {
	addAgentCmd.Flags().String("name", "", "agent name (required)")
	addAgentCmd.Flags().String("type", "", "agent type (http, claude-code, mcp, crewai, autogen, langchain, openai); auto-detected when --path is given")
	addAgentCmd.Flags().String("url", "", "agent URL (required unless --path or an openai adapter is given)")
	addAgentCmd.Flags().String("path", "", "local filesystem path to an agent project (auto-detects type)")
	addAgentCmd.Flags().String("assistant-id", "", "OpenAI assistant ID (required for --type openai)")
	addAgentCmd.Flags().String("api-key", "", "OpenAI API key (falls back to $OPENAI_API_KEY)")
	addAgentCmd.Flags().String("config", "", "YAML file providing name/type/url/path (CLI flags override file)")
	addAgentCmd.Flags().Bool("yes", false, "accept auto-detected type without prompting")

	agentTrustOverrideCmd.Flags().String("reason", "", "why this override is being set")

	statusCmd.Flags().Bool("json", false, "output in JSON format")
	statusCmd.Flags().Bool("costs", false, "include cost rollup and budget alerts")
	statusCmd.Flags().Bool("no-refresh", false, "skip live /health probes and show stored statuses only")

	agentCmd.AddCommand(agentSwapCmd)
	agentCmd.AddCommand(agentStatsCmd)
	agentCmd.AddCommand(agentTrustCmd)
	agentTrustCmd.AddCommand(agentTrustGetCmd)
	agentTrustCmd.AddCommand(agentTrustSetCmd)
	agentTrustCmd.AddCommand(agentTrustOverrideCmd)
	agentTrustCmd.AddCommand(agentTrustClearOverrideCmd)

	rootCmd.AddCommand(addAgentCmd)
	rootCmd.AddCommand(removeAgentCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(agentCmd)
}
