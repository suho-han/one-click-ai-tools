package update

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// stubCheckManagersOnPath shadows the real package managers for RunVersionCheck.
// The npm stub classifies MiniMax as npm-managed at 1.0.0 (`list`), answers
// `outdated` with outdatedShell (raw script lines; empty means "fail quietly",
// which is also what a missing npm does), and answers `view` with viewVersion.
func stubCheckManagersOnPath(t *testing.T, outdatedShell, viewVersion string) {
	t.Helper()
	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir stub bin: %v", err)
	}
	outdatedLines := outdatedShell
	if outdatedLines == "" {
		outdatedLines = "exit 1"
	}
	writeStub := func(name, body string) {
		t.Helper()
		path := filepath.Join(binDir, executableName(name))
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
			t.Fatalf("write stub %s: %v", name, err)
		}
	}
	writeStub("npm", "case \"$1\" in\n"+
		"list)\necho 'mmx-cli@1.0.0'\n;;\n"+
		"outdated)\n"+outdatedLines+"\n;;\n"+
		"view)\necho '"+viewVersion+"'\n;;\n"+
		"esac\nexit 0\n")
	for _, manager := range []string{"brew", "pnpm", "yarn"} {
		writeStub(manager, "exit 1\n")
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// stubAgysOnPath shadows any real `agy` on the machine with a stub whose
// --version is fixed and whose `update` is a no-op, so alias-resolution runs
// through the AntigravityUpdater path without touching a real installation.
func stubAgysOnPath(t *testing.T, version string) {
	t.Helper()
	binDir := filepath.Join(t.TempDir(), "agy-bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir agy stub bin: %v", err)
	}
	path := filepath.Join(binDir, executableName("agy"))
	body := "#!/bin/sh\nif [ \"$1\" = update ]; then echo 'agy updated'; else echo '" + version + "'; fi\nexit 0\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write agy stub: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// TestRunVersionCheckJSONEmitsCheckEvents drives `--check --json` against a
// stub npm whose `outdated` probe exits 1 (npm's "something is outdated" exit
// code) with mmx-cli listed at latest 2.0.0 while 1.0.0 is installed. The run
// must stream check_start → tool_check(outdated) → check_done.
func TestRunVersionCheckJSONEmitsCheckEvents(t *testing.T) {
	setTestHome(t, t.TempDir())
	stubCheckManagersOnPath(t, "echo '{\"mmx-cli\":{\"current\":\"1.0.0\",\"latest\":\"2.0.0\"}}'\nexit 1", "2.0.0")
	setAgentUpdateTools(t, []string{"minimax"})

	var out bytes.Buffer
	err := RunVersionCheck(t.Context(), Options{JSON: true, Only: []string{"minimax"}, Output: &out})
	if err != nil {
		t.Fatalf("RunVersionCheck() error = %v, output:\n%s", err, out.String())
	}

	events := decodeEventLines(t, out.String())
	assertEventKinds(t, events, "check_start", "tool_check", "check_done")
	if events[0].Total != 1 {
		t.Fatalf("check_start total = %d, want 1", events[0].Total)
	}
	check := events[1]
	if check.Binary != "mmx" || check.Manager != string(Npm) || check.VersionInstalled != "1.0.0" {
		t.Fatalf("tool_check = %+v, want mmx via npm at 1.0.0", check)
	}
	if check.VersionLatest != "2.0.0" || check.Outdated == nil || !*check.Outdated || check.LatestKnown == nil || !*check.LatestKnown {
		t.Fatalf("tool_check = %+v, want latest 2.0.0 outdated=true latest_known=true", check)
	}
}

// TestRunVersionCheckJSONFallsBackToRegistryLookup: when the one-shot
// `npm outdated --global` probe fails, the check must still answer through a
// per-package registry lookup (`npm view <pkg> version`) instead of reporting
// the tool's latest version as unknown.
func TestRunVersionCheckJSONFallsBackToRegistryLookup(t *testing.T) {
	setTestHome(t, t.TempDir())
	stubCheckManagersOnPath(t, "", "2.0.0")
	setAgentUpdateTools(t, []string{"minimax"})

	var out bytes.Buffer
	err := RunVersionCheck(t.Context(), Options{JSON: true, Only: []string{"minimax"}, Output: &out})
	if err != nil {
		t.Fatalf("RunVersionCheck() error = %v, output:\n%s", err, out.String())
	}

	events := decodeEventLines(t, out.String())
	assertEventKinds(t, events, "check_start", "tool_check", "check_done")
	check := events[1]
	if check.VersionLatest != "2.0.0" || check.Outdated == nil || !*check.Outdated || check.LatestKnown == nil || !*check.LatestKnown {
		t.Fatalf("tool_check = %+v, want registry latest 2.0.0 flagged outdated", check)
	}
}

// TestRunVersionCheckJSONUpToDateReportsKnownLatest: when the manager's
// outdated probe succeeds and the package is absent from it, the tool is up
// to date — latest_known must be true with an empty version_latest, not the
// "no query API" unknown, so the menu bar app hides the update button.
func TestRunVersionCheckJSONUpToDateReportsKnownLatest(t *testing.T) {
	setTestHome(t, t.TempDir())
	stubCheckManagersOnPath(t, "echo '{}'", "")
	setAgentUpdateTools(t, []string{"minimax"})

	var out bytes.Buffer
	err := RunVersionCheck(t.Context(), Options{JSON: true, Only: []string{"minimax"}, Output: &out})
	if err != nil {
		t.Fatalf("RunVersionCheck() error = %v, output:\n%s", err, out.String())
	}

	events := decodeEventLines(t, out.String())
	assertEventKinds(t, events, "check_start", "tool_check", "check_done")
	check := events[1]
	if check.VersionLatest != "" || check.Outdated == nil || *check.Outdated || check.LatestKnown == nil || !*check.LatestKnown {
		t.Fatalf("tool_check = %+v, want up to date with latest_known=true", check)
	}
}

// TestRunVersionCheckJSONUnknownLatestForNativeManager pins the "no query
// API" contract: a claude-native installation reports its installed version
// but latest_known=false, so consumers show an update button that simply
// runs the updater.
func TestRunVersionCheckJSONUnknownLatestForNativeManager(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	// Execution layer: `claude --version` runs the PATH stub...
	stubDir := filepath.Join(home, "stubs")
	if err := os.MkdirAll(stubDir, 0o755); err != nil {
		t.Fatalf("mkdir stub dir: %v", err)
	}
	claudeStub := filepath.Join(stubDir, executableName("claude"))
	if err := os.WriteFile(claudeStub, []byte("#!/bin/sh\necho '1.2.3'\n"), 0o755); err != nil {
		t.Fatalf("write claude stub: %v", err)
	}
	t.Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	// ...while the detection layer resolves the same name to a native
	// install layout, classifying the tool as claude-native.
	nativePath := filepath.Join(home, ".local", "share", "claude", "versions", "1.2.3", executableName("claude"))
	if err := os.MkdirAll(filepath.Dir(nativePath), 0o755); err != nil {
		t.Fatalf("mkdir native dir: %v", err)
	}
	if err := os.WriteFile(nativePath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write native binary: %v", err)
	}
	origLookup := binaryLookup
	binaryLookup = func(name string) (string, error) {
		if name == "claude" {
			return nativePath, nil
		}
		return "", errExecutableNotFound
	}
	defer func() { binaryLookup = origLookup }()

	setAgentUpdateTools(t, []string{"claude-code"})

	var out bytes.Buffer
	err := RunVersionCheck(t.Context(), Options{JSON: true, Only: []string{"claude-code"}, Output: &out})
	if err != nil {
		t.Fatalf("RunVersionCheck() error = %v, output:\n%s", err, out.String())
	}

	events := decodeEventLines(t, out.String())
	assertEventKinds(t, events, "check_start", "tool_check", "check_done")
	check := events[1]
	if check.Binary != "claude" || check.Manager != string(ClaudeNative) || check.VersionInstalled != "1.2.3" {
		t.Fatalf("tool_check = %+v, want claude-native at 1.2.3", check)
	}
	if check.LatestKnown == nil || *check.LatestKnown || check.Outdated == nil || *check.Outdated || check.VersionLatest != "" {
		t.Fatalf("tool_check = %+v, want latest_known=false outdated=false", check)
	}
}

// TestRunVersionCheckHumanOutput keeps the non-JSON output scannable: one
// line per tool, with the update arrow only when something is outdated.
func TestRunVersionCheckHumanOutput(t *testing.T) {
	setTestHome(t, t.TempDir())
	stubCheckManagersOnPath(t, "echo '{\"mmx-cli\":{\"current\":\"1.0.0\",\"latest\":\"2.0.0\"}}'", "")
	setAgentUpdateTools(t, []string{"minimax"})

	var out bytes.Buffer
	err := RunVersionCheck(t.Context(), Options{Only: []string{"minimax"}, Output: &out})
	if err != nil {
		t.Fatalf("RunVersionCheck() error = %v", err)
	}

	text := out.String()
	if !strings.Contains(text, "Checking 1 tools for updates...") {
		t.Fatalf("output missing header:\n%s", text)
	}
	if !strings.Contains(text, "MiniMax") || !strings.Contains(text, "1.0.0 -> 2.0.0 (update available)") {
		t.Fatalf("output missing update line:\n%s", text)
	}
	if strings.Contains(text, "event") {
		t.Fatalf("human output leaked NDJSON events:\n%s", text)
	}
}

// TestRunOnlyRestrictsRunAndBypassesEnabledFilter pins the --only contract:
// the run touches exactly the named tool even though enabled_tools lists a
// different one — an explicit selection is stronger than the enable list.
func TestRunOnlyRestrictsRunAndBypassesEnabledFilter(t *testing.T) {
	setTestHome(t, t.TempDir())
	stubManagersOnPath(t, "1.0.0", "2.0.0")
	setAgentUpdateTools(t, []string{"kimi"})

	var out bytes.Buffer
	err := Run(t.Context(), Options{JSON: true, Only: []string{"minimax"}, Input: strings.NewReader(""), Output: &out})
	if err != nil {
		t.Fatalf("Run() error = %v, output:\n%s", err, out.String())
	}

	events := decodeEventLines(t, out.String())
	assertEventKinds(t, events, "run_start", "tool_start", "tool_done", "run_done")
	if events[0].Total != 1 {
		t.Fatalf("run_start total = %d, want 1", events[0].Total)
	}
	if events[1].Binary != "mmx" || events[2].Status != EventStatusUpdated {
		t.Fatalf("run touched %s (%s), want mmx updated", events[1].Binary, events[2].Status)
	}
}

// TestRunOnlyAcceptsAliases: gemini resolves to agy through the same name
// matching the config flows use. The agy stub keeps the updater run off any
// real Antigravity installation.
func TestRunOnlyAcceptsAliases(t *testing.T) {
	setTestHome(t, t.TempDir())
	stubManagersOnPath(t, "1.0.0", "2.0.0")
	stubAgysOnPath(t, "1.0.0")
	setAgentUpdateTools(t, []string{"minimax"})

	var out bytes.Buffer
	err := Run(t.Context(), Options{JSON: true, Only: []string{"gemini"}, Input: strings.NewReader(""), Output: &out})
	if err != nil {
		t.Fatalf("Run() error = %v, output:\n%s", err, out.String())
	}

	events := decodeEventLines(t, out.String())
	assertEventKinds(t, events, "run_start", "tool_start", "tool_done", "run_done")
	if events[0].Total != 1 {
		t.Fatalf("run_start total = %d, want 1", events[0].Total)
	}
	if events[1].Binary != "agy" || events[2].Status != EventStatusUpToDate {
		t.Fatalf("only run touched %s (%s), want agy up_to_date", events[1].Binary, events[2].Status)
	}
}

func TestRunOnlyUnknownToolReturnsActionableError(t *testing.T) {
	setTestHome(t, t.TempDir())

	var out bytes.Buffer
	err := Run(t.Context(), Options{JSON: true, Only: []string{"no-such-tool"}, Input: strings.NewReader(""), Output: &out})
	if err == nil || !strings.Contains(err.Error(), "unknown tool(s): no-such-tool") {
		t.Fatalf("Run() error = %v, want unknown-tool error", err)
	}
	if !strings.Contains(err.Error(), "claude") || !strings.Contains(err.Error(), "mmx") {
		t.Fatalf("error = %v, want valid tool names in message", err)
	}
}

func TestResolveOnlyTools(t *testing.T) {
	ordered := GetOrderedTools(nil)

	if got, err := resolveOnlyTools(nil, ordered); got != nil || err != nil {
		t.Fatalf("resolveOnlyTools(nil) = %v, %v; want nil, nil", got, err)
	}
	if got, err := resolveOnlyTools([]string{"", ","}, ordered); got != nil || err != nil {
		t.Fatalf("resolveOnlyTools(blank) = %v, %v; want nil, nil", got, err)
	}

	got, err := resolveOnlyTools([]string{"gemini", "claude-code"}, ordered)
	if err != nil {
		t.Fatalf("resolveOnlyTools(aliases) error = %v", err)
	}
	if len(got) != 2 || got[0].BinaryName != "claude" || got[1].BinaryName != "agy" {
		t.Fatalf("resolveOnlyTools(aliases) = %v, want claude then agy in registry order", got)
	}

	if _, err := resolveOnlyTools([]string{"agy", "nope"}, ordered); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("resolveOnlyTools(unknown) error = %v, want unmatched name reported", err)
	}
}

func TestVersionIsNewer(t *testing.T) {
	cases := []struct {
		installed, latest string
		want              bool
	}{
		{"1.0.0", "2.0.0", true},
		{"2.0.0", "2.0.0", false},
		{"2.0.0", "1.9.9", false},
		{"v1.2.3", "1.10.0", true},
		{"1.2", "1.2.0", true},
		{"1.2.3-beta", "1.3.0", true},
		{"1.0.0", "", false},
		{"", "2.0.0", false},
		{"not-a-version", "2.0.0", false},
	}
	for _, tc := range cases {
		if got := versionIsNewer(tc.installed, tc.latest); got != tc.want {
			t.Fatalf("versionIsNewer(%q, %q) = %v, want %v", tc.installed, tc.latest, got, tc.want)
		}
	}
}

// TestCheckTimeoutKillsRun guards the ceiling: a hung manager probe must not
// outlive checkTimeout; the killed run reports latest unknown for the tool.
func TestCheckTimeoutKillsRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX stub timeout semantics")
	}
	setTestHome(t, t.TempDir())
	binDir := t.TempDir()
	// `exec sleep` replaces the shell image, so the context kill reaps the
	// hung probe itself instead of leaving a child holding the output pipe.
	npm := filepath.Join(binDir, executableName("npm"))
	body := "#!/bin/sh\n" +
		"if [ \"$1\" = outdated ]; then exec sleep 60; fi\n" +
		"case \"$1\" in\n" +
		"list) echo 'mmx-cli@1.0.0' ;;\n" +
		"view) exit 1 ;;\n" +
		"esac\nexit 0\n"
	if err := os.WriteFile(npm, []byte(body), 0o755); err != nil {
		t.Fatalf("write npm stub: %v", err)
	}
	for _, manager := range []string{"brew", "pnpm", "yarn"} {
		if err := os.WriteFile(filepath.Join(binDir, executableName(manager)), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
			t.Fatalf("write %s stub: %v", manager, err)
		}
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	setAgentUpdateTools(t, []string{"minimax"})

	origTimeout := checkTimeout
	checkTimeout = 2 * time.Second
	defer func() { checkTimeout = origTimeout }()

	var out bytes.Buffer
	start := time.Now()
	err := RunVersionCheck(t.Context(), Options{JSON: true, Only: []string{"minimax"}, Output: &out})
	if err != nil {
		t.Fatalf("RunVersionCheck() error = %v, output:\n%s", err, out.String())
	}
	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Fatalf("RunVersionCheck took %v, want the 2s ceiling to reap the hung probe", elapsed)
	}

	events := decodeEventLines(t, out.String())
	assertEventKinds(t, events, "check_start", "tool_check", "check_done")
	check := events[1]
	if check.LatestKnown == nil || *check.LatestKnown {
		t.Fatalf("tool_check = %+v, want latest_known=false after probe timeout", check)
	}
}
