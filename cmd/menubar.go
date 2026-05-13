package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var menubarCmd = &cobra.Command{
	Use:     "menubar",
	GroupID: "core",
	Short:   "Run macOS menu bar app (status item)",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runMenubar(); err != nil {
			fmt.Printf("menubar failed: %v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(menubarCmd)
}
