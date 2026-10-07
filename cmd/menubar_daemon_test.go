//go:build darwin && cgo

package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/suho-han/one-click-ai-tools/internal/schedule"
)

// stubDaemonSeams points the launchd-effect seams at recording stubs and
// restores them on cleanup. It returns the recorded launch items per call.
func stubDaemonSeams(t *testing.T) (enabled, installed *[]schedule.LaunchItem, disabled *[]string) {
	t.Helper()
	origEnable := enableLaunchItemCmd
	origInstall := installLaunchItemCmd
	origDisable := disableLaunchItemCmd
	origStatus := launchItemStatusCmd
	origPIDs := findMenubarInstancePIDsCmd
	t.Cleanup(func() {
		enableLaunchItemCmd = origEnable
		installLaunchItemCmd = origInstall
		disableLaunchItemCmd = origDisable
		launchItemStatusCmd = origStatus
		findMenubarInstancePIDsCmd = origPIDs
	})

	enabledItems := &[]schedule.LaunchItem{}
	installedItems := &[]schedule.LaunchItem{}
	disabledLabels := &[]string{}
	enableLaunchItemCmd = func(item schedule.LaunchItem) error {
		*enabledItems = append(*enabledItems, item)
		return nil
	}
	installLaunchItemCmd = func(item schedule.LaunchItem) error {
		*installedItems = append(*installedItems, item)
		return nil
	}
	disableLaunchItemCmd = func(label string) error {
		*disabledLabels = append(*disabledLabels, label)
		return nil
	}
	launchItemStatusCmd = func(string) (schedule.LaunchItemState, error) {
		return schedule.LaunchItemState{}, nil
	}
	findMenubarInstancePIDsCmd = func() []int { return nil }
	return enabledItems, installedItems, disabledLabels
}

func TestMenubarDaemonLabelPerTrack(t *testing.T) {
	stubMenubarTrack(t, "0.1.6")
	if got := menubarDaemonLabel(currentMenubarTrack()); got != stableMenubarDaemonLabel {
		t.Fatalf("stable label = %q, want %q", got, stableMenubarDaemonLabel)
	}

	stubMenubarTrack(t, "0.1.6-beta.1")
	if got := menubarDaemonLabel(currentMenubarTrack()); got != betaMenubarDaemonLabel {
		t.Fatalf("beta label = %q, want %q", got, betaMenubarDaemonLabel)
	}
}

func TestEnableMenubarDaemonStartsHelperWhenIdle(t *testing.T) {
	helperPath := pointMenubarHelperAtFake(t)
	stubMenubarTrack(t, "0.1.6")
	enabled, installed, _ := stubDaemonSeams(t)

	var out bytes.Buffer
	if err := enableMenubarDaemon(&out); err != nil {
		t.Fatalf("enableMenubarDaemon: %v", err)
	}

	if len(*enabled) != 1 {
		t.Fatalf("EnableLaunchItem calls = %d, want 1", len(*enabled))
	}
	if len(*installed) != 0 {
		t.Fatalf("idle enable must load (Enable), not only install; got %d Install calls", len(*installed))
	}
	item := (*enabled)[0]
	if item.Label != stableMenubarDaemonLabel {
		t.Fatalf("label = %q, want %q", item.Label, stableMenubarDaemonLabel)
	}
	if len(item.ProgramArguments) != 1 || item.ProgramArguments[0] != helperPath {
		t.Fatalf("program arguments = %v, want [%s]", item.ProgramArguments, helperPath)
	}
	if item.Environment["OCT_MENUBAR_OCT_PATH"] == "" {
		t.Fatal("OCT_MENUBAR_OCT_PATH not pinned to the running oct")
	}
	if !strings.Contains(item.Environment["PATH"], filepath.Dir(helperPath)) {
		t.Fatalf("PATH env %q missing helper dir %s", item.Environment["PATH"], filepath.Dir(helperPath))
	}
	if !strings.Contains(out.String(), "starts at login, running now") {
		t.Fatalf("output = %q, want the running-now confirmation", out.String())
	}
}

func TestEnableMenubarDaemonAlreadyRunningWritesPlistOnly(t *testing.T) {
	pointMenubarHelperAtFake(t)
	stubMenubarTrack(t, "0.1.6")
	enabled, installed, _ := stubDaemonSeams(t)
	findMenubarInstancePIDsCmd = func() []int { return []int{4242} }

	var out bytes.Buffer
	if err := enableMenubarDaemon(&out); err != nil {
		t.Fatalf("enableMenubarDaemon: %v", err)
	}

	if len(*installed) != 1 {
		t.Fatalf("InstallLaunchItem calls = %d, want 1 (loading would add a second status item)", len(*installed))
	}
	if len(*enabled) != 0 {
		t.Fatalf("EnableLaunchItem calls = %d, want 0 while an instance is running", len(*enabled))
	}
	if !strings.Contains(out.String(), "will also start at the next login") {
		t.Fatalf("output = %q, want the next-login note", out.String())
	}
}

func TestEnableMenubarDaemonRequiresInstalledHelper(t *testing.T) {
	// The beta track skips source-tree helper search, so an empty HOME and
	// PATH make resolution fail deterministically even inside the repo.
	stubMenubarTrack(t, "0.1.6-beta.1")
	stubDaemonSeams(t)
	t.Setenv("OCT_MENUBAR_HELPER_PATH", "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", "")

	var out bytes.Buffer
	err := enableMenubarDaemon(&out)
	if err == nil || !strings.Contains(err.Error(), "oct menubar install-helper") {
		t.Fatalf("error = %v, want the install-helper guidance", err)
	}
}

func TestDisableMenubarDaemon(t *testing.T) {
	stubMenubarTrack(t, "0.1.6")
	_, _, disabled := stubDaemonSeams(t)

	var out bytes.Buffer
	if err := disableMenubarDaemon(&out); err != nil {
		t.Fatalf("disableMenubarDaemon: %v", err)
	}
	if len(*disabled) != 1 || (*disabled)[0] != stableMenubarDaemonLabel {
		t.Fatalf("disabled labels = %v, want [%s]", *disabled, stableMenubarDaemonLabel)
	}
	if !strings.Contains(out.String(), "keeps running") {
		t.Fatalf("output = %q, want the manual-instance note", out.String())
	}
}

func TestMenubarDaemonSummary(t *testing.T) {
	_, _, _ = stubDaemonSeams(t)
	stubMenubarTrack(t, "0.1.6")
	cases := []struct {
		name  string
		state schedule.LaunchItemState
		want  string
	}{
		{"installed and loaded", schedule.LaunchItemState{Installed: true, Loaded: true}, "enabled (com.oct.menubar, loaded)"},
		{"installed but unloaded", schedule.LaunchItemState{Installed: true}, "enabled (com.oct.menubar, starts at next login)"},
		{"not installed", schedule.LaunchItemState{}, "disabled"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			launchItemStatusCmd = func(string) (schedule.LaunchItemState, error) { return tc.state, nil }
			if got := menubarDaemonSummary(); got != tc.want {
				t.Fatalf("menubarDaemonSummary() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMenubarDaemonSummarySurfacesErrors(t *testing.T) {
	_, _, _ = stubDaemonSeams(t)
	launchItemStatusCmd = func(string) (schedule.LaunchItemState, error) {
		return schedule.LaunchItemState{}, os.ErrPermission
	}
	if got := menubarDaemonSummary(); !strings.HasPrefix(got, "unknown (") {
		t.Fatalf("menubarDaemonSummary() = %q, want an unknown(...) report", got)
	}
}

func TestMenubarDoctorIncludesDaemon(t *testing.T) {
	pointMenubarHelperAtFake(t)
	stubMenubarHelperVersion(t, "0.1.6")
	stubMenubarTrack(t, "0.1.6")
	_, _, _ = stubDaemonSeams(t)
	launchItemStatusCmd = func(string) (schedule.LaunchItemState, error) {
		return schedule.LaunchItemState{Installed: true, Loaded: true}, nil
	}

	report, err := collectMenubarDoctorReport()
	if err != nil {
		t.Fatalf("collectMenubarDoctorReport: %v", err)
	}
	if !strings.Contains(report.Daemon, "enabled") {
		t.Fatalf("doctor daemon = %q, want an enabled status", report.Daemon)
	}
}

func TestMenubarDaemonCommandRendersStatus(t *testing.T) {
	_, _, _ = stubDaemonSeams(t)
	stubMenubarTrack(t, "0.1.6")
	launchItemStatusCmd = func(string) (schedule.LaunchItemState, error) {
		return schedule.LaunchItemState{Installed: true}, nil
	}

	var out bytes.Buffer
	menubarDaemonCmd.SetOut(&out)
	if err := menubarDaemonCmd.RunE(menubarDaemonCmd, nil); err != nil {
		t.Fatalf("daemon RunE: %v", err)
	}
	if !strings.Contains(out.String(), "enabled (com.oct.menubar, starts at next login)") {
		t.Fatalf("output = %q, want the status line", out.String())
	}
}
