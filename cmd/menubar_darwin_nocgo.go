//go:build darwin && !cgo

package cmd

import (
	"errors"
	"io"
	"time"
)

func runMenubar() error {
	return errors.New("menubar requires cgo-enabled darwin build")
}

func startMenubarDetached() error {
	return errors.New("menubar requires cgo-enabled darwin build")
}

func stopMenubarInstances() (menubarStopResult, error) {
	return menubarStopResult{}, errors.New("menubar requires cgo-enabled darwin build")
}

func findMenubarInstancePIDs() []int { return nil }

func waitForMenubarExit(pids []string, timeout time.Duration) {}

func enableMenubarDaemon(out io.Writer) error {
	return errors.New("menubar daemon management requires cgo-enabled darwin build")
}

func disableMenubarDaemon(out io.Writer) error {
	return errors.New("menubar daemon management requires cgo-enabled darwin build")
}

func menubarDaemonSummary() string { return "" }
