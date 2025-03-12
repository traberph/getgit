package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate completion script",
	Long: `Generates shell completion scripts.

To load completions:

Bash:
  # Generate and load for current session
  $ source <(getgit completion bash)

  # Install system-wide (Linux)
  $ getgit completion bash > /etc/bash_completion.d/getgit
  # Install system-wide (macOS)
  $ getgit completion bash > /usr/local/etc/bash_completion.d/getgit

Zsh:
  # Enable completions in your shell
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # Generate and install
  $ getgit completion zsh > "${fpath[1]}/_getgit"

Fish:
  # Generate and install
  $ getgit completion fish > ~/.config/fish/completions/getgit.fish

PowerShell:
  # Generate and load for current session
  PS> getgit completion powershell | Out-String | Invoke-Expression

  # Install for future sessions
  PS> getgit completion powershell > getgit.ps1
  # Source this file from your PowerShell profile`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.ExactValidArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "bash":
			cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
