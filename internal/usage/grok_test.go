package usage

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFetchGrokUsageBillingAndTier(t *testing.T) {
	t.Setenv("GROK_OAUTH_TOKEN", "grok-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer grok-token" {
			t.Errorf("Authorization = %q, want bearer grok-token", got)
		}
		if got := r.Header.Get("x-xai-token-auth"); got != "xai-grok-cli" {
			t.Errorf("x-xai-token-auth = %q, want xai-grok-cli", got)
		}
		switch r.URL.Path {
		case "/v1/billing":
			if r.URL.RawQuery != "format=credits" {
				t.Errorf("query = %q, want format=credits", r.URL.RawQuery)
			}
			w.Write([]byte(`{"config":{"creditUsagePercent":42,"currentPeriod":{"end":"2026-10-01T00:00:00Z"}}}`))
		case "/v1/settings":
			w.Write([]byte(`{"subscription_tier_display":"SuperGrok"}`))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OCT_GROK_USAGE_ENDPOINT", server.URL+"/v1/billing?format=credits")
	t.Setenv("OCT_GROK_SETTINGS_ENDPOINT", server.URL+"/v1/settings")
	result := FetchGrokUsage(t.Context())

	if result.Status != "ok" {
		t.Fatalf("status = %q, message = %q", result.Status, result.Message)
	}
	if result.Buckets["1m"] != "42" {
		t.Errorf("1m bucket = %q, want 42", result.Buckets["1m"])
	}
	if result.Used != "42" {
		t.Errorf("used = %q, want 42", result.Used)
	}
	if result.BucketResets["1m"] != "2026-10-01T00:00:00Z" {
		t.Errorf("reset = %q", result.BucketResets["1m"])
	}
	if result.Plan != "SuperGrok" {
		t.Errorf("plan = %q, want SuperGrok", result.Plan)
	}
	if result.Unit != "percent" || result.Period != "1m" {
		t.Errorf("unit/period = %q/%q, want percent/1m", result.Unit, result.Period)
	}
}

func TestFetchGrokUsageSettingsFailureDoesNotFailRow(t *testing.T) {
	t.Setenv("GROK_OAUTH_TOKEN", "grok-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/billing":
			w.Write([]byte(`{"config":{"creditUsagePercent":10,"billingPeriodEnd":"2026-10-01T00:00:00Z"}}`))
		case "/v1/settings":
			// 403 (not 5xx) so netclient's retry backoff doesn't slow the test.
			w.WriteHeader(http.StatusForbidden)
		}
	}))
	defer server.Close()

	t.Setenv("OCT_GROK_USAGE_ENDPOINT", server.URL+"/v1/billing")
	t.Setenv("OCT_GROK_SETTINGS_ENDPOINT", server.URL+"/v1/settings")
	result := FetchGrokUsage(t.Context())

	if result.Status != "ok" {
		t.Fatalf("status = %q, want ok despite settings failure", result.Status)
	}
	if result.Buckets["1m"] != "10" {
		t.Errorf("1m bucket = %q, want 10", result.Buckets["1m"])
	}
	if result.BucketResets["1m"] != "2026-10-01T00:00:00Z" {
		t.Errorf("billingPeriodEnd fallback reset = %q", result.BucketResets["1m"])
	}
}

func TestResolveGrokTokenFromAuthFile(t *testing.T) {
	t.Setenv("GROK_OAUTH_TOKEN", "")
	grokHome := t.TempDir()
	t.Setenv("GROK_HOME", grokHome)
	authJSON := `{
	  "https://auth.x.ai::client-abc": {"key": "preferred-token", "refresh_token": "r"},
	  "https://accounts.x.ai/sign-in": {"key": "fallback-token"}
	}`
	if err := os.WriteFile(filepath.Join(grokHome, "auth.json"), []byte(authJSON), 0o600); err != nil {
		t.Fatal(err)
	}

	token, source := resolveGrokToken()
	if token != "preferred-token" {
		t.Errorf("token = %q, want preferred-token", token)
	}
	if source != "grok:auth.json" {
		t.Errorf("source = %q", source)
	}

	// Without the preferred scope, the fallback scope is used.
	if err := os.WriteFile(filepath.Join(grokHome, "auth.json"), []byte(`{"https://accounts.x.ai/sign-in":{"key":"fallback-token"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	token, _ = resolveGrokToken()
	if token != "fallback-token" {
		t.Errorf("fallback token = %q, want fallback-token", token)
	}
}

func TestFetchGrokUsageWithoutToken(t *testing.T) {
	t.Setenv("GROK_OAUTH_TOKEN", "")
	origHome := userHomeDir
	userHomeDir = func() (string, error) { return t.TempDir(), nil }
	t.Cleanup(func() { userHomeDir = origHome })

	result := FetchGrokUsage(t.Context())
	if result.Status != "warn" {
		t.Fatalf("status = %q, want warn", result.Status)
	}
	if !strings.Contains(result.Message, "grok login") {
		t.Errorf("message = %q, want login guidance", result.Message)
	}
}

func TestFetchGrokUsageSlowSettingsDoesNotBlockRow(t *testing.T) {
	t.Setenv("GROK_OAUTH_TOKEN", "grok-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/billing":
			w.Write([]byte(`{"config":{"creditUsagePercent":42}}`))
		case "/v1/settings":
			time.Sleep(200 * time.Millisecond)
			w.Write([]byte(`{"subscription_tier_display":"SuperGrok"}`))
		}
	}))
	defer server.Close()

	origTimeout := grokSettingsTimeout
	grokSettingsTimeout = 10 * time.Millisecond
	t.Cleanup(func() { grokSettingsTimeout = origTimeout })

	t.Setenv("OCT_GROK_USAGE_ENDPOINT", server.URL+"/v1/billing?format=credits")
	t.Setenv("OCT_GROK_SETTINGS_ENDPOINT", server.URL+"/v1/settings")
	result := FetchGrokUsage(t.Context())

	if result.Status != "ok" {
		t.Fatalf("status = %q, message = %q", result.Status, result.Message)
	}
	if result.Used != "42" {
		t.Errorf("used = %q, want 42", result.Used)
	}
	if result.Plan != "" {
		t.Errorf("plan = %q, want empty (settings exceeded its own budget)", result.Plan)
	}
}
