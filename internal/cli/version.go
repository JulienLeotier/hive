package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Version is set via ldflags at build time.
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print Hive version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("hive %s\n", Version)
		fmt.Printf("  go:   %s\n", runtime.Version())
		fmt.Printf("  os:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
