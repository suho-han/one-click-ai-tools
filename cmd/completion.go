package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

const (
	completionBlockStart = "# >>> oct completion >>>"
	completionBlockEnd   = "# <<< oct completion <<<"
)

type completionShell struct {
	name       string
	extension  string
	profile    string
	sourceTmpl string
}

var supportedCompletionShells = []completionShell{
	{name: "bash", extension: "bash", profile: ".bashrc", sourceTmpl: "source %s"},
	{name: "zsh", extension: "zsh", profile: ".zshrc", sourceTmpl: "source %s"},
	{name: "fish", extension: "fish"},
	{name: "powershell", extension: "ps1", profile: powershellProfilePath(), sourceTmpl: ". %s"},
}

var completionNoDescriptions bool

var completionCmd = &cobra.Command{
	Use:     "completion",
	GroupID: "help",
	Short:   "🧩 Generate, install, or uninstall shell completion",
	Long: `Generate shell completion scripts, install them for new shell sessions, or uninstall the files and startup hook that oct added.

The install command writes to user-owned files under your home directory. It cannot alter completion state in the already-running parent shell; source the installed file once to activate it immediately in the current terminal.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var completionInstallCmd = &cobra.Command{
	Use:       "install [bash|zsh|fish|powershell]",
	Short:     "Install shell completion for new sessions",
	Args:      cobra.ExactArgs(1),
	ValidArgs: completionShellNames(),
	RunE: func(cmd *cobra.Command, args []string) error {
		shell, err := parseCompletionShell(args[0])
		if err != nil {
			return err
		}
		return installCompletion(cmd, shell, completionNoDescriptions)
	},
}

var completionUninstallCmd = &cobra.Command{
	Use:       "uninstall [bash|zsh|fish|powershell]",
	Short:     "Uninstall shell completion installed by oct",
	Args:      cobra.ExactArgs(1),
	ValidArgs: completionShellNames(),
	RunE: func(cmd *cobra.Command, args []string) error {
		shell, err := parseCompletionShell(args[0])
		if err != nil {
			return err
		}
		return uninstallCompletion(cmd, shell)
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
	completionCmd.AddCommand(completionInstallCmd, completionUninstallCmd)
	completionInstallCmd.Flags().BoolVar(&completionNoDescriptions, "no-descriptions", false, "disable completion descriptions")
	for _, shell := range supportedCompletionShells {
		completionCmd.AddCommand(newCompletionGenerateCmd(shell))
	}
}

func newCompletionGenerateCmd(shell completionShell) *cobra.Command {
	var noDescriptions bool
	cmd := &cobra.Command{
		Use:   shell.name,
		Short: fmt.Sprintf("Generate the autocompletion script for %s", shell.name),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateCompletion(cmd.Root(), shell, noDescriptions, cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&noDescriptions, "no-descriptions", false, "disable completion descriptions")
	return cmd
}

func generateCompletion(root *cobra.Command, shell completionShell, noDescriptions bool, out io.Writer) error {
	switch shell.name {
	case "bash":
		return root.GenBashCompletionV2(out, !noDescriptions)
	case "zsh":
		if noDescriptions {
			return root.GenZshCompletionNoDesc(out)
		}
		return root.GenZshCompletion(out)
	case "fish":
		return root.GenFishCompletion(out, !noDescriptions)
	case "powershell":
		if noDescriptions {
			return root.GenPowerShellCompletion(out)
		}
		return root.GenPowerShellCompletionWithDesc(out)
	default:
		return fmt.Errorf("unsupported shell %q", shell.name)
	}
}

func parseCompletionShell(name string) (completionShell, error) {
	for _, shell := range supportedCompletionShells {
		if shell.name == name {
			return shell, nil
		}
	}
	return completionShell{}, fmt.Errorf("unsupported shell %q", name)
}

func completionShellNames() []string {
	names := make([]string, 0, len(supportedCompletionShells))
	for _, shell := range supportedCompletionShells {
		names = append(names, shell.name)
	}
	return names
}
