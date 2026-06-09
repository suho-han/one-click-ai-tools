package usage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCountAntigravitySessionsCountsSupportedArtifacts(t *testing.T) {
	root := t.TempDir()
	pathA := filepath.Join(root, "conversations")
	pathB := filepath.Join(root, "cache")
	if err := os.MkdirAll(pathA, 0o755); err != nil {
		t.Fatalf("mkdir pathA failed: %v", err)
	}
	if err := os.MkdirAll(pathB, 0o755); err != nil {
		t.Fatalf("mkdir pathB failed: %v", err)
	}
	for _, rel := range []string{"chat-1.pb", "chat-2.db", "notes.jsonl"} {
		if err := os.WriteFile(filepath.Join(pathA, rel), []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s failed: %v", rel, err)
		}
	}
	if err := os.Mkdir(filepath.Join(pathB, "project-1"), 0o755); err != nil {
		t.Fatalf("mkdir project dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pathB, "ignore.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write ignore.txt failed: %v", err)
	}

	count, matched := countAntigravitySessions([]string{pathA, pathB})
	if count != 4 {
		t.Fatalf("expected 4 artifacts, got %d", count)
	}
	if len(matched) != 2 {
		t.Fatalf("expected 2 matched paths, got %v", matched)
	}
}

func TestFetchAntigravityLocalUsageNoHome(t *testing.T) {
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() {
		if oldHome == "" {
			_ = os.Unsetenv("HOME")
			return
		}
		_ = os.Setenv("HOME", oldHome)
	})
	if err := os.Unsetenv("HOME"); err != nil {
		t.Fatalf("unset HOME failed: %v", err)
	}

	result := FetchAntigravityLocalUsage()
	if result.Provider != "antigravity" {
		t.Fatalf("expected provider antigravity, got %q", result.Provider)
	}
	if result.Status != "warn" {
		t.Fatalf("expected warn status, got %q", result.Status)
	}
}

func TestFetchGeminiUsageDelegatesToAntigravity(t *testing.T) {
	result := FetchGeminiUsage()
	if result.Provider != "antigravity" {
		t.Fatalf("expected provider antigravity, got %q", result.Provider)
	}
	if !strings.EqualFold(result.Source, "local") {
		t.Fatalf("expected local source, got %q", result.Source)
	}
}
