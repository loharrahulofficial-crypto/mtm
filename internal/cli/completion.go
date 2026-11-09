package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newCompletionCmd builds the completion subcommands. The generators
// name the script after the root passed in, so pass the mtm root —
// standalone construction would produce a script named for the
// completion command itself.
func newCompletionCmd(root *cobra.Command) *cobra.Command {
	bash := &cobra.Command{
		Use:   "bash",
		Short: "Print the bash completion script",
		Long: `Print the bash completion script. Install it with one of:

  mtm completion bash > ~/.local/share/bash-completion/completions/mtm
  # then restart your shell (or: source ~/.local/share/bash-completion/completions/mtm)

  # or add to ~/.bashrc:
  source <(mtm completion bash)`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return root.GenBashCompletionV2(cmd.OutOrStdout(), true)
		},
	}
	zsh := &cobra.Command{
		Use:   "zsh",
		Short: "Print the zsh completion script",
		Long: `Print the zsh completion script. Install it with one of:

  mtm completion zsh > "${fpath[1]}/_mtm"
  # then: autoload -U compinit && compinit

  # or add to ~/.zshrc before compinit:
  source <(mtm completion zsh)`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return root.GenZshCompletion(cmd.OutOrStdout())
		},
	}
	fish := &cobra.Command{
		Use:   "fish",
		Short: "Print the fish completion script",
		Long: `Print the fish completion script. Install it with one of:

  mtm completion fish > ~/.config/fish/completions/mtm.fish
  # then restart your shell (fish picks up the file automatically)

  # or for the current session only:
  mtm completion fish | source`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return root.GenFishCompletion(cmd.OutOrStdout(), true)
		},
	}
	comp := &cobra.Command{
		Use:       "completion",
		Short:     "Print the completion script for bash, zsh or fish",
		Args:      cobra.OnlyValidArgs,
		ValidArgs: []string{"bash", "zsh", "fish"},
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return fmt.Errorf("shell required: bash|zsh|fish")
		},
	}
	comp.AddCommand(bash, zsh, fish)
	return comp
}
