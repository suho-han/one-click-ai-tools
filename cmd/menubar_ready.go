package cmd

import (
	"fmt"
	"os"
	"time"
)

const menubarReadyEnvKey = "OCT_MENUBAR_READY_FILE"

func newMenubarReadyFilePath() (string, error) {
	file, err := os.CreateTemp("", "oct-menubar-ready-*")
	if err != nil {
		return "", err
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		return "", err
	}
	if err := os.Remove(path); err != nil {
		return "", err
	}
	return path, nil
}

func menubarLaunchEnvironment(base []string, execPath string, readyFile string) []string {
	env := append([]string{}, base...)
	env = append(env, "OCT_MENUBAR_OCT_PATH="+execPath)
	if readyFile != "" {
		env = append(env, menubarReadyEnvKey+"="+readyFile)
	}
	return env
}

func waitForMenubarReady(readyFile string, processDone <-chan error, timeout time.Duration) error {
	if readyFile == "" {
		return nil
	}
	if _, err := os.Stat(readyFile); err == nil {
		return nil
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case err := <-processDone:
			if err != nil {
				return fmt.Errorf("swift menubar helper exited before ready: %w", err)
			}
			return fmt.Errorf("swift menubar helper exited before ready")
		case <-ticker.C:
			if _, err := os.Stat(readyFile); err == nil {
				return nil
			}
		case <-timer.C:
			return fmt.Errorf("timed out waiting for swift menubar helper to show")
		}
	}
}
