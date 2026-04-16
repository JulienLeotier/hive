package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/JulienLeotier/hive/internal/config"
	"github.com/JulienLeotier/hive/internal/federation"
	"github.com/JulienLeotier/hive/internal/storage"
	"github.com/spf13/cobra"
)

var federationCmd = &cobra.Command{
	Use:   "federation",
	Short: "Manage federation links with other Hive deployments",
}

var federationAddCmd = &cobra.Command{
	Use:     "add [name] [url]",
	Aliases: []string{"connect"}, // Story 19.1 AC uses `hive federation connect`
	Short:   "Register a federation link (alias: connect)",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		shared, _ := cmd.Flags().GetString("shared")
		caFile, _ := cmd.Flags().GetString("ca")
		certFile, _ := cmd.Flags().GetString("cert")
		keyFile, _ := cmd.Flags().GetString("key")

		caPEM, err := readFileIfPresent(caFile)
		if err != nil {
			return err
		}
		certPEM, err := readFileIfPresent(certFile)
		if err != nil {
			return err
		}
		keyPEM, err := readFileIfPresent(keyFile)
		if err != nil {
			return err
		}

		store, cleanup, err := openFederationStore()
		if err != nil {
			return err
		}
		defer cleanup()

		link := &federation.Link{
			Name: args[0], URL: args[1], Status: "active",
			SharedCaps: parseCSV(shared),
		}
		if err := store.Add(context.Background(), link, caPEM, certPEM, keyPEM); err != nil {
			return err
		}
		fmt.Printf("Federation link added: %s → %s\n", args[0], args[1])
		return nil
	},
}

var federationListCmd = &cobra.Command{
	Use:   "list",
	Short: "List federation links",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, cleanup, err := openFederationStore()
		if err != nil {
			return err
		}
		defer cleanup()

		links, err := store.List(context.Background())
		if err != nil {
			return err
		}
		if len(links) == 0 {
			fmt.Println("No federation links. Use 'hive federation add'.")
			return nil
		}
		fmt.Printf("%-20s %-12s %-40s %s\n", "NAME", "STATUS", "URL", "CAPABILITIES")
		for _, l := range links {
			fmt.Printf("%-20s %-12s %-40s %s\n", l.Name, l.Status, l.URL, strings.Join(l.SharedCaps, ","))
		}
		return nil
	},
}

var federationRemoveCmd = &cobra.Command{
	Use:   "remove [name]",
	Short: "Remove a federation link",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, cleanup, err := openFederationStore()
		if err != nil {
			return err
		}
		defer cleanup()
		if err := store.Remove(context.Background(), args[0]); err != nil {
			return err
		}
		fmt.Printf("Federation link removed: %s\n", args[0])
		return nil
	},
}

func openFederationStore() (*federation.Store, func(), error) {
	cfg, err := config.Load("hive.yaml")
	if err != nil {
		return nil, nil, err
	}
	st, err := storage.Open(cfg.DataDir)
	if err != nil {
		return nil, nil, err
	}
	return federation.NewStore(st.DB), func() { st.Close() }, nil
}

func readFileIfPresent(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	return string(data), nil
}

func parseCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := parts[:0]
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func init() {
	federationAddCmd.Flags().String("shared", "", "comma-separated capabilities to share with this peer")
	federationAddCmd.Flags().String("ca", "", "path to trusted CA certificate (PEM)")
	federationAddCmd.Flags().String("cert", "", "path to client certificate (PEM)")
	federationAddCmd.Flags().String("key", "", "path to client private key (PEM)")

	federationCmd.AddCommand(federationAddCmd)
	federationCmd.AddCommand(federationListCmd)
	federationCmd.AddCommand(federationRemoveCmd)
	rootCmd.AddCommand(federationCmd)
}
