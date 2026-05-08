package usage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseOpenCodeUsageFromJSONL_RateLimitsShape(t *testing.T) {
	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "session.jsonl")
	line := `{"type":"event_msg","payload":{"type":"token_count","rate_limits":{"primary":{"used_percent":22.2},"secondary":{"used_percent":66.6,"window_minutes":10080}}}}`
	if err := os.WriteFile(logPath, []byte(line+"\n"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	used, weekly, ok, err := parseOpenCodeUsageFromJSONL(logPath)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if used != "22.2" {
		t.Fatalf("expected used=22.2, got %s", used)
	}
	if weekly != "66.6" {
		t.Fatalf("expected weekly=66.6, got %s", weekly)
	}
}

func TestParseOpenCodeUsageFromJSONL_FlatShape(t *testing.T) {
	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "session.jsonl")
	line := `{"used_percent":15.5,"secondary":{"used_percent":45.0,"window_minutes":10080}}`
	if err := os.WriteFile(logPath, []byte(line+"\n"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	used, weekly, ok, err := parseOpenCodeUsageFromJSONL(logPath)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if used != "15.5" {
		t.Fatalf("expected used=15.5, got %s", used)
	}
	if weekly != "45" {
		t.Fatalf("expected weekly=45, got %s", weekly)
	}
}

func TestFetchOpenCodeUsage_NoLogs(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	result := FetchOpenCodeUsage()
	if result.Provider != "opencode" {
		t.Fatalf("expected provider=opencode, got %s", result.Provider)
	}
	if result.Status != "ok" {
		t.Fatalf("expected status=ok, got %s", result.Status)
	}
	if result.Used != "0" {
		t.Fatalf("expected used=0, got %s", result.Used)
	}
}

func TestFetchOpenCodeUsage_FromLocalLogs(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	logDir := filepath.Join(tmp, ".opencode", "sessions", "2026", "05", "09")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	logPath := filepath.Join(logDir, "latest.jsonl")
	line := `{"payload":{"rate_limits":{"primary":{"used_percent":31.0},"secondary":{"used_percent":78.0,"window_minutes":10080}}}}`
	if err := os.WriteFile(logPath, []byte(line+"\n"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	result := FetchOpenCodeUsage()
	if result.Status != "ok" {
		t.Fatalf("expected status=ok, got %s (%s)", result.Status, result.Message)
	}
	if result.Used != "31" {
		t.Fatalf("expected used=31, got %s", result.Used)
	}
	if result.Buckets["5h"] != "31" {
		t.Fatalf("expected 5h bucket=31, got %s", result.Buckets["5h"])
	}
	if result.Buckets["7d"] != "78" {
		t.Fatalf("expected 7d bucket=78, got %s", result.Buckets["7d"])
	}
}
