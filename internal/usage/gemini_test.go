package usage

import (
	"context"
	"errors"
	"os"
	"os/exec"
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

func TestAntigravityCacheSerializesProcesses(t *testing.T) {
	if path := os.Getenv("OCT_ANTIGRAVITY_LOCK_TEST_CACHE"); path != "" {
		antigravityCachePath = func() string { return path }
		counter := os.Getenv("OCT_ANTIGRAVITY_LOCK_TEST_COUNT")
		antigravityUsageCommandOutput = func(context.Context, time.Duration, string, ...string) (string, error) {
			file, err := os.OpenFile(counter, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
			if err != nil {
				return "", err
			}
			_, err = file.WriteString("x")
			closeErr := file.Close()
			if err != nil {
				return "", err
			}
			if closeErr != nil {
				return "", closeErr
			}
			time.Sleep(200 * time.Millisecond)
			return "Gemini Models Weekly Limit Remaining 80% tomorrow", nil
		}
		result, ok := fetchAntigravityCLIUsage(t.Context(), baseAntigravityUsageResult())
		if !ok || result.Status != "ok" {
			t.Fatalf("child usage = %+v, ok=%v", result, ok)
		}
		return
	}

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cache := filepath.Join(dir, "antigravity-usage.json")
	counter := filepath.Join(dir, "runs")
	var commands []*exec.Cmd
	for range 2 {
		command := exec.CommandContext(t.Context(), executable, "-test.run=^TestAntigravityCacheSerializesProcesses$")
		command.Env = append(os.Environ(),
			"OCT_ANTIGRAVITY_LOCK_TEST_CACHE="+cache,
			"OCT_ANTIGRAVITY_LOCK_TEST_COUNT="+counter,
		)
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		commands = append(commands, command)
	}
	for _, command := range commands {
		if err := command.Wait(); err != nil {
			t.Fatalf("usage subprocess: %v", err)
		}
	}
	runs, err := os.ReadFile(counter)
	if err != nil {
		t.Fatal(err)
	}
	if string(runs) != "x" {
		t.Fatalf("concurrent processes launched agy %d times, want once", len(runs))
	}
}

func TestAntigravityLockWaitHonorsUsageDeadline(t *testing.T) {
	withRedirectedAntigravityCache(t)
	unlock, err := lockAntigravityUsage(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	oldCommand := antigravityUsageCommandOutput
	t.Cleanup(func() { antigravityUsageCommandOutput = oldCommand })
	antigravityUsageCommandOutput = func(context.Context, time.Duration, string, ...string) (string, error) {
		t.Fatal("agy ran while another process held the cache lock")
		return "", nil
	}

	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Millisecond)
	defer cancel()
	result, ok := fetchAntigravityCLIUsage(ctx, baseAntigravityUsageResult())
	if !ok || result.Status != "warn" || !strings.Contains(result.Message, "refresh lock unavailable") {
		t.Fatalf("locked fetch = %+v, ok=%v; want bounded warning", result, ok)
	}
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
	if result.Buckets["model:Gemini 7d"] != "0.0" {
		t.Fatalf("expected Gemini 7d used 0.0, got %q", result.Buckets["model:Gemini 7d"])
	}
	if result.Buckets["model:Claude/GPT 7d"] != "12.5" {
		t.Fatalf("expected Claude/GPT 7d used 12.5, got %q", result.Buckets["model:Claude/GPT 7d"])
	}
	if result.BucketResets["model:Claude/GPT 7d"] != "2026-09-10T17:41:25Z" {
		t.Fatalf("expected reset time, got %q", result.BucketResets["model:Claude/GPT 7d"])
	}
}

func TestParseAntigravityCLIUsageMultiWindowAndSpecificModel(t *testing.T) {
	output := strings.Join([]string{
		"Quota:",
		"Gemini Models          Weekly Limit Remaining       82%   2026-11-30 12:44 KST",
		"Gemini Models          Five Hour Limit Remaining    44%   2026-11-23 17:44 KST",
		"Claude and GPT models  Weekly Limit Remaining       68%   2026-11-30 14:53 KST",
		"Claude and GPT models  Five Hour Limit Remaining    36%   2026-11-23 19:53 KST",
		"Gemini 4 Argon         Gemini 4 Argon Weekly Limit  100%  2026-11-30 17:42 KST",
	}, "\n")

	rows := parseAntigravityCLIUsage(output)
	want := []antigravityCLIUsageRow{
		{Label: "Gemini Models", Window: "Weekly", Remaining: 82, ResetTime: "2026-11-30T03:44:00Z"},
		{Label: "Gemini Models", Window: "Five Hour", Remaining: 44, ResetTime: "2026-11-23T08:44:00Z"},
		{Label: "Claude and GPT models", Window: "Weekly", Remaining: 68, ResetTime: "2026-11-30T05:53:00Z"},
		{Label: "Claude and GPT models", Window: "Five Hour", Remaining: 36, ResetTime: "2026-11-23T10:53:00Z"},
		{Label: "Gemini 4 Argon", Window: "Weekly", Remaining: 100, ResetTime: "2026-11-30T08:42:00Z"},
	}
	if len(rows) != len(want) {
		t.Fatalf("parsed %d rows, want %d: %#v", len(rows), len(want), rows)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Fatalf("row[%d] = %#v, want %#v", i, rows[i], want[i])
		}
	}

	// Row without a reset time.
	noReset, ok := parseAntigravityCLIUsageLine("Gemini 4 Argon         Gemini 4 Argon Weekly Limit  100%")
	if !ok || noReset.Label != "Gemini 4 Argon" || noReset.Window != "Weekly" || noReset.Remaining != 100 || noReset.ResetTime != "" {
		t.Fatalf("no-reset row = %#v, ok=%v", noReset, ok)
	}

	// Single-space fallbacks.
	single5h, ok := parseAntigravityCLIUsageLine("Gemini Models Five Hour Limit Remaining 44% 2026-11-23 17:44 KST")
	if !ok || single5h.Label != "Gemini Models" || single5h.Window != "Five Hour" || single5h.Remaining != 44 || single5h.ResetTime != "2026-11-23T08:44:00Z" {
		t.Fatalf("single-space 5h row = %#v, ok=%v", single5h, ok)
	}
	singleArgon, ok := parseAntigravityCLIUsageLine("Gemini 4 Argon Gemini 4 Argon Weekly Limit 100% 2026-11-30 17:42 KST")
	if !ok || singleArgon.Label != "Gemini 4 Argon" || singleArgon.Window != "Weekly" || singleArgon.Remaining != 100 || singleArgon.ResetTime != "2026-11-30T08:42:00Z" {
		t.Fatalf("single-space argon row = %#v, ok=%v", singleArgon, ok)
	}
}

func TestFetchAntigravityUsageAllQuotas(t *testing.T) {
	output := strings.Join([]string{
		"Quota:",
		"Gemini Models          Weekly Limit Remaining       82%   2026-11-30 12:44 KST",
		"Gemini Models          Five Hour Limit Remaining    44%   2026-11-23 17:44 KST",
		"Claude and GPT models  Weekly Limit Remaining       68%   2026-11-30 14:53 KST",
		"Claude and GPT models  Five Hour Limit Remaining    36%   2026-11-23 19:53 KST",
		"Gemini 4 Argon         Gemini 4 Argon Weekly Limit  100%  2026-11-30 17:42 KST",
	}, "\n") + "\n"
	withMockAntigravityUsageCommand(t, output, nil)

	result := FetchAntigravityUsage(t.Context())
	if result.Status != "ok" {
		t.Fatalf("status = %q, want ok", result.Status)
	}
	if result.Used != "64.0" {
		t.Fatalf("used = %q, want 64.0", result.Used)
	}
	wantBuckets := map[string]string{
		"model:Gemini 7d":         "18.0",
		"model:Gemini 5h":         "56.0",
		"model:Claude/GPT 7d":     "32.0",
		"model:Claude/GPT 5h":     "64.0",
		"model:Gemini 4 Argon 7d": "0.0",
	}
	if len(result.Buckets) != len(wantBuckets) {
		t.Fatalf("buckets len = %d, want %d: %#v", len(result.Buckets), len(wantBuckets), result.Buckets)
	}
	for k, wantVal := range wantBuckets {
		if got := result.Buckets[k]; got != wantVal {
			t.Fatalf("Buckets[%q] = %q, want %q", k, got, wantVal)
		}
	}
	wantResets := map[string]string{
		"model:Gemini 7d":         "2026-11-30T03:44:00Z",
		"model:Gemini 5h":         "2026-11-23T08:44:00Z",
		"model:Claude/GPT 7d":     "2026-11-30T05:53:00Z",
		"model:Claude/GPT 5h":     "2026-11-23T10:53:00Z",
		"model:Gemini 4 Argon 7d": "2026-11-30T08:42:00Z",
	}
	if len(result.BucketResets) != len(wantResets) {
		t.Fatalf("bucket_resets len = %d, want %d: %#v", len(result.BucketResets), len(wantResets), result.BucketResets)
	}
	for k, wantVal := range wantResets {
		if got := result.BucketResets[k]; got != wantVal {
			t.Fatalf("BucketResets[%q] = %q, want %q", k, got, wantVal)
		}
	}
}

func TestNormalizeAntigravityResetTime(t *testing.T) {
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load Asia/Shanghai: %v", err)
	}
	cases := []struct {
		name string
		raw  string
		loc  *time.Location
		want string
	}{
		{"rfc3339 passthrough", "2026-09-10T17:41:25Z", time.UTC, "2026-09-10T17:41:25Z"},
		{"kst in utc loc", "2026-11-30 12:44 KST", time.UTC, "2026-11-30T03:44:00Z"},
		{"gmt+9 offset", "2026-11-30 12:44 GMT+9", time.UTC, "2026-11-30T03:44:00Z"},
		{"numeric offset", "2026-11-30 12:44 +0900", time.UTC, "2026-11-30T03:44:00Z"},
		{"utc zone", "2026-11-30 12:44 UTC", time.UTC, "2026-11-30T12:44:00Z"},
		{"ambiguous cst matches local shanghai", "2026-11-30 12:44 CST", shanghai, "2026-11-30T04:44:00Z"},
		{"ambiguous cst untouched in utc loc", "2026-11-30 12:44 CST", time.UTC, "2026-11-30 12:44 CST"},
		{"non-timestamp passthrough", "tomorrow", time.UTC, "tomorrow"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeAntigravityResetTime(tc.raw, tc.loc); got != tc.want {
				t.Fatalf("normalizeAntigravityResetTime(%q, %v) = %q, want %q", tc.raw, tc.loc, got, tc.want)
			}
		})
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
		{"sign-in prompt", "Authentication required. Please visit the URL to log in:", nil, true},
		{"rate limit", "429 Too Many Requests", errors.New("exit status 1"), false},
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
	if first.Status != "warn" || !first.AuthRequired {
		t.Fatalf("expected reconnect warning, got %+v", first)
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
	if second.Status != "warn" || !second.AuthRequired {
		t.Fatalf("expected reconnect warning during backoff, got %+v", second)
	}
	if !strings.Contains(second.Message, "paused") {
		t.Fatalf("expected paused message, got %q", second.Message)
	}
}

func TestAntigravityUsageReportsAuthWhenAgyWaitsForSignIn(t *testing.T) {
	withRedirectedAntigravityCache(t)
	oldCommand := antigravityUsageCommandOutput
	t.Cleanup(func() { antigravityUsageCommandOutput = oldCommand })
	antigravityUsageCommandOutput = func(ctx context.Context, timeout time.Duration, _ string, _ ...string) (string, error) {
		child, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		<-child.Done()
		return "Authentication required. Please visit the URL to log in:", child.Err()
	}

	ctx, cancel := context.WithTimeout(t.Context(), 1200*time.Millisecond)
	defer cancel()
	finished := make(chan struct{})
	result := fetchWithDeadline(ctx, "antigravity", func(ctx context.Context) UsageResult {
		defer close(finished)
		return FetchAntigravityUsage(ctx)
	})
	<-finished
	if result.Status != "warn" || !result.AuthRequired {
		t.Fatalf("expected a reconnect warning, not a usage timeout: %+v", result)
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
	if result.Status != "warn" || !result.AuthRequired {
		t.Fatalf("expected cached result to still require reconnect, got %+v", result)
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
	if second.Status != "ok" || second.Source != "cache" || second.AuthRequired {
		t.Fatalf("expected healthy cached result, got %+v", second)
	}
	if first.Buckets["model:Gemini 7d"] != "1.0" {
		t.Fatalf("expected live bucket 1.0, got %#v", first.Buckets)
	}
	if second.Buckets["model:Gemini 7d"] != first.Buckets["model:Gemini 7d"] {
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
	result := FetchAntigravityUsage(t.Context())
	if calls() != 1 {
		t.Fatalf("expected a fresh agy run, got %d invocations", calls())
	}
	if result.Status != "ok" || result.Source != "agy-cli" {
		t.Fatalf("expected live result after hold expired, got status %q source %q", result.Status, result.Source)
	}
	if result.Buckets["model:Gemini 7d"] != "1.0" {
		t.Fatalf("expected fresh bucket from 99%% remaining, got %q", result.Buckets["model:Gemini 7d"])
	}
}

func TestFetchAntigravityUsageUpgradesUnversionedCacheEvenWithinMinInterval(t *testing.T) {
	calls := withMockAntigravityUsageCommand(t,
		"Gemini Models\tWeekly Limit Remaining\t82%\t2026-11-30 12:44 KST\n"+
			"Gemini Models\tFive Hour Limit Remaining\t44%\t2026-11-23 17:44 KST\n", nil)

	// An unversioned cache entry (written before multi-window parsing) must
	// not suppress a fresh agy run during the 5-minute min interval.
	writeAntigravityUsageCache(antigravityUsageCacheEntry{
		FetchedAtMs: time.Now().Add(-30 * time.Second).UnixMilli(),
		Buckets:     map[string]string{"model:Gemini": "56.0"},
	})
	result := FetchAntigravityUsage(t.Context())
	if calls() != 1 {
		t.Fatalf("expected unversioned cache to trigger a fresh agy run, got %d invocations", calls())
	}
	if result.Source != "agy-cli" || result.Buckets["model:Gemini 7d"] != "18.0" || result.Buckets["model:Gemini 5h"] != "56.0" {
		t.Fatalf("expected upgraded multi-window buckets, got source=%q buckets=%#v", result.Source, result.Buckets)
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
