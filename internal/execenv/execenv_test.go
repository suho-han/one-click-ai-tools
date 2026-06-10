package execenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildPATHAddsUserToolDirs(t *testing.T) {
	t.Setenv("HOME", "/tmp/oct-home")

	got := BuildPATH("/usr/bin")
	wantParts := []string{
		"/usr/bin",
		"/tmp/oct-home/.local/bin",
		"/tmp/oct-home/.npm-global/bin",
		"/tmp/oct-home/.opencode/bin",
	}
	for _, want := range wantParts {
		if !strings.Contains(got, want) {
			t.Fatalf("BuildPATH() missing %q in %q", want, got)
		}
	}
}

func TestLookPathFindsToolOutsideCurrentPATH(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", "")

	toolDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	toolPath := filepath.Join(toolDir, "oct-test-tool")
	if err := os.WriteFile(toolPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := LookPath("oct-test-tool")
	if err != nil {
		t.Fatalf("LookPath() error = %v", err)
	}
	if got != toolPath {
		t.Fatalf("LookPath() = %q, want %q", got, toolPath)
	}
}

func TestResolveExecutableUsesBootstrappedPATH(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", "")

	toolDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	toolPath := filepath.Join(toolDir, "oct-test-runner")
	if err := os.WriteFile(toolPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if got := ResolveExecutable("oct-test-runner"); got != toolPath {
		t.Fatalf("ResolveExecutable() = %q, want %q", got, toolPath)
	}
}
