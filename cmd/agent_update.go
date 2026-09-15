package cmd

import (
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
	Short:   "🔄 Update AI tools",
	Long:    `Update all or selected AI tools (Claude Code, OpenAI Codex, etc.).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return update.Run(cmd.Context(), update.Options{DryRun: agentUpdateDryRun, Explain: agentUpdateExplain})
	},
}

func init() {
	agentUpdateCmd.Flags().BoolVar(&agentUpdateDryRun, "dry-run", false, "show planned updates without executing installs")
	agentUpdateCmd.Flags().BoolVar(&agentUpdateExplain, "explain", false, "print manager/path/command details before execution")
	rootCmd.AddCommand(agentUpdateCmd)
}
