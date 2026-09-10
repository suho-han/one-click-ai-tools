package execenv

import (
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

func TestBuildPATHAddsUserToolDirs(t *testing.T) {
	home := filepath.Clean("/tmp/oct-home")
	setTestHome(t, home)

	got := BuildPATH("/usr/bin")
	wantParts := []string{
		filepath.Clean("/usr/bin"),
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".npm-global", "bin"),
		filepath.Join(home, ".opencode", "bin"),
	}
	for _, want := range wantParts {
		if !strings.Contains(got, want) {
			t.Fatalf("BuildPATH() missing %q in %q", want, got)
		}
	}
}

func TestLookPathFindsToolOutsideCurrentPATH(t *testing.T) {
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

	got, err := LookPath(executableName("oct-test-tool"))
	if err != nil {
		t.Fatalf("LookPath() error = %v", err)
	}
	if got != toolPath {
		t.Fatalf("LookPath() = %q, want %q", got, toolPath)
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

	if got := ResolveExecutable(executableName("oct-test-runner")); got != toolPath {
		t.Fatalf("ResolveExecutable() = %q, want %q", got, toolPath)
	}
}

// TestLookPathRejectsNonExecutableFile pins the Unix rule for the bootstrap
// fallback: a regular file without an exec bit must not count as found.
func TestLookPathRejectsNonExecutableFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix exec-bit semantics do not apply on Windows")
	}
	home := t.TempDir()
	setTestHome(t, home)
	t.Setenv("PATH", "")

	toolDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(toolDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	toolPath := filepath.Join(toolDir, executableName("oct-test-stale"))
	if err := os.WriteFile(toolPath, []byte("not executable"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := LookPath(executableName("oct-test-stale")); err == nil {
		t.Fatalf("LookPath() accepted a non-executable file at %s", toolPath)
	}

	if err := os.Chmod(toolPath, 0o755); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}
	got, err := LookPath(executableName("oct-test-stale"))
	if err != nil {
		t.Fatalf("LookPath() error = %v, want found after chmod +x", err)
	}
	if got != toolPath {
		t.Fatalf("LookPath() = %q, want %q", got, toolPath)
	}
}

// TestHasWindowsExecutableExt pins the Windows rule for the fallback: an
// executable extension (not an exec bit) decides findability.
func TestHasWindowsExecutableExt(t *testing.T) {
	for path, want := range map[string]bool{
		"C:\\tools\\oct.exe":   true,
		"C:\\tools\\run.cmd":   true,
		"C:\\tools\\run.bat":   true,
		"C:\\tools\\run.com":   true,
		"C:\\tools\\RUN.EXE":   true,
		"C:\\tools\\oct":       false,
		"C:\\tools\\notes.txt": false,
		"oct.exe":              true,
	} {
		if got := hasWindowsExecutableExt(path); got != want {
			t.Errorf("hasWindowsExecutableExt(%q) = %v, want %v", path, got, want)
		}
	}
}
