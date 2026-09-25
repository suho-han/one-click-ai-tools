package usage

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"
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

	restoreSecurity := mockSecurityCommand(t, "")
	defer restoreSecurity()

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

func TestClaudeUsageCacheServesLastGoodOnRateLimit(t *testing.T) {
	claudeCacheTestSetup(t)
	calls := 0

	claudeCacheTestClient(t, http.StatusOK, claudeCacheUsageBody, &calls)
	okResult := FetchClaudeUsage(t.Context())
	if okResult.Status != "ok" || okResult.Source != "oauth" {
		t.Fatalf("expected ok/oauth on first fetch, got %s/%s", okResult.Status, okResult.Source)
	}
	if calls != 1 {
		t.Fatalf("expected 1 API call, got %d", calls)
	}

	claudeCacheTestClient(t, http.StatusTooManyRequests, "", &calls)
	limited := FetchClaudeUsage(t.Context())
	if limited.Status != "ok" {
		t.Fatalf("expected cached ok on 429, got %s (message=%s)", limited.Status, limited.Message)
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
}

func TestClaudeUsageCacheBackoffSkipsEndpoint(t *testing.T) {
	claudeCacheTestSetup(t)
	now := time.Now()
	saveClaudeLastGoodUsage(
		map[string]string{"5h": "12.0", "7d": "41.0"},
		map[string]string{"5h": "2026-09-25T00:00:00Z"},
		now,
	)
	markClaudeRateLimited(now, 0)

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
	markClaudeRateLimited(time.Now(), 0)

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

func TestLastGoodClaudeUsageExpires(t *testing.T) {
	base := UsageResult{Provider: "claude-code"}
	staleEntry := claudeUsageCacheEntry{
		FetchedAtMs: time.Now().Add(-claudeUsageCacheMaxAge - time.Minute).UnixMilli(),
		Buckets:     map[string]string{"5h": "12.0"},
	}

	tmp := t.TempDir()
	origPath := claudeUsageCachePath
	claudeUsageCachePath = func() string { return filepath.Join(tmp, "claude-usage.json") }
	t.Cleanup(func() { claudeUsageCachePath = origPath })
	writeClaudeUsageCache(staleEntry)

	if _, ok := lastGoodClaudeUsage(base, time.Now(), "test"); ok {
		t.Fatalf("expected stale cache to be rejected")
	}

	freshEntry := staleEntry
	freshEntry.FetchedAtMs = time.Now().UnixMilli()
	writeClaudeUsageCache(freshEntry)
	if _, ok := lastGoodClaudeUsage(base, time.Now(), "test"); !ok {
		t.Fatalf("expected fresh cache to be served")
	}
}

func TestClaudeUsageCacheSuccessClearsBackoff(t *testing.T) {
	claudeCacheTestSetup(t)
	markClaudeRateLimited(time.Now(), 0)
	if _, ok := claudeUsageBackoffRemaining(time.Now()); !ok {
		t.Fatalf("expected backoff to be armed")
	}

	saveClaudeLastGoodUsage(map[string]string{"5h": "9.0"}, nil, time.Now())
	if _, ok := claudeUsageBackoffRemaining(time.Now()); ok {
		t.Fatalf("expected a successful fetch to clear backoff")
	}
}

func TestParseRetryAfter(t *testing.T) {
	if got := parseRetryAfter("30"); got != 30*time.Second {
		t.Fatalf("expected 30s, got %s", got)
	}
	for _, junk := range []string{"", "junk", "-5"} {
		if got := parseRetryAfter(junk); got != 0 {
			t.Fatalf("expected 0 for %q, got %s", junk, got)
		}
	}
}
