//go:build darwin && cgo

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

func launchSwiftMenubarHelper(detached bool) (bool, error) {
	execPath, err := os.Executable()
	if err != nil {
		return false, err
	}
	workingDir, _ := os.Getwd()
	launch, searched := resolveMenubarHelperLaunch(menubarEnvironmentMap(), execPath, workingDir)
	if launch.Executable == "" {
		return false, nil
	}
	readyFile := ""
	if !detached {
		readyFile, err = newMenubarReadyFilePath()
		if err != nil {
			return true, err
		}
		defer os.Remove(readyFile)
	}

	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return true, err
	}
	defer devNull.Close()

	cmd := exec.Command(launch.Executable, launch.Args...)
	cmd.Env = menubarLaunchEnvironment(os.Environ(), execPath, readyFile)
	cmd.Stdout = devNull
	cmd.Stderr = devNull
	cmd.Stdin = devNull
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return true, fmt.Errorf("swift menubar helper launch failed (%s): %w", strings.Join(searched, ", "), err)
	}
	if detached {
		return true, nil
	}
	processDone := make(chan error, 1)
	go func() {
		processDone <- cmd.Wait()
	}()
	if err := waitForMenubarReady(readyFile, processDone, 10*time.Second); err != nil {
		return true, err
	}
	fmt.Println("oct menubar ready")
	return true, nil
}
