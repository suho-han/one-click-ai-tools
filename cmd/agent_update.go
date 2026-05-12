package cmd

import (
	"fmt"
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
			fmt.Printf("Update failed: %v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(agentUpdateCmd)
}
