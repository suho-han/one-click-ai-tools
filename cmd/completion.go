package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var completionShells = []string{"bash", "zsh", "fish"}

var completionCmd = &cobra.Command{
	Use:       "completion <bash|zsh|fish>",
	Short:     "Print a shell completion script",
	GroupID:   "maintenance",
	Args:      cobra.MaximumNArgs(1),
	ValidArgs: completionShells,
	RunE: func(cmd *cobra.Command, args []string) error {
		shell := ""
		if len(args) == 1 {
			shell = args[0]
		}
		w := cmd.OutOrStdout()
		switch shell {
		case "bash":
			return rootCmd.GenBashCompletionV2(w, true)
		case "zsh":
			return rootCmd.GenZshCompletion(w)
		case "fish":
			return rootCmd.GenFishCompletion(w, true)
		}
		errw := cmd.ErrOrStderr()
		fmt.Fprintf(errw, "Supported shells: %s\n", strings.Join(completionShells, ", "))
		fmt.Fprintln(errw, "Example: oct completion zsh")
		return &usageError{fmt.Sprintf("pass a supported shell explicitly: oct completion <%s>", strings.Join(completionShells, "|"))}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
