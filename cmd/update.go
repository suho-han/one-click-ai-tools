package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:     "update",
	GroupID: "maintenance",
	Short:   "Update oct package",
	Long:    `Update oct (one-click-tools) itself to the latest version.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runSelfUpdate(); err != nil {
			fmt.Printf("oct update failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("oct updated successfully.")
	},
}

func runSelfUpdate() error {
	if runtime.GOOS == "darwin" {
		if _, err := exec.LookPath("brew"); err == nil {
			if out, err := exec.Command("brew", "list", "one-click-tools").CombinedOutput(); err == nil && len(out) > 0 {
				cmd := exec.Command("brew", "upgrade", "one-click-tools")
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				return cmd.Run()
			}
		}
	}

	cmd := exec.Command("go", "install", "github.com/suho-han/one-click-ai-tools@latest")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
