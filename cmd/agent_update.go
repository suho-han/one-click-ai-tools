package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/suho-han/one-click-ai-tools/internal/update"
)

var (
	agentUpdateDryRun  bool
	agentUpdateExplain bool
)

var agentUpdateCmd = &cobra.Command{
	Use:     "agent-update",
	GroupID: "maintenance",
	Short:   "Update AI tools",
	Long:    `Update all or selected AI tools (Claude Code, OpenAI Codex, etc.) parallelly.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := update.Run(update.Options{DryRun: agentUpdateDryRun, Explain: agentUpdateExplain}); err != nil {
			fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	agentUpdateCmd.Flags().BoolVar(&agentUpdateDryRun, "dry-run", false, "show planned updates without executing installs")
	agentUpdateCmd.Flags().BoolVar(&agentUpdateExplain, "explain", false, "print manager/path/command details before execution")
	rootCmd.AddCommand(agentUpdateCmd)
}
