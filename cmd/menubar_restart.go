package cmd

import (
	"fmt"
	"io"
	"runtime"
	"strings"
	"time"
)

// restartMenubarAfterUpdateCmd is a seam so tests can stub the post-update hook.
var restartMenubarAfterUpdateCmd = restartMenubarAfterUpdate

// restartMenubarCommand stops this track's menubar instances and starts a
// fresh one. It never prompts.
func restartMenubarCommand(out io.Writer) error {
	// Checked before touching the platform stubs so the non-macOS error names
	// restart instead of leaking the stop command's wording.
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("menubar is currently supported only on macOS")
	}
	result, err := stopMenubarInstances()
	if err != nil {
		return err
	}
	if len(result.PIDs) > 0 {
		waitForMenubarExit(result.PIDs, 3*time.Second)
	}
	if menubarDaemonInstalled() {
		if err := enableMenubarDaemon(out); err != nil {
			return err
		}
	} else if err := startMenubarDetachedCommand(); err != nil {
		return err
	}
	if len(result.PIDs) > 0 {
		fmt.Fprintf(out, "menubar restarted (stopped: %s)\n", strings.Join(result.PIDs, ", "))
	} else {
		fmt.Fprintln(out, "menubar restarted")
	}
	return nil
}

// restartMenubarAfterUpdate restarts the menubar only if one is running or
// its daemon is installed.
func restartMenubarAfterUpdate(out io.Writer) (bool, error) {
	if len(findMenubarInstancePIDs()) == 0 && !menubarDaemonInstalled() {
		return false, nil
	}
	if err := restartMenubarCommand(io.Discard); err != nil {
		return false, err
	}
	return true, nil
}
