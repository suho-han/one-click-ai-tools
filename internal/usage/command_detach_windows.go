//go:build windows

package usage

import (
	"os/exec"
)

// detachCommandTTY is a no-op on Windows: there is no controlling-terminal
// concept to drop, and the browser-launch concern only exists for CLIs that
// gate their login fallback on one (agy on macOS/Linux).
func detachCommandTTY(cmd *exec.Cmd) {
	_ = cmd
}
