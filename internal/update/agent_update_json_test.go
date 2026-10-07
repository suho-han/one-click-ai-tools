package update

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// decodeEventLines splits NDJSON output and decodes every line; a malformed
// line fails the test because `--json` stdout must stay machine-readable.
func decodeEventLines(t *testing.T, out string) []updateEvent {
	t.Helper()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("expected at least one NDJSON event, got %q", out)
	}
	events := make([]updateEvent, 0, len(lines))
	for i, line := range lines {
		var ev updateEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("line %d is not valid JSON (%v): %q", i+1, err, line)
		}
		events = append(events, ev)
	}
	return events
}

func assertEventKinds(t *testing.T, events []updateEvent, want ...string) {
	t.Helper()
	got := make([]string, 0, len(events))
	for _, ev := range events {
		got = append(got, ev.Event)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("event kinds = %v, want %v", got, want)
	}
}

func setAgentUpdateTools(t *testing.T, enabled []string) {
	t.Helper()
	origEnabled := viper.Get("enabled_tools")
	origOrder := viper.Get("agent_order")
	viper.Set("enabled_tools", enabled)
	viper.Set("agent_order", enabled)
	t.Cleanup(func() {
		viper.Set("enabled_tools", origEnabled)
		viper.Set("agent_order", origOrder)
	})
}

// stubManagersOnPath shadows the machine's real package managers with stub
// executables so Run's real exec calls (version probes, installs) are
// hermetic: the machine may or may not have the tested tools installed. The
// npm stub answers `list` with versionBefore (or versionAfter once `install`
// has run, tracked via a state file); empty versions mean "not installed".
func stubManagersOnPath(t *testing.T, versionBefore, versionAfter string) {
	t.Helper()
	binDir := filepath.Join(t.TempDir(), "bin")
	stateFile := filepath.Join(t.TempDir(), "npm-state")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir stub bin: %v", err)
	}
	listBody := ":" // no output: the tool is not installed
	switch {
	case versionBefore != "":
		listBody = "if [ -f '" + stateFile + "' ]; then\necho 'mmx-cli@" + versionAfter + "'\nelse\necho 'mmx-cli@" + versionBefore + "'\nfi"
	case versionAfter != "":
		listBody = "if [ -f '" + stateFile + "' ]; then\necho 'mmx-cli@" + versionAfter + "'\nfi"
	}
	writeStub := func(name, body string) {
		t.Helper()
		path := filepath.Join(binDir, executableName(name))
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
			t.Fatalf("write stub %s: %v", name, err)
		}
	}
	writeStub("npm", "case \"$1\" in\nlist)\n"+listBody+"\n;;\ninstall)\ntouch '"+stateFile+"'\n;;\nesac\nexit 0\n")
	for _, manager := range []string{"brew", "pnpm", "yarn"} {
		writeStub(manager, "exit 1\n")
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestEventEmitterWritesNDJSONRecords(t *testing.T) {
	buf := &bytes.Buffer{}
	emit := newEventEmitter(true, buf)
	emit.runStart(2)
	emit.toolStart(Tool{Name: "MiniMax", BinaryName: "mmx"}, Npm, 1, 2, "1.0.0")
	emit.toolDone(Tool{Name: "MiniMax", BinaryName: "mmx"}, EventStatusUpdated, "2.0.0", 3.25, "")
	emit.runDone(0)

	events := decodeEventLines(t, buf.String())
	assertEventKinds(t, events, "run_start", "tool_start", "tool_done", "run_done")
	if events[0].Total != 2 {
		t.Fatalf("run_start total = %d, want 2", events[0].Total)
	}
	start := events[1]
	if start.Binary != "mmx" || start.Manager != string(Npm) || start.Index != 1 || start.VersionBefore != "1.0.0" {
		t.Fatalf("tool_start = %+v", start)
	}
	done := events[2]
	if done.Status != EventStatusUpdated || done.VersionAfter != "2.0.0" || done.DurationSec != 3.25 || done.Error != "" {
		t.Fatalf("tool_done = %+v", done)
	}
	if events[3].Failed != 0 {
		t.Fatalf("run_done failed = %d, want 0", events[3].Failed)
	}

	silent := &bytes.Buffer{}
	muted := newEventEmitter(false, silent)
	muted.runStart(1)
	muted.runDone(0)
	if silent.Len() != 0 {
		t.Fatalf("inactive emitter wrote %q", silent.String())
	}
}

// TestRunJSONEmitsUpdatedEventSequence drives a full `--json` run against a
// stub npm on PATH: the tool reports 1.0.0 before the install and 2.0.0 after
// (the stub flips its state file when `npm install` runs), so the run must
// stream run_start → tool_start → tool_done(updated) → run_done and print no
// human-readable lines.
func TestRunJSONEmitsUpdatedEventSequence(t *testing.T) {
	setTestHome(t, t.TempDir())
	stubManagersOnPath(t, "1.0.0", "2.0.0")
	setAgentUpdateTools(t, []string{"minimax"})

	var out bytes.Buffer
	err := Run(t.Context(), Options{JSON: true, Input: strings.NewReader(""), Output: &out})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	events := decodeEventLines(t, out.String())
	assertEventKinds(t, events, "run_start", "tool_start", "tool_done", "run_done")
	if events[0].Total != 1 {
		t.Fatalf("run_start total = %d, want 1", events[0].Total)
	}
	start := events[1]
	if start.Binary != "mmx" || start.Manager != string(Npm) || start.VersionBefore != "1.0.0" {
		t.Fatalf("tool_start = %+v, want mmx via npm at 1.0.0", start)
	}
	done := events[2]
	if done.Status != EventStatusUpdated || done.VersionAfter != "2.0.0" {
		t.Fatalf("tool_done = %+v, want updated 2.0.0", done)
	}
	if events[3].Failed != 0 {
		t.Fatalf("run_done failed = %d, want 0", events[3].Failed)
	}
	if strings.ContainsAny(out.String(), "✓✗") {
		t.Fatalf("JSON output leaked human-readable lines:\n%s", out.String())
	}
}

// TestRunJSONSkipsMissingToolsWithoutPrompting pins the background contract:
// JSON mode never prompts (the prompt would corrupt the event stream) and a
// not-installed tool is reported as skipped_not_installed instead of being
// silently installed on EOF.
func TestRunJSONSkipsMissingToolsWithoutPrompting(t *testing.T) {
	setTestHome(t, t.TempDir())
	// An npm stub reporting no installed packages keeps the "not installed"
	// verdict independent of what the machine really has installed.
	stubManagersOnPath(t, "", "")
	reset := stubManagerDetection(t)
	defer reset()
	setAgentUpdateTools(t, []string{"minimax"})

	var out bytes.Buffer
	err := Run(t.Context(), Options{JSON: true, Input: strings.NewReader("y\n"), Output: &out})
	if err != nil {
		t.Fatalf("Run() error = %v, output:\n%s", err, out.String())
	}

	events := decodeEventLines(t, out.String())
	assertEventKinds(t, events, "tool_done", "run_start", "run_done")
	if events[0].Status != EventStatusSkippedNotInstalled || events[0].Binary != "mmx" {
		t.Fatalf("first event = %+v, want mmx skipped_not_installed", events[0])
	}
	if events[1].Total != 0 || events[2].Failed != 0 {
		t.Fatalf("empty run events = %+v, %+v", events[1], events[2])
	}
	if strings.Contains(out.String(), "[Y/n]") {
		t.Fatalf("JSON output leaked the install prompt:\n%s", out.String())
	}
}

// TestRunSkipMissingWithoutJSONKeepsHumanOutput covers the explicit
// --skip-missing flag without --json: same skip decision, human prose.
func TestRunSkipMissingWithoutJSONKeepsHumanOutput(t *testing.T) {
	setTestHome(t, t.TempDir())
	stubManagersOnPath(t, "", "")
	reset := stubManagerDetection(t)
	defer reset()
	setAgentUpdateTools(t, []string{"minimax"})

	var out bytes.Buffer
	err := Run(t.Context(), Options{SkipMissing: true, Input: strings.NewReader("y\n"), Output: &out})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	text := out.String()
	if !strings.Contains(text, "Skipping MiniMax (not installed).") {
		t.Fatalf("output missing skip notice:\n%s", text)
	}
	if !strings.Contains(text, "No tools selected for update.") {
		t.Fatalf("output missing empty-run notice:\n%s", text)
	}
	if strings.Contains(text, "[Y/n]") {
		t.Fatalf("skip-missing must not prompt:\n%s", text)
	}
}

// TestRunJSONBrewMissingReportsGenericSkippedStatus exercises the in-loop
// skip path for a non-Node manager: a brew-classified tool (binary under a
// /Cellar path) without brew on PATH gets the generic skipped status —
// npm_missing stays reserved for the Node.js toolchain.
func TestRunJSONBrewMissingReportsGenericSkippedStatus(t *testing.T) {
	setTestHome(t, t.TempDir())
	reset := stubManagerDetection(t)
	defer reset()
	// A failing stub brew keeps the upfront `brew update` fast and harmless.
	stubManagersOnPath(t, "", "")

	fakeCellarBinary := filepath.Join(t.TempDir(), "Cellar", "fake", "mmx")
	origLookup := binaryLookup
	binaryLookup = func(name string) (string, error) {
		if name == "mmx" {
			return fakeCellarBinary, nil
		}
		return "", errExecutableNotFound
	}
	defer func() { binaryLookup = origLookup }()

	setAgentUpdateTools(t, []string{"minimax"})

	var out bytes.Buffer
	err := Run(t.Context(), Options{JSON: true, InstallMissing: true, Input: strings.NewReader(""), Output: &out})
	if err == nil || !strings.Contains(err.Error(), "1 tool(s) failed") {
		t.Fatalf("Run() error = %v, want 1 tool failure, output:\n%s", err, out.String())
	}

	events := decodeEventLines(t, out.String())
	assertEventKinds(t, events, "run_start", "tool_start", "tool_done", "run_done")
	done := events[2]
	if done.Status != EventStatusSkipped || !strings.Contains(done.Error, "brew not found in PATH") {
		t.Fatalf("tool_done = %+v, want skipped with brew guidance", done)
	}
	if events[3].Failed != 1 {
		t.Fatalf("run_done failed = %d, want 1", events[3].Failed)
	}
}

func TestRunJSONNoMatchingToolsEmitsEmptyRun(t *testing.T) {
	setTestHome(t, t.TempDir())
	setAgentUpdateTools(t, []string{"no-such-tool"})

	var out bytes.Buffer
	err := Run(t.Context(), Options{JSON: true, Input: strings.NewReader(""), Output: &out})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	events := decodeEventLines(t, out.String())
	assertEventKinds(t, events, "run_start", "run_done")
	if events[0].Total != 0 || events[1].Failed != 0 {
		t.Fatalf("empty run events = %+v, %+v", events[0], events[1])
	}
}

// TestRunJSONInstallMissingInstallsToolThroughNpm pins the confirmation-
// dialog contract: with --install-missing a not-installed tool is included
// without prompting and installed through its default manager (the stub npm
// here reports 2.0.0 once its install ran).
func TestRunJSONInstallMissingInstallsToolThroughNpm(t *testing.T) {
	setTestHome(t, t.TempDir())
	stubManagersOnPath(t, "", "2.0.0")
	setAgentUpdateTools(t, []string{"minimax"})

	var out bytes.Buffer
	err := Run(t.Context(), Options{JSON: true, InstallMissing: true, Input: strings.NewReader(""), Output: &out})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	events := decodeEventLines(t, out.String())
	assertEventKinds(t, events, "run_start", "tool_start", "tool_done", "run_done")
	if events[0].Total != 1 {
		t.Fatalf("run_start total = %d, want 1", events[0].Total)
	}
	if events[1].VersionBefore != "" {
		t.Fatalf("tool_start version_before = %q, want empty (not installed)", events[1].VersionBefore)
	}
	done := events[2]
	if done.Status != EventStatusUpdated || done.VersionAfter != "2.0.0" {
		t.Fatalf("tool_done = %+v, want updated 2.0.0", done)
	}
	if events[3].Failed != 0 {
		t.Fatalf("run_done failed = %d, want 0", events[3].Failed)
	}
}

// TestRunJSONNpmMissingReportsNpmMissingStatus: a tool whose manager needs
// the Node.js toolchain with none of it on PATH gets the dedicated
// npm_missing status so consumers can offer a Node.js install link.
func TestRunJSONNpmMissingReportsNpmMissingStatus(t *testing.T) {
	setTestHome(t, t.TempDir())
	reset := stubManagerDetection(t)
	defer reset()
	setAgentUpdateTools(t, []string{"minimax"})

	var out bytes.Buffer
	err := Run(t.Context(), Options{JSON: true, InstallMissing: true, Input: strings.NewReader(""), Output: &out})
	if err == nil || !strings.Contains(err.Error(), "1 tool(s) failed") {
		t.Fatalf("Run() error = %v, want 1 tool failure, output:\n%s", err, out.String())
	}

	events := decodeEventLines(t, out.String())
	assertEventKinds(t, events, "run_start", "tool_start", "tool_done", "run_done")
	done := events[2]
	if done.Status != EventStatusNpmMissing || !strings.Contains(done.Error, "npm not found in PATH") {
		t.Fatalf("tool_done = %+v, want npm_missing with npm guidance", done)
	}
}

// TestRunJSONFailedToolCarriesInstallerOutputTail pins the failure-dialog
// contract: a failed tool_done must carry the installer's own error output
// (as a capped tail), not just "exit status 1", so the menu bar app's alert
// can explain why a provider failed to update.
func TestRunJSONFailedToolCarriesInstallerOutputTail(t *testing.T) {
	setTestHome(t, t.TempDir())
	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir stub bin: %v", err)
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
		"install)\necho 'npm error EACCES permission denied, access /usr/local/lib/node_modules'\nexit 1\n;;\n"+
		"esac\nexit 0\n")
	for _, manager := range []string{"brew", "pnpm", "yarn"} {
		writeStub(manager, "exit 1\n")
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	setAgentUpdateTools(t, []string{"minimax"})

	var out bytes.Buffer
	err := Run(t.Context(), Options{JSON: true, Input: strings.NewReader(""), Output: &out})
	if err == nil || !strings.Contains(err.Error(), "1 tool(s) failed") {
		t.Fatalf("Run() error = %v, want 1 tool failure, output:\n%s", err, out.String())
	}

	events := decodeEventLines(t, out.String())
	assertEventKinds(t, events, "run_start", "tool_start", "tool_done", "run_done")
	done := events[2]
	if done.Status != EventStatusFailed {
		t.Fatalf("tool_done = %+v, want failed", done)
	}
	if !strings.Contains(done.Error, "exit status 1") || !strings.Contains(done.Error, "npm error EACCES permission denied") {
		t.Fatalf("tool_done error = %q, want exec error plus installer output tail", done.Error)
	}
}
