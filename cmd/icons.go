package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/suho-han/one-click-tools/internal/ui"
	"github.com/suho-han/one-click-tools/internal/update"
)

var iconsCmd = &cobra.Command{
	Use:   "icons",
	Short: "Preview tool icons rendering in current terminal",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Icon Rendering Comparison (6x6 Target):")
		for _, t := range update.Tools {
			if t.LobeIcon == "" {
				continue
			}
			ui.PrintIconComparison(t.LobeIcon, 6)
		}
	},
}

func init() {
	rootCmd.AddCommand(iconsCmd)
}
