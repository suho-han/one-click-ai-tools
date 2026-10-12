//go:build darwin

package schedule

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func renderTestLaunchItemPlist(t *testing.T, item LaunchItem) string {
	t.Helper()
	var buf strings.Builder
	if err := renderLaunchItemPlist(&buf, item); err != nil {
		t.Fatalf("renderLaunchItemPlist: %v", err)
	}
	return buf.String()
}

func TestRenderLaunchItemPlist(t *testing.T) {
	plist := renderTestLaunchItemPlist(t, LaunchItem{
		Label:            "com.oct.menubar",
		ProgramArguments: []string{"/Users/me/.local/bin/OctMenubarApp"},
		Environment: map[string]string{
			"PATH":                 "/Users/me/.local/bin:/usr/bin:/bin",
			"OCT_MENUBAR_OCT_PATH": "/Users/me/.local/bin/oct",
		},
		LogPath: "/Users/me/.oct/logs/menubar.log",
	})

	for _, want := range []string{
		"<string>com.oct.menubar</string>",
		"<string>/Users/me/.local/bin/OctMenubarApp</string>",
		"<key>EnvironmentVariables</key>",
		"<key>OCT_MENUBAR_OCT_PATH</key>",
		"<string>/Users/me/.local/bin/oct</string>",
		"<key>PATH</key>",
		"<key>RunAtLoad</key>",
		"<true/>",
		"<string>/Users/me/.oct/logs/menubar.log</string>",
	} {
		if !strings.Contains(plist, want) {
			t.Fatalf("plist missing %q:\n%s", want, plist)
		}
	}
	for _, unwanted := range []string{"StartInterval", "StartCalendarInterval", "KeepAlive"} {
		if strings.Contains(plist, unwanted) {
			t.Fatalf("login item plist must not contain %q:\n%s", unwanted, plist)
		}
	}
	// Environment renders sorted by key so identical items stay
	// byte-identical across re-enables.
	if strings.Index(plist, "<key>OCT_MENUBAR_OCT_PATH</key>") > strings.Index(plist, "<key>PATH</key>") {
		t.Fatalf("environment keys not sorted:\n%s", plist)
	}
}

func TestRenderLaunchItemPlistEscapesXML(t *testing.T) {
	plist := renderTestLaunchItemPlist(t, LaunchItem{
		Label:            "com.oct.menubar",
		ProgramArguments: []string{`/Users/me/.local/bin/App&<>'"`},
	})
	if !strings.Contains(plist, "App&amp;&lt;&gt;&#39;&#34;") {
		t.Fatalf("XML-special characters not escaped:\n%s", plist)
	}
}

// stubLaunchdForAutostart points homeDirPath at a temp dir and captures
// launchctl invocations, restoring both on cleanup.
func stubLaunchdForAutostart(t *testing.T) (home string, launchctlCalls *[]string) {
	t.Helper()
	origHome := homeDirPath
	origRun := launchctlRun
	origList := launchctlList
	t.Cleanup(func() {
		homeDirPath = origHome
		launchctlRun = origRun
		launchctlList = origList
	})
	home = t.TempDir()
	homeDirPath = func() (string, error) { return home, nil }
	calls := &[]string{}
	launchctlRun = func(args ...string) (string, error) {
		*calls = append(*calls, strings.Join(args, " "))
		return "", nil
	}
	launchctlList = func(string) (bool, error) { return false, nil }
	return home, calls
}

func TestEnableLaunchItemWritesPlistAndLoads(t *testing.T) {
	home, calls := stubLaunchdForAutostart(t)

	item := LaunchItem{
		Label:            "com.oct.menubar",
		ProgramArguments: []string{"/Users/me/.local/bin/OctMenubarApp"},
		Environment:      map[string]string{"OCT_MENUBAR_OCT_PATH": "/usr/local/bin/oct"},
		LogPath:          filepath.Join(home, ".oct", "logs", "menubar.log"),
	}
	if err := EnableLaunchItem(item); err != nil {
		t.Fatalf("EnableLaunchItem: %v", err)
	}

	plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.oct.menubar.plist")
	data, err := os.ReadFile(plistPath)
	if err != nil {
		t.Fatalf("plist not written: %v", err)
	}
	if !strings.Contains(string(data), "<key>RunAtLoad</key>") {
		t.Fatalf("plist missing RunAtLoad:\n%s", data)
	}
	if _, err := os.Stat(filepath.Join(home, ".oct", "logs")); err != nil {
		t.Fatalf("log dir not created: %v", err)
	}
	wantCalls := []string{"unload " + plistPath, "load " + plistPath}
	if len(*calls) != len(wantCalls) || (*calls)[0] != wantCalls[0] || (*calls)[1] != wantCalls[1] {
		t.Fatalf("launchctl calls = %v, want %v", *calls, wantCalls)
	}
}

func TestInstallLaunchItemWritesPlistWithoutLaunchctl(t *testing.T) {
	home, calls := stubLaunchdForAutostart(t)

	err := InstallLaunchItem(LaunchItem{
		Label:            "com.oct.menubar",
		ProgramArguments: []string{"/Users/me/.local/bin/OctMenubarApp"},
	})
	if err != nil {
		t.Fatalf("InstallLaunchItem: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "Library", "LaunchAgents", "com.oct.menubar.plist")); err != nil {
		t.Fatalf("plist not written: %v", err)
	}
	if len(*calls) != 0 {
		t.Fatalf("install must not touch launchctl, got %v", *calls)
	}
}

func TestEnableLaunchItemLoadFailureKeepsPlist(t *testing.T) {
	home, _ := stubLaunchdForAutostart(t)
	launchctlRun = func(args ...string) (string, error) {
		if args[0] == "load" {
			return "Load failed: 5: Input/output error\n", errors.New("exit status 5")
		}
		return "", nil
	}

	err := EnableLaunchItem(LaunchItem{
		Label:            "com.oct.menubar",
		ProgramArguments: []string{"/Users/me/.local/bin/OctMenubarApp"},
	})
	if err == nil || !strings.Contains(err.Error(), "launchctl load failed") {
		t.Fatalf("EnableLaunchItem error = %v, want a launchctl load failure", err)
	}
	if _, err := os.Stat(filepath.Join(home, "Library", "LaunchAgents", "com.oct.menubar.plist")); err != nil {
		t.Fatalf("plist must survive a failed load so the item still starts at next login: %v", err)
	}
}

func TestDisableLaunchItemRemovesPlistAndUnloads(t *testing.T) {
	home, calls := stubLaunchdForAutostart(t)
	plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.oct.menubar.plist")
	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(plistPath, []byte("plist"), 0o644); err != nil {
		t.Fatalf("write plist: %v", err)
	}

	if err := DisableLaunchItem("com.oct.menubar"); err != nil {
		t.Fatalf("DisableLaunchItem: %v", err)
	}
	if _, err := os.Stat(plistPath); !os.IsNotExist(err) {
		t.Fatalf("plist still present after disable: %v", err)
	}
	if len(*calls) != 1 || (*calls)[0] != "unload "+plistPath {
		t.Fatalf("launchctl calls = %v, want [unload %s]", *calls, plistPath)
	}

	// Disabling an item that was never installed is a no-op, not an error.
	if err := DisableLaunchItem("com.oct.menubar"); err != nil {
		t.Fatalf("DisableLaunchItem without plist: %v", err)
	}
}

func TestLaunchItemStatus(t *testing.T) {
	origList := launchctlList
	t.Cleanup(func() { launchctlList = origList })
	home, _ := stubLaunchdForAutostart(t)
	label := "com.oct.menubar"

	// Missing plist: disabled, launchctl never consulted.
	listed := 0
	launchctlList = func(string) (bool, error) { listed++; return true, nil }
	state, err := LaunchItemStatus(label)
	if err != nil {
		t.Fatalf("LaunchItemStatus without plist: %v", err)
	}
	if state.Installed || state.Loaded {
		t.Fatalf("state = %+v, want disabled", state)
	}
	if listed != 0 {
		t.Fatal("launchctl consulted even though the plist is absent")
	}

	plistPath := launchItemPath(home, label)
	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(plistPath, []byte("plist"), 0o644); err != nil {
		t.Fatalf("write plist: %v", err)
	}

	launchctlList = func(string) (bool, error) { return true, nil }
	state, err = LaunchItemStatus(label)
	if err != nil {
		t.Fatalf("LaunchItemStatus: %v", err)
	}
	if !state.Installed || !state.Loaded {
		t.Fatalf("state = %+v, want installed+loaded", state)
	}

	launchctlList = func(string) (bool, error) { return false, nil }
	state, err = LaunchItemStatus(label)
	if err != nil {
		t.Fatalf("LaunchItemStatus: %v", err)
	}
	if !state.Installed || state.Loaded {
		t.Fatalf("state = %+v, want installed but not loaded", state)
	}
}
