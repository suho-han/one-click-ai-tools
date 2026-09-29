//go:build unix

package usage

import (
	"os/exec"
	"syscall"
)

// detachCommandTTY isolates a subprocess from the caller's controlling
// terminal: the child runs in a new session (Setsid) with no controlling
// terminal, and stdin stays at exec.Cmd's /dev/null default. Interactive
// CLIs use the controlling terminal as the signal that a browser-based
// login fallback may start — specifically `agy --print /usage`, which on an
// expired OAuth token would otherwise open a browser login window from a
// background menubar refresh. Detached, agy's print mode refuses the
// interactive login ("no controlling terminal") and exits with an auth
// error instead.
func detachCommandTTY(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
}
