package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:       "completion [bash|zsh|fish|powershell]",
	Short:     "Generate shell completion script",
	Long:      completionLong,
	Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return fmt.Errorf("unknown shell: %s", args[0])
	},
}

const completionLong = `Generate the autocompletion script for the specified shell.

  Bash:    hive completion bash > /etc/bash_completion.d/hive
  Zsh:     hive completion zsh > "${fpath[1]}/_hive"
  Fish:    hive completion fish > ~/.config/fish/completions/hive.fish
  Pwsh:    hive completion powershell | Out-String | Invoke-Expression`

func init() {
	rootCmd.AddCommand(completionCmd)
}
