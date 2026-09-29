package usage

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// withRedirectedAntigravityCache points the antigravity state file at a
// per-test temp directory so cache/backoff state never leaks between tests
// or onto the real user cache.
func withRedirectedAntigravityCache(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "antigravity-usage.json")
	old := antigravityCachePath
	t.Cleanup(func() { antigravityCachePath = old })
	antigravityCachePath = func() string { return path }
	return path
}

// withMockAntigravityUsageCommand replaces the agy invocation with a canned
// response and returns a counter of how many times it ran.
func withMockAntigravityUsageCommand(t *testing.T, output string, err error) func() int {
	t.Helper()
	withRedirectedAntigravityCache(t)
	calls := 0
	old := antigravityUsageCommandOutput
	t.Cleanup(func() { antigravityUsageCommandOutput = old })
	antigravityUsageCommandOutput = func(ctx context.Context, timeout time.Duration, name string, args ...string) (string, error) {
		calls++
		if timeout != 20*time.Second {
			t.Fatalf("timeout = %v, want 20s", timeout)
		}
		if name != "agy" {
			t.Fatalf("command = %q, want agy", name)
		}
		if len(args) != 2 || args[0] != "--print" || args[1] != "/usage" {
			t.Fatalf("args = %v, want [--print /usage]", args)
		}
		return output, err
	}
	return func() int { return calls }
}

func TestParseAntigravityCLIUsage(t *testing.T) {
	rows := parseAntigravityCLIUsage("Gemini Models\tWeekly Limit Remaining\t100%\t2026-09-10T17:41:25Z\nClaude and GPT models\tWeekly Limit Remaining\t87.5%\t2026-09-10T17:41:25Z\n")
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Label != "Gemini Models" || rows[0].Window != "Weekly" || rows[0].Remaining != 100 || rows[0].ResetTime != "2026-09-10T17:41:25Z" {
		t.Fatalf("unexpected first row: %#v", rows[0])
	}
	if rows[1].Label != "Claude and GPT models" || rows[1].Remaining != 87.5 {
		t.Fatalf("unexpected second row: %#v", rows[1])
	}
}

func TestFetchAntigravityUsageUsesAgyPrintUsage(t *testing.T) {
	withMockAntigravityUsageCommand(t, "Gemini Models\tWeekly Limit Remaining\t100%\t2026-09-10T17:41:25Z\nClaude and GPT models\tWeekly Limit Remaining\t87.5%\t2026-09-10T17:41:25Z\n", nil)

	result := FetchAntigravityUsage(t.Context())
	if result.Provider != "antigravity" {
		t.Fatalf("expected provider antigravity, got %q", result.Provider)
	}
	if result.Status != "ok" {
		t.Fatalf("expected ok status, got %q", result.Status)
	}
	if result.Source != "agy-cli" {
		t.Fatalf("expected agy-cli source, got %q", result.Source)
	}
	if result.Used != "12.5" {
		t.Fatalf("expected max used percent 12.5, got %q", result.Used)
	}
	if result.Buckets["model:Gemini"] != "0.0" {
		t.Fatalf("expected Gemini used 0.0, got %q", result.Buckets["model:Gemini"])
	}
	if result.Buckets["model:Claude/GPT"] != "12.5" {
		t.Fatalf("expected Claude/GPT used 12.5, got %q", result.Buckets["model:Claude/GPT"])
	}
	if result.BucketResets["model:Claude/GPT"] != "2026-09-10T17:41:25Z" {
		t.Fatalf("expected reset time, got %q", result.BucketResets["model:Claude/GPT"])
	}
}

func TestFetchAntigravityUsageNoCLIUsage(t *testing.T) {
	withMockAntigravityUsageCommand(t, "", errors.New("agy failed"))

	result := FetchAntigravityUsage(t.Context())
	if result.Status != "warn" {
		t.Fatalf("expected warn status, got %q", result.Status)
	}
	if result.Used != "n/a" {
		t.Fatalf("expected n/a usage, got %q", result.Used)
	}
	if result.Unit != "percent" {
		t.Fatalf("expected percent unit, got %q", result.Unit)
	}
	if strings.EqualFold(result.Unit, "sessions") || strings.Contains(strings.ToLower(result.Message), "session") {
		t.Fatalf("local session fallback leaked into result: %#v", result)
	}
}

func TestFetchGeminiUsageDelegatesToAntigravity(t *testing.T) {
	withMockAntigravityUsageCommand(t, "Gemini Models\tWeekly Limit Remaining\t99%\t2026-09-10T17:41:25Z\n", nil)

	result := FetchGeminiUsage(t.Context())
	if result.Provider != "antigravity" {
		t.Fatalf("expected provider antigravity, got %q", result.Provider)
	}
	if !strings.EqualFold(result.Source, "agy-cli") {
		t.Fatalf("expected agy-cli source, got %q", result.Source)
	}
}

func TestIsAntigravityAuthFailure(t *testing.T) {
	cases := []struct {
		name   string
		output string
		err    error
		want   bool
	}{
		{"silent auth refusal", "Print mode: not logged in and no controlling terminal; cannot complete interactive login", errors.New("exit status 1"), true},
		{"keychain auth error", "", errors.New("exit status 1: You are not logged into Antigravity."), true},
		{"browser fallback", "Failed to open browser: exit status 1", nil, true},
		{"plain failure", "", errors.New("exit status 1: something else broke"), false},
		{"success output", "Gemini Models\tWeekly Limit Remaining\t99%\ttomorrow", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isAntigravityAuthFailure(tc.output, tc.err); got != tc.want {
				t.Fatalf("isAntigravityAuthFailure(%q, %v) = %v, want %v", tc.output, tc.err, got, tc.want)
			}
		})
	}
}

func TestFetchAntigravityUsageAuthFailureBacksOff(t *testing.T) {
	calls := withMockAntigravityUsageCommand(t,
		"Print mode: not logged in and no controlling terminal; cannot complete interactive login",
		errors.New("exit status 1"))

	first := FetchAntigravityUsage(t.Context())
	if first.Status != "error" {
		t.Fatalf("expected error status, got %q (%s)", first.Status, first.Message)
	}
	if !strings.Contains(first.Message, "re-authenticate") {
		t.Fatalf("expected actionable re-auth message, got %q", first.Message)
	}
	if calls() != 1 {
		t.Fatalf("expected 1 agy invocation, got %d", calls())
	}

	// The backoff must persist across processes: the next refresh (a fresh
	// oct process) reads the state file and skips the agy invocation.
	second := FetchAntigravityUsage(t.Context())
	if calls() != 1 {
		t.Fatalf("expected agy to be skipped during backoff, got %d invocations", calls())
	}
	if second.Status != "warn" {
		t.Fatalf("expected warn status on held refresh, got %q (%s)", second.Status, second.Message)
	}
	if !strings.Contains(second.Message, "paused") {
		t.Fatalf("expected paused message, got %q", second.Message)
	}
}

func TestFetchAntigravityUsageAuthFailureServesCachedUsage(t *testing.T) {
	withMockAntigravityUsageCommand(t, "", errors.New("exit status 1: You are not logged into Antigravity."))
	// Last good numbers from a while ago, plus an active auth backoff
	// recorded by a previous process.
	writeAntigravityUsageCache(antigravityUsageCacheEntry{
		FetchedAtMs: time.Now().Add(-10 * time.Minute).UnixMilli(),
		Buckets:     map[string]string{"model:Gemini": "5.0"},
	})
	markAntigravityAuthFailure(time.Now(), "silent auth failed")

	result := FetchAntigravityUsage(t.Context())
	if result.Status != "ok" {
		t.Fatalf("expected cached ok status, got %q (%s)", result.Status, result.Message)
	}
	if result.Source != "cache" {
		t.Fatalf("expected cache source, got %q", result.Source)
	}
	if result.Buckets["model:Gemini"] != "5.0" {
		t.Fatalf("expected cached bucket preserved, got %#v", result.Buckets)
	}
	if !strings.Contains(result.Message, "auth") {
		t.Fatalf("expected auth context in message, got %q", result.Message)
	}
}

func TestFetchAntigravityUsageMinIntervalServesCache(t *testing.T) {
	calls := withMockAntigravityUsageCommand(t,
		"Gemini Models\tWeekly Limit Remaining\t99%\t2026-09-10T17:41:25Z\n", nil)

	first := FetchAntigravityUsage(t.Context())
	if first.Status != "ok" || first.Source != "agy-cli" {
		t.Fatalf("expected live ok result, got status %q source %q", first.Status, first.Source)
	}

	// A second refresh within the min interval is served from the cache
	// without invoking agy again.
	second := FetchAntigravityUsage(t.Context())
	if calls() != 1 {
		t.Fatalf("expected min interval to suppress second agy run, got %d invocations", calls())
	}
	if second.Status != "ok" || second.Source != "cache" {
		t.Fatalf("expected cached result, got status %q source %q", second.Status, second.Source)
	}
	if second.Buckets["model:Gemini"] != first.Buckets["model:Gemini"] {
		t.Fatalf("expected same buckets, got %#v vs %#v", second.Buckets, first.Buckets)
	}
}

func TestFetchAntigravityUsageRerunsAfterMinInterval(t *testing.T) {
	calls := withMockAntigravityUsageCommand(t,
		"Gemini Models\tWeekly Limit Remaining\t99%\t2026-09-10T17:41:25Z\n", nil)

	// Seed an old last-good entry so the min interval has elapsed.
	writeAntigravityUsageCache(antigravityUsageCacheEntry{
		FetchedAtMs: time.Now().Add(-10 * time.Minute).UnixMilli(),
		Buckets:     map[string]string{"model:Gemini": "2.0"},
	})

	if _, held := antigravityFetchHold(time.Now()); held {
		t.Fatalf("expected no hold after min interval elapsed")
	}
	result := FetchAntigravityUsage(t.Context())
	if calls() != 1 {
		t.Fatalf("expected a fresh agy run, got %d invocations", calls())
	}
	if result.Status != "ok" || result.Source != "agy-cli" {
		t.Fatalf("expected live result after hold expired, got status %q source %q", result.Status, result.Source)
	}
	if result.Buckets["model:Gemini"] != "1.0" {
		t.Fatalf("expected fresh bucket from 99%% remaining, got %q", result.Buckets["model:Gemini"])
	}
}

func TestFetchAntigravityUsageSuccessClearsBackoff(t *testing.T) {
	calls := withMockAntigravityUsageCommand(t,
		"Gemini Models\tWeekly Limit Remaining\t99%\t2026-09-10T17:41:25Z\n", nil)

	// A stale backoff (already expired) must not block, and a successful
	// fetch must clear the recorded reason.
	markAntigravityAuthFailure(time.Now().Add(-antigravityAuthBackoff), "old failure")

	result := FetchAntigravityUsage(t.Context())
	if calls() != 1 {
		t.Fatalf("expected agy to run after backoff expiry, got %d invocations", calls())
	}
	if result.Status != "ok" || result.Source != "agy-cli" {
		t.Fatalf("expected live ok result, got status %q source %q", result.Status, result.Source)
	}
	if entry := readAntigravityUsageCache(); entry.BackoffUntilMs != 0 || entry.BackoffReason != "" {
		t.Fatalf("expected success to clear the backoff record, got %#v", entry)
	}
}
