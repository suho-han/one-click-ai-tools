package usage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFetchOpenCodeUsage_NoLogsWarns(t *testing.T) {
	tempHome := t.TempDir()
	prevHome := os.Getenv("HOME")
	t.Cleanup(func() {
		_ = os.Setenv("HOME", prevHome)
	})
	_ = os.Setenv("HOME", tempHome)

	result := FetchOpenCodeUsage()
	if result.Status != "warn" {
		t.Fatalf("expected warn status, got %q", result.Status)
	}
	if !strings.HasPrefix(result.Message, "No data:") {
		t.Fatalf("expected no-data message, got %q", result.Message)
	}
}

func TestFetchOpenCodeUsage_NoMetricsWarns(t *testing.T) {
	tempHome := t.TempDir()
	logDir := filepath.Join(tempHome, ".opencode", "sessions")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	logPath := filepath.Join(logDir, "latest.jsonl")
	if err := os.WriteFile(logPath, []byte("{\"message\":\"hello\"}\n"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	prevHome := os.Getenv("HOME")
	t.Cleanup(func() {
		_ = os.Setenv("HOME", prevHome)
	})
	_ = os.Setenv("HOME", tempHome)

	result := FetchOpenCodeUsage()
	if result.Status != "warn" {
		t.Fatalf("expected warn status, got %q", result.Status)
	}
	if !strings.Contains(result.Message, "no usage metrics") {
		t.Fatalf("expected no-usage-metrics message, got %q", result.Message)
	}
}

func TestFetchOpenCodeUsage_FromLocalLogsOK(t *testing.T) {
	tempHome := t.TempDir()
	logDir := filepath.Join(tempHome, ".opencode", "sessions")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	logPath := filepath.Join(logDir, "latest.jsonl")
	content := "{\"used_percent\":42.5,\"secondary\":{\"window_minutes\":10080,\"used_percent\":77}}\n"
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	prevHome := os.Getenv("HOME")
	t.Cleanup(func() {
		_ = os.Setenv("HOME", prevHome)
	})
	_ = os.Setenv("HOME", tempHome)

	result := FetchOpenCodeUsage()
	if result.Status != "ok" {
		t.Fatalf("expected ok status, got %q", result.Status)
	}
	if result.Used != "42.5" {
		t.Fatalf("expected used=42.5, got %q", result.Used)
	}
	if result.Buckets["7d"] != "77" {
		t.Fatalf("expected 7d bucket=77, got %#v", result.Buckets)
	}
	if !strings.Contains(result.Message, "Fetched from local OpenCode session logs") {
		t.Fatalf("expected fetched message, got %q", result.Message)
	}
}
