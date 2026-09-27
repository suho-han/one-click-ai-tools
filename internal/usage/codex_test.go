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

	result := FetchCodexUsage(t.Context())
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

	result := FetchCodexUsage(t.Context())
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

func TestFetchCodexUsageMapsPlusPrimaryAndWeeklySecondary(t *testing.T) {
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
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"plan_type":"plus","rate_limit":{"primary_window":{"used_percent":21.5,"limit_window_seconds":18000,"reset_at":1785289572},"secondary_window":{"used_percent":63.0,"limit_window_seconds":604800,"reset_at":1785894372}}}`)
	}))
	defer server.Close()
	t.Setenv("OCT_CODEX_USAGE_ENDPOINT", server.URL)

	result := FetchCodexUsage(t.Context())
	if result.Status != "ok" {
		t.Fatalf("expected ok, got %s", result.Status)
	}
	if result.Plan != "plus" {
		t.Fatalf("expected plus plan, got %s", result.Plan)
	}
	if result.Buckets["5h"] != "21.5" {
		t.Fatalf("expected 5h bucket 21.5, got %s", result.Buckets["5h"])
	}
	if result.Buckets["7d"] != "63.0" {
		t.Fatalf("expected 7d bucket 63.0, got %s", result.Buckets["7d"])
	}
	if result.BucketResets["5h"] != "1785289572" {
		t.Fatalf("expected 5h reset 1785289572, got %s", result.BucketResets["5h"])
	}
	if result.BucketResets["7d"] != "1785894372" {
		t.Fatalf("expected 7d reset 1785894372, got %s", result.BucketResets["7d"])
	}
	if result.Used != "63.0" {
		t.Fatalf("expected used to stay weekly 63.0, got %s", result.Used)
	}
}

func TestFetchCodexUsageMapsPrimaryOnlySubWeeklyWindow(t *testing.T) {
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
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"plan_type":"plus","rate_limit":{"primary_window":{"used_percent":11.0,"limit_window_seconds":18000,"reset_at":1785289572}}}`)
	}))
	defer server.Close()
	t.Setenv("OCT_CODEX_USAGE_ENDPOINT", server.URL)

	result := FetchCodexUsage(t.Context())
	if result.Status != "ok" {
		t.Fatalf("expected ok, got %s", result.Status)
	}
	if result.Buckets["5h"] != "11.0" {
		t.Fatalf("expected 5h bucket 11.0, got %s", result.Buckets["5h"])
	}
	if result.Buckets["7d"] != "" {
		t.Fatalf("did not expect 7d bucket, got %s", result.Buckets["7d"])
	}
	if result.Used != "11.0" {
		t.Fatalf("expected used fallback to 5h 11.0, got %s", result.Used)
	}
}

func TestFetchCodexUsageSessionLogMapsPrimaryFiveHourWindow(t *testing.T) {
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
	line := `{"type":"event_msg","payload":{"type":"token_count","rate_limits":{"primary":{"used_percent":16.0,"window_minutes":300},"secondary":{"used_percent":72.0,"window_minutes":10080}}}}`
	if err := os.WriteFile(logPath, []byte(line+"\n"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	result := FetchCodexUsage(t.Context())
	if result.Status != "ok" {
		t.Fatalf("expected ok, got %s", result.Status)
	}
	if result.Buckets["5h"] != "16.0" {
		t.Fatalf("expected 5h bucket 16.0, got %s", result.Buckets["5h"])
	}
	if result.Buckets["7d"] != "72.0" {
		t.Fatalf("expected 7d bucket 72.0, got %s", result.Buckets["7d"])
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

func TestFetchCodexUsageSessionLogMapsWeeklyPrimaryToSevenDay(t *testing.T) {
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
	// Some accounts report the weekly window on primary; it must fill 7d
	// instead of being dropped, and must not overwrite the secondary's value.
	line := `{"type":"event_msg","payload":{"type":"token_count","rate_limits":{"primary":{"used_percent":31.0,"window_minutes":10080},"secondary":{"used_percent":72.0,"window_minutes":10080}}}}`
	if err := os.WriteFile(logPath, []byte(line+"\n"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	result := FetchCodexUsage(t.Context())
	if result.Buckets["7d"] != "72.0" {
		t.Fatalf("expected 7d bucket 72.0, got %s", result.Buckets["7d"])
	}
	if result.Buckets["5h"] != "" {
		t.Fatalf("did not expect a 5h bucket, got %s", result.Buckets["5h"])
	}
	if result.Used != "72.0" {
		t.Fatalf("expected used 72.0, got %s", result.Used)
	}
}

func TestFetchCodexUsageSessionLogSurvivesOversizedLine(t *testing.T) {
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
	// A conversation payload larger than the default 64KB scanner token sits
	// before the token_count event; the scan must not stop at the big line.
	big := strings.Repeat("x", 200*1024)
	var b strings.Builder
	fmt.Fprintf(&b, "{\"type\":\"event_msg\",\"payload\":{\"type\":\"token_count\",\"info\":{\"total_token_usage\":{\"input_tokens\":%d}}}}\n", len(big))
	b.WriteString(big + "\n")
	b.WriteString(`{"type":"event_msg","payload":{"type":"token_count","rate_limits":{"primary":{"used_percent":16.0,"window_minutes":300},"secondary":{"used_percent":72.0,"window_minutes":10080}}}}` + "\n")
	if err := os.WriteFile(logPath, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	result := FetchCodexUsage(t.Context())
	if result.Buckets["5h"] != "16.0" {
		t.Fatalf("expected 5h bucket 16.0 after oversized line, got %q", result.Buckets["5h"])
	}
	if result.Buckets["7d"] != "72.0" {
		t.Fatalf("expected 7d bucket 72.0 after oversized line, got %q", result.Buckets["7d"])
	}
}

func TestAddCodexBackendWindowKeepsFirstWindowPerBucket(t *testing.T) {
	primary := 5.0
	secondary := 40.0
	fiveH := 18000
	day := 86400
	reset := int64(1785289572)
	buckets := map[string]string{}
	resets := map[string]string{}

	// A 5h primary plus a daily secondary both map to the 5h bucket; the
	// first window written must win instead of being silently overwritten.
	addCodexBackendWindow(buckets, resets, &codexBackendRateLimitWindow{UsedPercent: &primary, LimitWindowSeconds: &fiveH, ResetAt: &reset})
	addCodexBackendWindow(buckets, resets, &codexBackendRateLimitWindow{UsedPercent: &secondary, LimitWindowSeconds: &day, ResetAt: &reset})

	if buckets["5h"] != "5.0" {
		t.Fatalf("expected primary 5h value to win, got %q", buckets["5h"])
	}
	if resets["5h"] != "1785289572" {
		t.Fatalf("expected primary reset to win, got %q", resets["5h"])
	}
}
