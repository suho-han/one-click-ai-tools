//go:build !darwin

package cmd

import (
	"fmt"
	"time"
)

func runMenubar() error {
	return fmt.Errorf("menubar is currently supported only on macOS")
}

func startMenubarDetached() error {
	return fmt.Errorf("menubar daemon is currently supported only on macOS")
}

func stopMenubarInstances() (menubarStopResult, error) {
	return menubarStopResult{}, fmt.Errorf("menubar stop is currently supported only on macOS")
}

func findMenubarInstancePIDs() []int { return nil }

func waitForMenubarExit(pids []string, timeout time.Duration) {}
