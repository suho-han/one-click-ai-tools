//go:build darwin && !cgo

package cmd

import (
	"errors"
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
