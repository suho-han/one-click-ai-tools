//go:build darwin

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/getlantern/systray"
)

func runMenubar() error {
	systray.Run(onMenubarReady, func() {})
	return nil
}

func startMenubarDetached() error {
	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer devNull.Close()

	cmd := exec.Command(execPath, "menubar")
	cmd.Stdout = devNull
	cmd.Stderr = devNull
	cmd.Stdin = devNull
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}

func onMenubarReady() {
	systray.SetTitle("oct")
	systray.SetTooltip("one-click-tools")

	mUsage := systray.AddMenuItem("Usage (once)", "Run `oct usage` in a new Terminal")
	mMonitor := systray.AddMenuItem("Monitor (once)", "Run `oct monitor --once` in a new Terminal")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Quit menubar")

	go func() {
		for {
			select {
			case <-mUsage.ClickedCh:
				_ = runInTerminal("oct usage")
			case <-mMonitor.ClickedCh:
				_ = runInTerminal("oct monitor --once")
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func runInTerminal(command string) error {
	script := fmt.Sprintf(`tell application "Terminal"
	activate
	do script "%s"
end tell`, command)
	return exec.Command("osascript", "-e", script).Run()
}
