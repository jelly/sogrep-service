package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// completionsCmd represents the completions command
var completionsCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish]",
	Short: "Generate completion script",
	Long: `To load completions:

Bash:

  $ source <(sogrep-service completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ sogrep-service completion bash > /etc/bash_completion.d/sogrep-service
  # macOS:
  $ sogrep-service completion bash > /usr/local/etc/bash_completion.d/sogrep-service

Zsh:

  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ sogrep-service completion zsh > "${fpath[1]}/_sogrep-service"

  # You will need to start a new shell for this setup to take effect.

fish:

  $ sogrep-service completion fish | source

  # To load completions for each session, execute once:
  $ sogrep-service completion fish > ~/.config/fish/completions/sogrep-service.fish
`,
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
		}
	},
}

func init() {
	RootCmd.AddCommand(completionsCmd)
}
