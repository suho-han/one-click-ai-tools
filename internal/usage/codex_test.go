package usage

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFetchCodexUsageMapsWeeklyBucketOnly(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("CODEX_HOME", filepath.Join(tmp, ".codex"))
	t.Setenv("OCT_CODEX_USAGE_ENDPOINT", "")

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
	if result.Used != "72.0" {
		t.Fatalf("expected used 72.0, got %s", result.Used)
	}
	if result.Buckets["5h"] != "" {
		t.Fatalf("did not expect codex 5h bucket, got %s", result.Buckets["5h"])
	}
	if result.Buckets["7d"] != "72.0" {
		t.Fatalf("expected 7d bucket 72.0, got %s", result.Buckets["7d"])
	}
	if strings.Contains(result.Message, "\x1b]8;;") {
		t.Fatalf("message should not contain hyperlink escape sequence")
	}
}

func TestFetchCodexUsageUsesBackendWeeklyOnlyWindow(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("CODEX_HOME", filepath.Join(tmp, ".codex"))

	codexDir := filepath.Join(tmp, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(codexDir, "auth.json"), []byte(`{"tokens":{"access_token":"test-token","account_id":"acct-1"}}`), 0o600); err != nil {
		t.Fatalf("write auth failed: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("Authorization header did not contain test bearer token")
		}
		if r.Header.Get("ChatGPT-Account-Id") != "acct-1" {
			t.Fatalf("ChatGPT-Account-Id header did not contain test account id")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"plan_type":"prolite","rate_limit":{"primary_window":{"used_percent":7,"limit_window_seconds":604800,"reset_at":1785289572}}}`)
	}))
	defer server.Close()
	t.Setenv("OCT_CODEX_USAGE_ENDPOINT", server.URL)

	result := FetchCodexUsage()
	if result.Status != "ok" {
		t.Fatalf("expected ok, got %s", result.Status)
	}
	if result.Source != "backend" {
		t.Fatalf("expected backend source, got %s", result.Source)
	}
	if result.Plan != "prolite" {
		t.Fatalf("expected backend plan, got %s", result.Plan)
	}
	if result.Buckets["5h"] != "" {
		t.Fatalf("did not expect 5h bucket for weekly-only backend window, got %s", result.Buckets["5h"])
	}
	if result.Buckets["7d"] != "7.0" {
		t.Fatalf("expected 7d bucket 7.0, got %s", result.Buckets["7d"])
	}
	if result.Used != "7.0" {
		t.Fatalf("expected used fallback to weekly 7.0, got %s", result.Used)
	}
}

func TestCodexLocalModelSourceDetailIncludesSpark(t *testing.T) {
	t.Setenv("OCT_USAGE_DEBUG", "1")
	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "session.jsonl")
	content := strings.Join([]string{
		`{"type":"turn_context","payload":{"model":"gpt-5.3-codex-spark"}}`,
		`{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"cached_input_tokens":20,"output_tokens":30,"total_tokens":130}}}}`,
		`{"type":"event_msg","payload":{"type":"token_count","info":{"model":"gpt-5.2-codex","last_token_usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}}}`,
		"",
	}, "\n")
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write log failed: %v", err)
	}

	detail := codexLocalModelSourceDetail([]string{logPath}, 10)
	if !strings.Contains(detail, "local_recent_models=") {
		t.Fatalf("expected local model detail, got %q", detail)
	}
	if !strings.Contains(detail, "gpt-5.3-codex-spark:130t/1e") {
		t.Fatalf("expected spark model detail, got %q", detail)
	}
	if !strings.Contains(detail, "gpt-5.2-codex:15t/1e") {
		t.Fatalf("expected regular model detail, got %q", detail)
	}
}
