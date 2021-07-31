package main

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate completion script",
	Long: `To load completions:

Bash:

  $ source <(reproducible-proxy completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ reproducible-proxy completion bash > /etc/bash_completion.d/reproducible-proxy
  # macOS:
  $ reproducible-proxy completion bash > /usr/local/etc/bash_completion.d/reproducible-proxy

Zsh:

  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ reproducible-proxy completion zsh > "${fpath[1]}/_reproducible-proxy"

  # You will need to start a new shell for this setup to take effect.

fish:

  $ reproducible-proxy completion fish | source

  # To load completions for each session, execute once:
  $ reproducible-proxy completion fish > ~/.config/fish/completions/reproducible-proxy.fish

PowerShell:

  PS> reproducible-proxy completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> reproducible-proxy completion powershell > reproducible-proxy.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.ExactValidArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "bash":
			cmd.Root().GenBashCompletionV2(os.Stdout, true)
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

type completeFunc func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)

func completeArgNone(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return nil, cobra.ShellCompDirectiveNoFileComp
}

func completeArgDefault(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return nil, cobra.ShellCompDirectiveDefault
}

// completeArgList takes a list of completion functions and completes each arg separately
func completeArgList(funcList []completeFunc) completeFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		pos := len(args)
		if pos >= len(funcList) {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return funcList[pos](cmd, args, toComplete)
	}
}
