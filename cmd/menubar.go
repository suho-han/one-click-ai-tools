package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var menubarDaemon bool
var menubarLegacy bool

var menubarCmd = &cobra.Command{
	Use:     "menubar",
	GroupID: "core",
	Short:   "Run macOS menu bar app (status item)",
	Run: func(cmd *cobra.Command, args []string) {
		if menubarDaemon {
			if err := startMenubarDetached(); err != nil {
				fmt.Printf("menubar daemon start failed: %v\n", err)
				return
			}
			fmt.Println("menubar daemon started")
			return
		}

		if err := runMenubar(); err != nil {
			fmt.Printf("menubar failed: %v\n", err)
		}
	},
}

func init() {
	menubarCmd.Flags().BoolVar(&menubarDaemon, "daemon", false, "start menubar in background and return")
	menubarCmd.Flags().BoolVar(&menubarLegacy, "legacy", false, "force legacy systray/NSMenu menubar instead of Swift helper")
	rootCmd.AddCommand(menubarCmd)
}
