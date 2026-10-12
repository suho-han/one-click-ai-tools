package cmd

import (
	"github.com/spf13/cobra"
	"github.com/suho-han/one-click-ai-tools/internal/update"
)

var (
	agentUpdateDryRun       bool
	agentUpdateExplain      bool
	agentUpdateJSON         bool
	agentUpdateSkipMissing  bool
	agentUpdateInstallMssng bool
	agentUpdateOnly         []string
	agentUpdateCheck        bool
)

var agentUpdateCmd = &cobra.Command{
	Use:     "agent-update",
	GroupID: "maintenance",
	Short:   "🔄 Update AI tools",
	Long:    `Update all or selected AI tools (Claude Code, OpenAI Codex, etc.).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := update.Options{
			DryRun:         agentUpdateDryRun,
			Explain:        agentUpdateExplain,
			JSON:           agentUpdateJSON,
			SkipMissing:    agentUpdateSkipMissing,
			InstallMissing: agentUpdateInstallMssng,
			Only:           agentUpdateOnly,
		}
		if agentUpdateCheck {
			return update.RunVersionCheck(cmd.Context(), opts)
		}
		return update.Run(cmd.Context(), opts)
	},
}

func init() {
	agentUpdateCmd.Flags().BoolVar(&agentUpdateDryRun, "dry-run", false, "show planned updates without executing installs")
	agentUpdateCmd.Flags().BoolVar(&agentUpdateExplain, "explain", false, "print manager/path/command details before execution")
	agentUpdateCmd.Flags().BoolVar(&agentUpdateJSON, "json", false, "emit newline-delimited JSON events instead of human output (never prompts; missing tools are skipped unless --install-missing)")
	agentUpdateCmd.Flags().BoolVar(&agentUpdateSkipMissing, "skip-missing", false, "skip providers that are not installed instead of prompting to install them")
	agentUpdateCmd.Flags().BoolVar(&agentUpdateInstallMssng, "install-missing", false, "install providers that are not installed through their default manager instead of prompting")
	agentUpdateCmd.Flags().StringArrayVar(&agentUpdateOnly, "only", nil, "update only the named tool(s); accepts aliases like gemini; repeatable or comma-separated")
	agentUpdateCmd.Flags().BoolVar(&agentUpdateCheck, "check", false, "check installed versions against the latest known versions without updating")
	rootCmd.AddCommand(agentUpdateCmd)
}
