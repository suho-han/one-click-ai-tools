package sessionrefresh

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/suho-han/one-click-tools/internal/update"
)

func TestRefreshUnknownProvider(t *testing.T) {
	results := Refresh(RefreshOptions{Providers: []string{"nope"}, DryRun: true})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "unsupported" {
		t.Fatalf("expected unsupported, got %#v", results[0])
	}
}

func TestRefreshAliasMapsGeminiToAntigravity(t *testing.T) {
	results := Refresh(RefreshOptions{Providers: []string{"gemini"}, DryRun: true})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Provider != "agy" {
		t.Fatalf("expected gemini alias to resolve to agy, got %#v", results[0])
	}
}

func TestResolveToolMatchesAlias(t *testing.T) {
	tool, ok := resolveTool(update.NormalizeToolName("agent"))
	if !ok {
		t.Fatal("expected to resolve Cursor tool from agent alias")
	}
	if tool.BinaryName != "cursor-agent" {
		t.Fatalf("expected cursor-agent, got %q", tool.BinaryName)
	}
}

func TestFirstNonEmptyLine(t *testing.T) {
	if got := firstNonEmptyLine("\n hello\nworld"); got != "hello" {
		t.Fatalf("unexpected first non-empty line: %q", got)
	}
}

func TestProbeCodexSessionUsesBootstrappedPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is unix-only")
	}

	home := t.TempDir()
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	codexPath := filepath.Join(binDir, "codex")
	if err := os.WriteFile(codexPath, []byte("#!/bin/sh\necho Logged in\n"), 0o755); err != nil {
		t.Fatalf("write codex fixture failed: %v", err)
	}

	t.Setenv("HOME", home)
	t.Setenv("PATH", "/usr/bin:/bin")

	result := probeCodexSession(RefreshOptions{}, update.Tool{BinaryName: "codex"})
	if result.Status != "ok" {
		t.Fatalf("expected ok result via bootstrapped PATH, got %#v", result)
	}
	if result.Message != "Logged in" {
		t.Fatalf("unexpected codex probe message: %q", result.Message)
	}
}

func TestFindFirstBinaryUsesBootstrappedPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is unix-only")
	}

	home := t.TempDir()
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	agyPath := filepath.Join(binDir, "agy")
	if err := os.WriteFile(agyPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write agy fixture failed: %v", err)
	}

	t.Setenv("HOME", home)
	t.Setenv("PATH", "/usr/bin:/bin")

	got, err := findFirstBinary("agy")
	if err != nil {
		t.Fatalf("expected agy to resolve via bootstrapped PATH: %v", err)
	}
	if got != agyPath {
		t.Fatalf("unexpected agy path: got %q want %q", got, agyPath)
	}
}
