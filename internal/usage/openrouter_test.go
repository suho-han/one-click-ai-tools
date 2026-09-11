package usage

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchOpenRouterUsageSpend(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "sk-or-v1-test")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-or-v1-test" {
			t.Errorf("Authorization = %q, want bearer key", got)
		}
		w.Write([]byte(`{"data":{"usage":25.5,"usage_daily":1.25,"usage_weekly":3.2,"usage_monthly":12.8,` +
			`"limit":100,"limit_remaining":74.5,"limit_reset":"monthly","is_free_tier":false}}`))
	}))
	defer server.Close()

	t.Setenv("OCT_OPENROUTER_USAGE_ENDPOINT", server.URL)
	result := FetchOpenRouterUsage(t.Context())

	if result.Status != "ok" {
		t.Fatalf("status = %q, message = %q", result.Status, result.Message)
	}
	if result.Used != "25.50" {
		t.Errorf("used = %q, want 25.50", result.Used)
	}
	if result.Limit != "100.00" {
		t.Errorf("limit = %q, want 100.00", result.Limit)
	}
	if result.Buckets["7d"] != "3.20" || result.Buckets["1m"] != "12.80" {
		t.Errorf("buckets = %v, want 7d 3.20 / 1m 12.80", result.Buckets)
	}
	if result.Plan != "paid" {
		t.Errorf("plan = %q, want paid", result.Plan)
	}
	if !strings.Contains(result.Message, "74.50 remaining") {
		t.Errorf("message = %q, want remaining amount", result.Message)
	}
	if result.Unit != "USD" {
		t.Errorf("unit = %q, want USD", result.Unit)
	}
}

func TestFetchOpenRouterUsageNullLimit(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "sk-or-v1-test")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{"usage":2.0,"usage_weekly":2.0,"usage_monthly":2.0,"limit":null,"is_free_tier":true}}`))
	}))
	defer server.Close()

	t.Setenv("OCT_OPENROUTER_USAGE_ENDPOINT", server.URL)
	result := FetchOpenRouterUsage(t.Context())

	if result.Status != "ok" {
		t.Fatalf("status = %q, message = %q", result.Status, result.Message)
	}
	if result.Limit != "" {
		t.Errorf("limit = %q, want empty for null limit", result.Limit)
	}
	if result.Plan != "free" {
		t.Errorf("plan = %q, want free", result.Plan)
	}
	if strings.Contains(result.Message, "remaining") {
		t.Errorf("message = %q, want no remaining note when absent", result.Message)
	}
}

func TestFetchOpenRouterUsageWithoutKey(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")

	result := FetchOpenRouterUsage(t.Context())
	if result.Status != "warn" {
		t.Fatalf("status = %q, want warn", result.Status)
	}
	if !strings.Contains(result.Message, "OPENROUTER_API_KEY") {
		t.Errorf("message = %q, want key guidance", result.Message)
	}
}

func TestFetchOpenRouterUsageForbiddenForManagementOperation(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "sk-or-v1-test")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":{"message":"Only management keys can perform this operation"}}`))
	}))
	defer server.Close()

	t.Setenv("OCT_OPENROUTER_USAGE_ENDPOINT", server.URL)
	result := FetchOpenRouterUsage(t.Context())

	if result.Status != "error" {
		t.Fatalf("status = %q, want error", result.Status)
	}
	if !strings.Contains(result.Message, "403") {
		t.Errorf("message = %q, want 403 excerpt", result.Message)
	}
}
