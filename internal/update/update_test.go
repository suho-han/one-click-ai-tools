package update

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func setTestHome(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
	}
}

func executableName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

func TestToolFiltering(t *testing.T) {
	ordered := GetOrderedTools(nil)

	result := GetFilteredTools([]string{"gemini"}, ordered)
	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}
	if result[0].BinaryName != "agy" {
		t.Fatalf("expected agy, got %s", result[0].BinaryName)
	}
}

func TestToolFilteringCaseInsensitive(t *testing.T) {
	ordered := GetOrderedTools(nil)

	result := GetFilteredTools([]string{"Gemini"}, ordered)
	if len(result) != 1 {
		t.Fatalf("expected 1 tool for 'Gemini', got %d", len(result))
	}
	if result[0].BinaryName != "agy" {
		t.Fatalf("expected agy, got %s", result[0].BinaryName)
	}
}

func TestToolFilteringMultiple(t *testing.T) {
	ordered := GetOrderedTools(nil)

	result := GetFilteredTools([]string{"antigravity", "cursor"}, ordered)
	if len(result) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(result))
	}
	if result[0].BinaryName != "agy" {
		t.Fatalf("expected agy, got %s", result[0].BinaryName)
	}
	if result[1].BinaryName != "cursor-agent" {
		t.Fatalf("expected cursor-agent, got %s", result[1].BinaryName)
	}
}

func TestToolFilteringEmpty(t *testing.T) {
	ordered := GetOrderedTools(nil)

	result := GetFilteredTools([]string{}, ordered)
	if len(result) != len(Tools) {
		t.Fatalf("expected all %d tools when enabled is empty, got %d", len(Tools), len(result))
	}
}

func TestPlanIsInstalled(t *testing.T) {
	tests := []struct {
		name string
		plan Plan
		want bool
	}{
		{name: "active path", plan: Plan{ActivePath: "/tmp/codex", Reason: "default fallback"}, want: true},
		{name: "version", plan: Plan{VersionBefore: "1.2.3", Reason: "default fallback"}, want: true},
		{name: "installed package lookup", plan: Plan{Reason: "installed package lookup"}, want: true},
		{name: "missing default fallback", plan: Plan{Reason: "default fallback"}, want: false},
	}

	for _, tc := range tests {
		if got := tc.plan.IsInstalled(); got != tc.want {
			t.Fatalf("%s: IsInstalled() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestConfirmMissingToolInstalls(t *testing.T) {
	origPrompt := confirmInstallPrompt
	defer func() { confirmInstallPrompt = origPrompt }()

	prompted := []string{}
	confirmInstallPrompt = func(_ io.Reader, _ io.Writer, plan Plan) (bool, error) {
		prompted = append(prompted, plan.Tool.Name)
		return plan.Tool.Name == "OpenAI Codex", nil
	}

	tools := []Tool{
		{Name: "OpenAI Codex", BinaryName: "codex"},
		{Name: "Claude Code", BinaryName: "claude"},
	}
	plans := []Plan{
		{Tool: tools[0], Reason: "default fallback", InstallCommand: []string{"npm", "install", "-g", "@openai/codex"}},
		{Tool: tools[1], Reason: "active binary path", ActivePath: "/tmp/claude"},
	}

	confirmedTools, confirmedPlans, err := confirmMissingToolInstalls(strings.NewReader(""), io.Discard, tools, plans)
	if err != nil {
		t.Fatalf("confirmMissingToolInstalls() error = %v", err)
	}
	if len(prompted) != 1 || prompted[0] != "OpenAI Codex" {
		t.Fatalf("prompted = %v, want [OpenAI Codex]", prompted)
	}
	if len(confirmedTools) != 2 || len(confirmedPlans) != 2 {
		t.Fatalf("confirmed lens = (%d,%d), want (2,2)", len(confirmedTools), len(confirmedPlans))
	}
}

func TestDefaultConfirmInstallPrompt(t *testing.T) {
	out := &strings.Builder{}
	ok, err := defaultConfirmInstallPrompt(strings.NewReader("n\n"), out, Plan{
		Tool:           Tool{Name: "OpenAI Codex"},
		Manager:        Npm,
		InstallCommand: []string{"npm", "install", "-g", "@openai/codex"},
	})
	if err != nil {
		t.Fatalf("defaultConfirmInstallPrompt() error = %v", err)
	}
	if ok {
		t.Fatal("expected negative answer to decline install")
	}
	if !strings.Contains(out.String(), "OpenAI Codex is not installed") || !strings.Contains(out.String(), "npm install -g @openai/codex") {
		t.Fatalf("unexpected prompt output: %q", out.String())
	}
}

func TestFormatVersionSummary(t *testing.T) {
	if got := formatVersionSummary("1.0.0", "1.0.1"); got != " (1.0.0 → 1.0.1)" {
		t.Fatalf("formatVersionSummary change = %q", got)
	}
	if got := formatVersionSummary("1.0.1", "1.0.1"); got != " (1.0.1)" {
		t.Fatalf("formatVersionSummary same = %q", got)
	}
	if got := formatVersionSummary("", "1.0.1"); got != " (1.0.1)" {
		t.Fatalf("formatVersionSummary after = %q", got)
	}
	if got := formatVersionSummary("", ""); got != "" {
		t.Fatalf("formatVersionSummary empty = %q", got)
	}
}

func TestIsAlreadyUpToDatePrefersObservedVersionChange(t *testing.T) {
	if isAlreadyUpToDate(ClaudeNative, "2.1.214", "2.1.217", "Already up to date") {
		t.Fatal("version change should be reported as an update even when command output contains no-change text")
	}
	if !isAlreadyUpToDate(ClaudeNative, "2.1.217", "2.1.217", "") {
		t.Fatal("same version should be reported as already up to date")
	}
	if !isAlreadyUpToDate(ClaudeNative, "", "", "Already up to date") {
		t.Fatal("no-change command output should be used when versions are unavailable")
	}
}

func TestIsNpmPermissionError(t *testing.T) {
	if !isNpmPermissionError("npm ERR! code EACCES\nnpm ERR! permission denied") {
		t.Fatal("expected EACCES output to be detected as npm permission error")
	}
	if isNpmPermissionError("npm ERR! 404 Not Found") {
		t.Fatal("did not expect unrelated npm error to be treated as permission error")
	}
}

func TestIsNpmConflictError(t *testing.T) {
	if !isNpmConflictError("npm ERR! code EEXIST\nnpm ERR! File exists: /home/me/.local/bin/claude") {
		t.Fatal("expected EEXIST output to be detected as conflict")
	}
	if !isNpmConflictError("npm ERR! code ENOTEMPTY\nnpm ERR! ENOTEMPTY: directory not empty, rename '/a' -> '/b'") {
		t.Fatal("expected ENOTEMPTY output to be detected as conflict")
	}
	if isNpmConflictError("npm ERR! 404 Not Found") {
		t.Fatal("did not expect unrelated npm error to be treated as conflict")
	}
}

func TestExtractNpmDestPath(t *testing.T) {
	output := "npm ERR! code ENOTEMPTY\nnpm ERR! dest /home/me/.local/lib/node_modules/@github/.copilot-abc123\n"
	if got := extractNpmDestPath(output); got != "/home/me/.local/lib/node_modules/@github/.copilot-abc123" {
		t.Fatalf("extractNpmDestPath() = %q", got)
	}
}

func TestShouldRemoveNpmConflictDest(t *testing.T) {
	home := "/home/me"

	if !shouldRemoveNpmConflictDest("/home/me/.local/lib/node_modules/@github/.copilot-abc123", home) {
		t.Fatal("expected local npm temp directory to be removable")
	}
	if shouldRemoveNpmConflictDest("/usr/local/lib/node_modules/@github/.copilot-abc123", home) {
		t.Fatal("did not expect global directory to be removable")
	}
	if shouldRemoveNpmConflictDest("/home/me/.local/lib/node_modules/@github/copilot", home) {
		t.Fatal("did not expect non-temp package directory to be removable")
	}
}

func TestBootstrapPATHAddsUserToolDirs(t *testing.T) {
	home := filepath.Clean("/tmp/oct-home")
	setTestHome(t, home)

	got := bootstrapPATH("/usr/bin")
	wantParts := []string{
		filepath.Clean("/usr/bin"),
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".npm-global", "bin"),
		filepath.Join(home, ".opencode", "bin"),
	}
	for _, want := range wantParts {
		if !strings.Contains(got, want) {
			t.Fatalf("bootstrapPATH() missing %q in %q", want, got)
		}
	}
}

func TestLookPathWithBootstrapFindsToolOutsideCurrentPATH(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	t.Setenv("PATH", "")

	toolDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	toolPath := filepath.Join(toolDir, executableName("oct-test-tool"))
	if err := os.WriteFile(toolPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := lookPathWithBootstrap(executableName("oct-test-tool"))
	if err != nil {
		t.Fatalf("lookPathWithBootstrap() error = %v", err)
	}
	if got != toolPath {
		t.Fatalf("lookPathWithBootstrap() = %q, want %q", got, toolPath)
	}
}

func TestResolveExecutableUsesBootstrappedPATH(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	t.Setenv("PATH", "")

	toolDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	toolPath := filepath.Join(toolDir, executableName("oct-test-runner"))
	if err := os.WriteFile(toolPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if got := resolveExecutable(executableName("oct-test-runner")); got != toolPath {
		t.Fatalf("resolveExecutable() = %q, want %q", got, toolPath)
	}
}
