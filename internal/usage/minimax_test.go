package usage

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const minimaxRemainsPayload = `{
  "current_interval_total_count": 200,
  "current_interval_usage_count": "139",
  "end_time": 1767225600000,
  "current_weekly_total_count": 2048,
  "current_weekly_usage_count": 214,
  "weekly_end_time": 1767312000
}`

func TestFetchMinimaxUsageMapsWindows(t *testing.T) {
	t.Setenv("MINIMAX_CODING_API_KEY", "sk-cp-test")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-cp-test" {
			t.Errorf("Authorization = %q, want bearer sk-cp-test", got)
		}
		w.Write([]byte(minimaxRemainsPayload))
	}))
	defer server.Close()

	t.Setenv("OCT_MINIMAX_USAGE_ENDPOINT", server.URL)
	result := FetchMinimaxUsage(t.Context())

	if result.Status != "ok" {
		t.Fatalf("status = %q, message = %q", result.Status, result.Message)
	}
	// 139/200 -> 70%, 214/2048 -> 10% (rounded).
	if result.Buckets["5h"] != "70" {
		t.Errorf("5h bucket = %q, want 70", result.Buckets["5h"])
	}
	if result.Buckets["7d"] != "10" {
		t.Errorf("7d bucket = %q, want 10", result.Buckets["7d"])
	}
	if result.Used != "10" {
		t.Errorf("used = %q, want 10 (weekly primary)", result.Used)
	}
	if result.BucketResets["5h"] == "" || result.BucketResets["7d"] == "" {
		t.Errorf("resets = %v, want both windows' reset times", result.BucketResets)
	}
}

func TestFetchMinimaxUsageWrappedInData(t *testing.T) {
	t.Setenv("MINIMAX_CODING_API_KEY", "sk-cp-test")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{"current_interval_total_count":100,"current_interval_usage_count":50}}`))
	}))
	defer server.Close()

	t.Setenv("OCT_MINIMAX_USAGE_ENDPOINT", server.URL)
	result := FetchMinimaxUsage(t.Context())

	if result.Status != "ok" {
		t.Fatalf("status = %q, message = %q", result.Status, result.Message)
	}
	if result.Buckets["5h"] != "50" {
		t.Errorf("5h bucket = %q, want 50", result.Buckets["5h"])
	}
}

func TestFetchMinimaxUsageExhaustedWindowWarns(t *testing.T) {
	t.Setenv("MINIMAX_CODING_API_KEY", "sk-cp-test")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"current_interval_total_count":100,"current_interval_usage_count":100,` +
			`"current_weekly_total_count":2048,"current_weekly_usage_count":10}`))
	}))
	defer server.Close()

	t.Setenv("OCT_MINIMAX_USAGE_ENDPOINT", server.URL)
	result := FetchMinimaxUsage(t.Context())

	if result.Status != "warn" {
		t.Fatalf("status = %q, want warn for exhausted window", result.Status)
	}
	if !strings.Contains(result.Message, "exhausted") {
		t.Errorf("message = %q, want exhausted note", result.Message)
	}
}

func TestFetchMinimaxUsageWithoutToken(t *testing.T) {
	t.Setenv("MINIMAX_CODING_API_KEY", "")
	t.Setenv("MINIMAX_API_KEY", "")

	result := FetchMinimaxUsage(t.Context())
	if result.Status != "warn" {
		t.Fatalf("status = %q, want warn", result.Status)
	}
	if !strings.Contains(result.Message, "MINIMAX_CODING_API_KEY") {
		t.Errorf("message = %q, want key guidance", result.Message)
	}
}

func TestResolveMinimaxTokenPriority(t *testing.T) {
	t.Setenv("MINIMAX_CODING_API_KEY", "coding-key")
	t.Setenv("MINIMAX_API_KEY", "plain-key")
	token, source := resolveMinimaxToken()
	if token != "coding-key" || source != "env:MINIMAX_CODING_API_KEY" {
		t.Fatalf("token/source = %q/%q", token, source)
	}

	t.Setenv("MINIMAX_CODING_API_KEY", "")
	token, source = resolveMinimaxToken()
	if token != "plain-key" || source != "env:MINIMAX_API_KEY" {
		t.Fatalf("fallback token/source = %q/%q", token, source)
	}
}

func TestMinimaxRegionBaseURL(t *testing.T) {
	t.Setenv("MINIMAX_REGION", "")
	if got := minimaxBaseURL(); got != "https://api.minimax.io" {
		t.Errorf("base = %q, want global host", got)
	}

	t.Setenv("MINIMAX_REGION", "cn")
	if got := minimaxBaseURL(); got != "https://api.minimaxi.com" {
		t.Errorf("base = %q, want China host", got)
	}
}
