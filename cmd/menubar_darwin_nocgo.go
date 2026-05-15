//go:build darwin && !cgo

package cmd

import "errors"

func runMenubar() error {
	return errors.New("menubar requires cgo-enabled darwin build")
}

func startMenubarDetached() error {
	return errors.New("menubar requires cgo-enabled darwin build")
}
