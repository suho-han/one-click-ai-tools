//go:build !darwin

package cmd

import "fmt"

func runMenubar() error {
	return fmt.Errorf("menubar is currently supported only on macOS")
}

func startMenubarDetached() error {
	return fmt.Errorf("menubar daemon is currently supported only on macOS")
}
