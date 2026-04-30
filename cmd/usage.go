package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/suho-han/one-click-tools/internal/usage"
)

var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Show tool usage report",
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
		}
	},
}

func init() {
	rootCmd.AddCommand(usageCmd)
	usageCmd.Flags().Bool("json", false, "Output in JSON format")
}
