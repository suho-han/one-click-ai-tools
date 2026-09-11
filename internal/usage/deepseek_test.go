package usage

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchDeepseekUsageBalance(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "sk-deepseek")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-deepseek" {
			t.Errorf("Authorization = %q, want bearer sk-deepseek", got)
		}
		w.Write([]byte(`{"is_available":true,"balance_infos":[` +
			`{"currency":"USD","total_balance":"110.00","granted_balance":"10.00","topped_up_balance":"100.00"},` +
			`{"currency":"CNY","total_balance":"20.50","granted_balance":"0.00","topped_up_balance":"20.50"}]}`))
	}))
	defer server.Close()

	t.Setenv("OCT_DEEPSEEK_BALANCE_ENDPOINT", server.URL)
	result := FetchDeepseekUsage(t.Context())

	if result.Status != "ok" {
		t.Fatalf("status = %q, message = %q", result.Status, result.Message)
	}
	if result.Used != "110.00" {
		t.Errorf("used = %q, want 110.00", result.Used)
	}
	if result.Unit != "USD" {
		t.Errorf("unit = %q, want USD", result.Unit)
	}
	if !strings.Contains(result.Message, "20.50 CNY") {
		t.Errorf("message = %q, want extra currency summary", result.Message)
	}
}

func TestFetchDeepseekUsageAccountUnavailable(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "sk-deepseek")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"is_available":false,"balance_infos":[{"currency":"USD","total_balance":"0.00"}]}`))
	}))
	defer server.Close()

	t.Setenv("OCT_DEEPSEEK_BALANCE_ENDPOINT", server.URL)
	result := FetchDeepseekUsage(t.Context())

	if result.Status != "warn" {
		t.Fatalf("status = %q, want warn when account unavailable", result.Status)
	}
	if !strings.Contains(result.Message, "unavailable") {
		t.Errorf("message = %q, want unavailable note", result.Message)
	}
}

func TestFetchDeepseekUsageWithoutKey(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "")

	result := FetchDeepseekUsage(t.Context())
	if result.Status != "warn" {
		t.Fatalf("status = %q, want warn", result.Status)
	}
	if !strings.Contains(result.Message, "DEEPSEEK_API_KEY") {
		t.Errorf("message = %q, want key guidance", result.Message)
	}
}

func TestFetchDeepseekUsageHTTPError(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "sk-deepseek")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"auth failed"}}`))
	}))
	defer server.Close()

	t.Setenv("OCT_DEEPSEEK_BALANCE_ENDPOINT", server.URL)
	result := FetchDeepseekUsage(t.Context())

	if result.Status != "error" {
		t.Fatalf("status = %q, want error", result.Status)
	}
	if !strings.Contains(result.Message, "401") {
		t.Errorf("message = %q, want 401 excerpt", result.Message)
	}
}
