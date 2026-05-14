package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/suho-han/one-click-tools/internal/update"
)

var agentUpdateCmd = &cobra.Command{
	Use:     "agent-update",
	GroupID: "maintenance",
	Short: "Update AI tools",
	Long:  `Update all or selected AI tools (Claude Code, OpenAI Codex, etc.) parallelly.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := update.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(agentUpdateCmd)
}
