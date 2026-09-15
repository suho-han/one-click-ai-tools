package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompletionInstallCreatesUserOwnedScript_whenShellRequested(t *testing.T) {
	// Given
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgPath := writeTempConfig(t)
	viperResetForTest(t)

	// When
	var stdout, stderr bytes.Buffer
	code := runCLI([]string{"--config", cfgPath, "completion", "install", "zsh"}, &stdout, &stderr)

	// Then
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr.String())
	}
	scriptPath := filepath.Join(home, ".oct", "completions", "oct.zsh")
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read installed script: %v", err)
	}
	if !strings.Contains(string(script), "#compdef oct") {
		t.Fatalf("installed script does not look like zsh completion: %s", script)
	}
	profilePath := filepath.Join(home, ".zshrc")
	profile, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("read profile: %v", err)
	}
	if !strings.Contains(string(profile), "source ") || !strings.Contains(string(profile), scriptPath) {
		t.Fatalf("profile = %q, want source line for %s", profile, scriptPath)
	}
	if !strings.Contains(stdout.String(), "Installed zsh completion") {
		t.Fatalf("stdout = %q, want install summary", stdout.String())
	}
}

func TestCompletionUninstallRemovesInstalledScriptAndProfileBlock_whenShellRequested(t *testing.T) {
	// Given
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgPath := writeTempConfig(t)
	viperResetForTest(t)
	profilePath := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(profilePath, []byte("before\n"), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	var installOut, installErr bytes.Buffer
	if code := runCLI([]string{"--config", cfgPath, "completion", "install", "zsh"}, &installOut, &installErr); code != 0 {
		t.Fatalf("install exit = %d, stderr: %s", code, installErr.String())
	}

	// When
	var stdout, stderr bytes.Buffer
	code := runCLI([]string{"--config", cfgPath, "completion", "uninstall", "zsh"}, &stdout, &stderr)

	// Then
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr.String())
	}
	scriptPath := filepath.Join(home, ".oct", "completions", "oct.zsh")
	if _, err := os.Stat(scriptPath); !os.IsNotExist(err) {
		t.Fatalf("script stat error = %v, want not exist", err)
	}
	profile, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("read profile: %v", err)
	}
	if got := string(profile); strings.Contains(got, "oct completion") || got != "before\n" {
		t.Fatalf("profile = %q, want original content only", got)
	}
	if !strings.Contains(stdout.String(), "Uninstalled zsh completion") {
		t.Fatalf("stdout = %q, want uninstall summary", stdout.String())
	}
}

func TestCompletionSourceLineQuotesPaths_whenShellUsesDifferentEscapes(t *testing.T) {
	// Given
	zshShell, err := parseCompletionShell("zsh")
	if err != nil {
		t.Fatalf("parse zsh shell: %v", err)
	}
	powerShell, err := parseCompletionShell("powershell")
	if err != nil {
		t.Fatalf("parse powershell shell: %v", err)
	}
	path := "/tmp/oct's/completion"

	// When
	zshLine := zshShell.sourceLine(path)
	powerShellLine := powerShell.sourceLine(path)

	// Then
	if !strings.Contains(zshLine, "'\\''") {
		t.Fatalf("zsh source line = %q, want POSIX single-quote escape", zshLine)
	}
	if !strings.Contains(powerShellLine, "oct''s") {
		t.Fatalf("powershell source line = %q, want PowerShell single-quote escape", powerShellLine)
	}
}
