package usage

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/suho-han/one-click-ai-tools/internal/netclient"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestFetchClaudeUsageBuckets(t *testing.T) {
	oldClient := netclient.DefaultClient.HTTPClient
	oldRetries := netclient.DefaultClient.MaxRetries

	t.Setenv("CLAUDE_API_TOKEN", "dummy-token")

	netclient.DefaultClient.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://api.anthropic.com/api/oauth/usage" {
				t.Fatalf("unexpected URL: %s", req.URL.String())
			}
			if got := req.Header.Get("Authorization"); got == "" || !strings.HasPrefix(got, "Bearer ") {
				t.Fatalf("missing or invalid auth header: %s", got)
			}

			body := `{"five_hour":{"utilization":42.5},"seven_day":{"utilization":77.7}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	}
	netclient.DefaultClient.MaxRetries = 0
	defer func() {
		netclient.DefaultClient.HTTPClient = oldClient
		netclient.DefaultClient.MaxRetries = oldRetries
	}()

	result := FetchClaudeUsage()

	if result.Status != "ok" {
		t.Fatalf("expected status ok, got %s (message=%s)", result.Status, result.Message)
	}
	if result.Used != "42.5" {
		t.Fatalf("expected used=42.5 from 5h bucket, got %s", result.Used)
	}
	if result.Buckets == nil {
		t.Fatalf("expected buckets map to be populated")
	}
	if got := result.Buckets["5h"]; got != "42.5" {
		t.Fatalf("expected 5h bucket 42.5, got %s", got)
	}
	if got := result.Buckets["7d"]; got != "77.7" {
		t.Fatalf("expected 7d bucket 77.7, got %s", got)
	}
}

func TestFetchClaudeUsageRateLimitedBuckets(t *testing.T) {
	oldClient := netclient.DefaultClient.HTTPClient
	oldRetries := netclient.DefaultClient.MaxRetries

	t.Setenv("CLAUDE_API_TOKEN", "dummy-token")
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	cache := fmt.Sprintf(`{
		"cachedUsageUtilization": {
			"fetchedAtMs": 1784687082111,
			"utilization": {
				"five_hour": {"utilization": 12, "resets_at": "2026-07-22T06:09:59Z"},
				"seven_day": {"utilization": 41, "resets_at": "2026-07-24T22:59:59Z"}
			}
		}
	}`)
	if err := os.WriteFile(filepath.Join(tmp, ".claude.json"), []byte(cache), 0o600); err != nil {
		t.Fatalf("write cache failed: %v", err)
	}

	netclient.DefaultClient.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}
	netclient.DefaultClient.MaxRetries = 0
	defer func() {
		netclient.DefaultClient.HTTPClient = oldClient
		netclient.DefaultClient.MaxRetries = oldRetries
	}()

	result := FetchClaudeUsage()
	if result.Status != "ok" {
		t.Fatalf("expected status ok, got %s", result.Status)
	}
	if result.Source != "cache" {
		t.Fatalf("expected cache source, got %s", result.Source)
	}
	if result.Used != "12.0" {
		t.Fatalf("expected used=12.0 from cache, got %s", result.Used)
	}
	if got := result.Buckets["5h"]; got != "12.0" {
		t.Fatalf("expected 5h bucket 12.0, got %s", got)
	}
	if got := result.Buckets["7d"]; got != "41.0" {
		t.Fatalf("expected 7d bucket 41.0, got %s", got)
	}
}
