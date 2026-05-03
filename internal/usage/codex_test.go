package usage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFetchCodexUsageMaps5hBucket(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	logDir := filepath.Join(tmp, ".codex", "sessions", "2026", "05", "03")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	logPath := filepath.Join(logDir, "rollout-2026-05-03T20-00-00.jsonl")
	line := `{"type":"event_msg","payload":{"type":"token_count","rate_limits":{"primary":{"used_percent":16.0},"secondary":{"used_percent":72.0,"window_minutes":10080}}}}`
	if err := os.WriteFile(logPath, []byte(line+"\n"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	result := FetchCodexUsage()
	if result.Status != "ok" {
		t.Fatalf("expected ok, got %s", result.Status)
	}
	if result.Used != "16.0" {
		t.Fatalf("expected used 16.0, got %s", result.Used)
	}
	if result.Buckets["5h"] != "16.0" {
		t.Fatalf("expected 5h bucket 16.0, got %s", result.Buckets["5h"])
	}
	if result.Buckets["7d"] != "72.0" {
		t.Fatalf("expected 7d bucket 72.0, got %s", result.Buckets["7d"])
	}
	if strings.Contains(result.Message, "\x1b]8;;") {
		t.Fatalf("message should not contain hyperlink escape sequence")
	}
}
