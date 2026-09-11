package usage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

// writeQwenUsageFixture writes a token-usage JSONL under home/qwenHome with a
// mix of today's records, a stale record, and a non-usage file.
func writeQwenUsageFixture(t *testing.T, qwenHome string, todayCount int) {
	t.Helper()
	dir := filepath.Join(qwenHome, "usage")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	var lines []string
	for i := 0; i < todayCount; i++ {
		lines = append(lines, fmt.Sprintf(`{"date":%q,"authType":"qwen-oauth","model":"qwen3-coder","apiDurationMs":1200}`, today))
	}
	lines = append(lines, fmt.Sprintf(`{"date":%q,"authType":"qwen-oauth","model":"qwen3-coder"}`, yesterday))
	lines = append(lines, `{"localDate":"`+today+`","authType":"openai-api-key"}`)

	file := filepath.Join(dir, "token-usage-2026-09.jsonl")
	if err := os.WriteFile(file, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Unrelated file must be ignored by the filename filter.
	if err := os.WriteFile(filepath.Join(dir, "notes.jsonl"), []byte(`{"date":"`+today+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestFetchQwenUsageCountsToday(t *testing.T) {
	qwenHome := t.TempDir()
	t.Setenv("QWEN_HOME", qwenHome)
	writeQwenUsageFixture(t, qwenHome, 3)

	result := FetchQwenUsage(t.Context())

	if result.Status != "ok" {
		t.Fatalf("status = %q, message = %q", result.Status, result.Message)
	}
	if result.Used != "4" {
		t.Errorf("used = %q, want 4 (3 today-dated + 1 localDate today)", result.Used)
	}
	if result.Limit != "100" {
		t.Errorf("limit = %q, want 100 (viper default)", result.Limit)
	}
	if result.Period != "1d" || result.Unit != "req" {
		t.Errorf("period/unit = %q/%q, want 1d/req", result.Period, result.Unit)
	}
	if result.Plan != "OAuth" {
		t.Errorf("plan = %q, want OAuth (first record's auth type)", result.Plan)
	}
}

func TestFetchQwenUsageRespectsDailyLimitConfig(t *testing.T) {
	qwenHome := t.TempDir()
	t.Setenv("QWEN_HOME", qwenHome)
	writeQwenUsageFixture(t, qwenHome, 5)

	viper.Set("qwen_daily_limit", 4)
	t.Cleanup(func() { viper.Set("qwen_daily_limit", nil) })
	result := FetchQwenUsage(t.Context())

	if result.Status != "warn" {
		t.Fatalf("status = %q, want warn at/over limit", result.Status)
	}
	if result.Limit != "4" {
		t.Errorf("limit = %q, want 4", result.Limit)
	}
	if !strings.Contains(result.Message, "limit") {
		t.Errorf("message = %q, want limit mention", result.Message)
	}
}

func TestFetchQwenUsageNoRecords(t *testing.T) {
	t.Setenv("QWEN_HOME", t.TempDir())

	result := FetchQwenUsage(t.Context())
	if result.Status != "warn" {
		t.Fatalf("status = %q, want warn", result.Status)
	}
	if !strings.Contains(result.Message, "No data") {
		t.Errorf("message = %q, want no-data guidance", result.Message)
	}
}

func TestCollectQwenUsageIgnoresMalformedLines(t *testing.T) {
	qwenHome := t.TempDir()
	dir := filepath.Join(qwenHome, "usage")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	today := time.Now().Format("2006-01-02")
	content := "not json\n\n" + `{"date":"` + today + `","authType":"qwen-oauth"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "token-usage-2026-09.jsonl"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	count, plan, any := collectQwenUsage(qwenHome, time.Now())
	if count != 1 {
		t.Errorf("count = %d, want 1 (malformed and empty lines skipped)", count)
	}
	if plan != "OAuth" {
		t.Errorf("plan = %q, want OAuth", plan)
	}
	if !any {
		t.Error("any = false, want true")
	}
}

func TestQwenPlanLabel(t *testing.T) {
	tests := map[string]string{
		"qwen-oauth":     "OAuth",
		"openai-api-key": "API Key",
		"":               "",
		"custom-mode":    "custom-mode",
	}
	for in, want := range tests {
		if got := qwenPlanLabel(in); got != want {
			t.Errorf("qwenPlanLabel(%q) = %q, want %q", in, got, want)
		}
	}
}
