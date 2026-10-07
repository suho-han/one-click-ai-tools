//go:build darwin && cgo

package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/suho-han/one-click-ai-tools/internal/schedule"
)

// Indirection seams so tests can stub launchd effects without touching the
// user's LaunchAgents.
var (
	enableLaunchItemCmd  = schedule.EnableLaunchItem
	installLaunchItemCmd = schedule.InstallLaunchItem
	disableLaunchItemCmd = schedule.DisableLaunchItem
	launchItemStatusCmd  = schedule.LaunchItemStatus
)

// Each menubar track owns its own LaunchAgent label, mirroring the per-track
// helper binary names: registering the daemon from one track must never
// rewrite or unload the other track's login item.
const (
	stableMenubarDaemonLabel = "com.oct.menubar"
	betaMenubarDaemonLabel   = "com.oct-beta.menubar"
)

func menubarDaemonLabel(track string) string {
	if track == menubarTrackBeta {
		return betaMenubarDaemonLabel
	}
	return stableMenubarDaemonLabel
}

// enableMenubarDaemon installs the login item for the running track. When an
// instance is already up, the plist is written but not loaded: loading fires
// RunAtLoad and would put a second status item next to the live one.
func enableMenubarDaemon(out io.Writer) error {
	track := currentMenubarTrack()
	label := menubarDaemonLabel(track)

	octPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve oct path: %w", err)
	}
	workingDir, _ := os.Getwd()
	helperPath, _ := resolveMenubarHelperPath(menubarEnvironmentMap(), octPath, workingDir)
	if helperPath == "" {
		return errors.New("no installed menubar helper found; build and install it first with 'oct menubar install-helper' (see 'oct menubar doctor')")
	}
	// launchd sessions have no user shell PATH and the helper locates oct by
	// env (OCT_MENUBAR_OCT_PATH first, then PATH), so both are pinned at
	// enable time. Re-run 'daemon enable' after moving either binary.
	item := schedule.LaunchItem{
		Label:            label,
		ProgramArguments: []string{helperPath},
		Environment: map[string]string{
			"OCT_MENUBAR_OCT_PATH": octPath,
			"PATH":                 menubarDaemonPathEnv(helperPath),
		},
		LogPath: menubarDaemonLogPath(),
	}

	if len(findMenubarInstancePIDsCmd()) > 0 {
		if err := installLaunchItemCmd(item); err != nil {
			return err
		}
		fmt.Fprintf(out, "menubar daemon enabled (%s): already running, so it will also start at the next login\n", label)
		return nil
	}
	if err := enableLaunchItemCmd(item); err != nil {
		return err
	}
	fmt.Fprintf(out, "menubar daemon enabled (%s): starts at login, running now\n", label)
	return nil
}

func disableMenubarDaemon(out io.Writer) error {
	label := menubarDaemonLabel(currentMenubarTrack())
	if err := disableLaunchItemCmd(label); err != nil {
		return err
	}
	fmt.Fprintf(out, "menubar daemon disabled (%s). An instance the daemon started stops with it; one started manually keeps running.\n", label)
	return nil
}

// menubarDaemonSummary renders the status for the daemon command and the
// doctor report. The plist on disk decides whether the daemon starts at
// login; the launchd load state is a detail because an installed-but-
// unloaded item still comes back at the next login. Empty on platforms
// without menubar support.
func menubarDaemonSummary() string {
	label := menubarDaemonLabel(currentMenubarTrack())
	state, err := launchItemStatusCmd(label)
	if err != nil {
		return fmt.Sprintf("unknown (%v)", err)
	}
	switch {
	case state.Installed && state.Loaded:
		return fmt.Sprintf("enabled (%s, loaded)", label)
	case state.Installed:
		return fmt.Sprintf("enabled (%s, starts at next login)", label)
	default:
		return "disabled"
	}
}

func menubarDaemonPathEnv(helperPath string) string {
	pathEnv := strings.TrimSpace(os.Getenv("PATH"))
	if pathEnv == "" {
		return filepath.Dir(helperPath)
	}
	return filepath.Dir(helperPath) + string(os.PathListSeparator) + pathEnv
}

func menubarDaemonLogPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "oct-menubar.log")
	}
	return filepath.Join(home, ".oct", "logs", "menubar.log")
}
