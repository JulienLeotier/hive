package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JulienLeotier/hive/internal/config"
	"github.com/JulienLeotier/hive/internal/dialog"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/spf13/cobra"
)

var dialogsCmd = &cobra.Command{
	Use:   "dialogs",
	Short: "View agent dialog threads",
}

var dialogsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List dialog threads",
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOutput, _ := cmd.Flags().GetBool("json")

		cfg, _ := config.Load("hive.yaml")
		store, err := storage.Open(cfg.DataDir)
		if err != nil {
			return err
		}
		defer store.Close()

		mgr := dialog.NewManager(store.DB)
		threads, err := mgr.ListThreads(context.Background())
		if err != nil {
			return err
		}

		if jsonOutput {
			return json.NewEncoder(cmd.OutOrStdout()).Encode(threads)
		}

		if len(threads) == 0 {
			fmt.Println("No dialog threads.")
			return nil
		}

		for _, t := range threads {
			fmt.Printf("[%s] %s — %s ↔ %s (%d messages)\n",
				t.Status, t.Topic, t.InitiatorAgentID, t.ParticipantAgentID, t.MessageCount)
		}
		return nil
	},
}

var dialogsShowCmd = &cobra.Command{
	Use:   "show [thread-id]",
	Short: "Show messages in a dialog thread",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load("hive.yaml")
		store, err := storage.Open(cfg.DataDir)
		if err != nil {
			return err
		}
		defer store.Close()

		mgr := dialog.NewManager(store.DB)
		messages, err := mgr.GetMessages(context.Background(), args[0])
		if err != nil {
			return err
		}

		if len(messages) == 0 {
			fmt.Println("No messages in this thread.")
			return nil
		}

		for _, m := range messages {
			fmt.Printf("[%s] %s: %s\n", m.CreatedAt.Format("15:04:05"), m.SenderAgentID, m.Content)
		}
		return nil
	},
}

func init() {
	dialogsListCmd.Flags().Bool("json", false, "output as JSON")
	dialogsCmd.AddCommand(dialogsListCmd, dialogsShowCmd)
	rootCmd.AddCommand(dialogsCmd)
}
