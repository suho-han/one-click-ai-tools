package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/suho-han/one-click-tools/internal/usage"
)

var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Show tool usage report",
	Long: `Show tool usage report for configured AI developer tools.

To properly fetch usage, ensure you are authenticated:
  - Gemini:  Run 'gemini auth' to log in via browser
  - Claude:  Run 'claude auth login' to log in via browser
  - Copilot: Configure your token via 'oct config'
  - Codex:   Automatically reads from local session logs`,
	Run: func(cmd *cobra.Command, args []string) {
		results, err := usage.GetUsage()
		if err != nil {
			fmt.Printf("Error fetching usage: %v\n", err)
			return
		}

		jsonMode, _ := cmd.Flags().GetBool("json")
		if jsonMode {
			usage.PrintJSON(results)
		} else {
			usage.PrintTable(results)
			fmt.Println("\nTip: Run 'oct usage --help' for authentication instructions.")
		}
	},
}

func init() {
	rootCmd.AddCommand(usageCmd)
	usageCmd.Flags().Bool("json", false, "Output in JSON format")
}
