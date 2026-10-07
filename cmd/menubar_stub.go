//go:build !darwin

package cmd

import (
	"fmt"
	"io"
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

func enableMenubarDaemon(out io.Writer) error {
	return fmt.Errorf("menubar daemon management is currently supported only on macOS")
}

func disableMenubarDaemon(out io.Writer) error {
	return fmt.Errorf("menubar daemon management is currently supported only on macOS")
}

func menubarDaemonSummary() string { return "" }
