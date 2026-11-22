package usage

import (
	"crypto/sha256"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/suho-han/one-click-ai-tools/internal/netclient"
)

// claudeCacheTestSetup isolates the last-good cache file and the keychain so
// tests only exercise the token -> API -> cache path via CLAUDE_API_TOKEN.
func claudeCacheTestSetup(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	origPath := claudeUsageCachePath
	claudeUsageCachePath = func() string { return filepath.Join(tmp, "claude-usage.json") }
	t.Cleanup(func() { claudeUsageCachePath = origPath })

	t.Cleanup(mockSecurityCommand(t, `{}`))

	t.Setenv("CLAUDE_API_TOKEN", "dummy-token")
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
}

func claudeCacheTestClient(t *testing.T, status int, body string, calls *int) {
	t.Helper()
	origClient := netclient.DefaultClient.HTTPClient
	origRetries := netclient.DefaultClient.MaxRetries
	netclient.DefaultClient.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			*calls++
			if status != http.StatusOK && body == "" {
				body = `{}`
			}
			return &http.Response{
				StatusCode: status,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	}
	netclient.DefaultClient.MaxRetries = 0
	t.Cleanup(func() {
		netclient.DefaultClient.HTTPClient = origClient
		netclient.DefaultClient.MaxRetries = origRetries
	})
}

func claudeCacheSilentClient(t *testing.T, calls *int) {
	t.Helper()
	origClient := netclient.DefaultClient.HTTPClient
	origRetries := netclient.DefaultClient.MaxRetries
	netclient.DefaultClient.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			*calls++
			t.Errorf("usage endpoint must not be called, got %s", req.URL)
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}
	netclient.DefaultClient.MaxRetries = 0
	t.Cleanup(func() {
		netclient.DefaultClient.HTTPClient = origClient
		netclient.DefaultClient.MaxRetries = origRetries
	})
}

const claudeCacheUsageBody = `{"five_hour":{"utilization":42.5,"resets_at":"2026-09-25T00:00:00Z"},"seven_day":{"utilization":77.7}}`

var claudeTestCredential = sha256.Sum256([]byte("dummy-token"))

func TestClaudeUsageCoalescesConcurrentPolls(t *testing.T) {
	claudeCacheTestSetup(t)
	oldClient := netclient.DefaultClient.HTTPClient
	t.Cleanup(func() { netclient.DefaultClient.HTTPClient = oldClient })
	var calls atomic.Int32
	netclient.DefaultClient.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			calls.Add(1)
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(claudeCacheUsageBody))}, nil
		}),
	}
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if result := FetchClaudeUsage(t.Context()); result.Buckets["5h"] != "42.5" {
				t.Errorf("caller lost the fetched usage: %+v", result)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("overlapping polls made %d upstream calls, want one", calls.Load())
	}
}

func TestClaudeUsageCacheServesLastGoodOnRateLimit(t *testing.T) {
	claudeCacheTestSetup(t)
	calls := 0

	now := time.Now().Truncate(time.Second)
	lastChecked := now.Add(-3 * time.Minute)
	total := 3
	entry := claudeUsageCacheEntry{Credential: claudeTestCredential}
	entry.save(UsageResult{
		Used: "42.5", Buckets: map[string]string{"5h": "42.5", "7d": "77.7"},
		ResetCredits: &total,
		ResetCreditDetails: []ResetCreditDetail{
			{Scope: "Fable · Weekly", Count: 2, ExpiresAt: now.Add(time.Hour).Format(time.RFC3339)},
			{Count: 1, ExpiresAt: now.Add(-time.Minute).Format(time.RFC3339)},
		},
	}, lastChecked)

	claudeCacheTestClient(t, http.StatusTooManyRequests, "", &calls)
	limited := FetchClaudeUsage(t.Context())
	if limited.Status != "ok" {
		t.Fatalf("expected cached ok on 429, got %s (message=%s)", limited.Status, limited.Message)
	}
	if limited.AuthRequired {
		t.Fatal("rate limiting must not prompt a new login")
	}
	if limited.Source != "cache" {
		t.Fatalf("expected cache source, got %s", limited.Source)
	}
	if got := limited.Buckets["5h"]; got != "42.5" {
		t.Fatalf("expected cached 5h bucket 42.5, got %q", got)
	}
	if got := limited.Buckets["7d"]; got != "77.7" {
		t.Fatalf("expected cached 7d bucket 77.7, got %q", got)
	}
	if !strings.Contains(limited.Message, "API rate limited") {
		t.Fatalf("expected rate-limit reason in message, got %q", limited.Message)
	}
	if limited.ResetCredits == nil || *limited.ResetCredits != 2 ||
		len(limited.ResetCreditDetails) != 1 || limited.ResetCreditsCachedAt != lastChecked.UTC().Format(time.RFC3339) {
		t.Fatalf("429 should retain only active credits with the last confirmation time: %+v", limited)
	}
	again := FetchClaudeUsage(t.Context())
	if calls != 1 || again.ResetCreditsCachedAt != limited.ResetCreditsCachedAt {
		t.Fatalf("backoff retried or lost the last check time: calls=%d result=%+v", calls, again)
	}
}

func TestClaudeUsageCacheBackoffSkipsEndpoint(t *testing.T) {
	claudeCacheTestSetup(t)
	now := time.Now()
	entry := claudeUsageCacheEntry{Credential: claudeTestCredential}
	entry.save(UsageResult{Used: "12.0", Buckets: map[string]string{"5h": "12.0", "7d": "41.0"}}, now)
	entry.rateLimited(now, "")

	calls := 0
	claudeCacheSilentClient(t, &calls)

	result := FetchClaudeUsage(t.Context())
	if calls != 0 {
		t.Fatalf("expected no API calls during backoff, got %d", calls)
	}
	if result.Status != "ok" || result.Source != "cache" {
		t.Fatalf("expected ok/cache during backoff, got %s/%s", result.Status, result.Source)
	}
	if !strings.Contains(result.Message, "retrying in") {
		t.Fatalf("expected backoff reason in message, got %q", result.Message)
	}
}

func TestClaudeUsageCacheBackoffWithoutCachedUsage(t *testing.T) {
	claudeCacheTestSetup(t)
	entry := claudeUsageCacheEntry{Credential: claudeTestCredential}
	entry.rateLimited(time.Now(), "")

	calls := 0
	claudeCacheSilentClient(t, &calls)

	result := FetchClaudeUsage(t.Context())
	if calls != 0 {
		t.Fatalf("expected no API calls during backoff, got %d", calls)
	}
	if result.Status != "warn" || result.Used != "n/a" {
		t.Fatalf("expected warn/n/a during backoff without cache, got %s/%s", result.Status, result.Used)
	}
	if !strings.Contains(result.Message, "backing off") {
		t.Fatalf("expected backing-off message, got %q", result.Message)
	}
}

func TestClaudeUsageCacheDoesNotReuseAnotherCredential(t *testing.T) {
	claudeCacheTestSetup(t)
	total := 1
	entry := claudeUsageCacheEntry{Credential: claudeTestCredential}
	entry.save(UsageResult{Used: "12.0", ResetCredits: &total, ResetCreditDetails: []ResetCreditDetail{{Count: 1}}}, time.Now())
	entry.rateLimited(time.Now(), "3600")
	t.Setenv("CLAUDE_API_TOKEN", "different-token")
	calls := 0
	claudeCacheTestClient(t, http.StatusTooManyRequests, "", &calls)
	result := FetchClaudeUsage(t.Context())
	if calls != 1 || result.ResetCredits != nil || result.ResetCreditsCachedAt != "" {
		t.Fatalf("another credential reused cached data: calls=%d result=%+v", calls, result)
	}
	t.Setenv("CLAUDE_API_TOKEN", "dummy-token")
	result = FetchClaudeUsage(t.Context())
	if calls != 1 || result.ResetCredits == nil || *result.ResetCredits != 1 {
		t.Fatalf("another credential overwrote the original cache or backoff: calls=%d result=%+v", calls, result)
	}
}

func TestClaudeUsageCacheUpdatesInventoryOnlyWhenConfirmed(t *testing.T) {
	claudeCacheTestSetup(t)
	now := time.Now().Truncate(time.Second)
	checkedAt := now.Add(-7 * 24 * time.Hour)
	total := 1
	entry := claudeUsageCacheEntry{Credential: claudeTestCredential}
	result := UsageResult{Used: "12.0", ResetCredits: &total, ResetCreditDetails: []ResetCreditDetail{{Count: 1}}}
	for _, grants := range []string{
		`[{"resets_left":"1"}]`,
		`[{"resets_left":0},{"resets_left":1,"ends_at":"invalid"}]`,
	} {
		entry.save(result, checkedAt)
		entry.rateLimited(checkedAt, "30")
		calls := 0
		claudeCacheTestClient(t, http.StatusOK, `{"five_hour":{"utilization":14},"cedar_ember":{"eligible":true,"grants":`+grants+`}}`, &calls)
		got := FetchClaudeUsage(t.Context())
		if calls != 1 || got.Used != "14.0" || got.ResetCredits == nil || *got.ResetCredits != 1 || got.ResetCreditsCachedAt != checkedAt.UTC().Format(time.RFC3339) {
			t.Fatalf("incomplete live inventory must not erase the last confirmation: %+v", got)
		}
	}
	reloaded := readClaudeUsageCache(claudeTestCredential)
	cached, ok := reloaded.usage(UsageResult{}, now, "recent fetch")
	if !ok || cached.ResetCredits == nil || *cached.ResetCredits != 1 || cached.ResetCreditsCachedAt != checkedAt.UTC().Format(time.RFC3339) || reloaded.BackoffUntilMs != 0 {
		t.Fatalf("quota-only response should clear backoff without overwriting credit confirmation: %+v", cached)
	}
	creditCount, details := parseClaudeResetCredits([]byte(`{"eligible":true,"grants":[]}`), nil, now)
	reloaded.save(UsageResult{Used: "14.0", ResetCredits: creditCount, ResetCreditDetails: details}, now)
	cached = readClaudeUsageCache(claudeTestCredential).withCachedResets(UsageResult{}, now.Add(time.Second))
	if cached.ResetCredits == nil || *cached.ResetCredits != 0 || len(cached.ResetCreditDetails) != 0 {
		t.Fatalf("confirmed zero should replace the old grant: %+v", cached)
	}
}

func TestClaudeUsageCacheHonorsLongRetryAfter(t *testing.T) {
	claudeCacheTestSetup(t)
	now := time.Now().UTC().Truncate(time.Second)
	until := now.Add(time.Hour)
	for _, retryAfter := range []string{"3600", until.Format(http.TimeFormat)} {
		entry := claudeUsageCacheEntry{Credential: claudeTestCredential}
		entry.rateLimited(now, retryAfter)
		if got := readClaudeUsageCache(claudeTestCredential).BackoffUntilMs; got != until.UnixMilli() {
			t.Fatalf("Retry-After %q: got %v, want %v", retryAfter, time.UnixMilli(got), until)
		}
	}
}
