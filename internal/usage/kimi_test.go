package usage

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const kimiUsagePayload = `{
  "usage": {"limit": "2048", "used": "214", "remaining": "1834", "resetTime": "2026-09-15T00:00:00Z"},
  "limits": [
    {"window": {"duration": 300, "timeUnit": "TIME_UNIT_MINUTE"},
     "detail": {"limit": "200", "used": "139", "remaining": "61", "resetTime": "2026-09-12T18:00:00Z"}}
  ],
  "planName": "Moderato"
}`

func TestFetchKimiUsageMapsWindows(t *testing.T) {
	t.Setenv("KIMI_CODE_API_KEY", "test-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want bearer test-token", got)
		}
		w.Write([]byte(kimiUsagePayload))
	}))
	defer server.Close()

	orig := kimiUsageEndpoint
	kimiUsageEndpoint = server.URL
	t.Cleanup(func() { kimiUsageEndpoint = orig })

	result := FetchKimiUsage(t.Context())

	if result.Status != "ok" {
		t.Fatalf("status = %q, message = %q", result.Status, result.Message)
	}
	if result.Plan != "Moderato" {
		t.Errorf("plan = %q, want Moderato", result.Plan)
	}
	if result.Buckets["7d"] != "214" {
		t.Errorf("weekly bucket = %q, want 214", result.Buckets["7d"])
	}
	if result.Buckets["5h"] != "139" {
		t.Errorf("5h bucket = %q, want 139", result.Buckets["5h"])
	}
	if result.BucketResets["5h"] != "2026-09-12T18:00:00Z" {
		t.Errorf("5h reset = %q", result.BucketResets["5h"])
	}
	if result.Used != "214" || result.Limit != "2048" {
		t.Errorf("used/limit = %q/%q, want 214/2048", result.Used, result.Limit)
	}
	if result.Unit != "req" {
		t.Errorf("unit = %q, want req", result.Unit)
	}
}

func TestFetchKimiUsageWithoutToken(t *testing.T) {
	t.Setenv("KIMI_CODE_API_KEY", "")
	origHome := userHomeDir
	userHomeDir = func() (string, error) { return t.TempDir(), nil }
	t.Cleanup(func() { userHomeDir = origHome })

	result := FetchKimiUsage(t.Context())
	if result.Status != "warn" {
		t.Fatalf("status = %q, want warn", result.Status)
	}
	if !strings.Contains(result.Message, "kimi login") {
		t.Errorf("message = %q, want login guidance", result.Message)
	}
}

func TestResolveKimiTokenFromCredentialFile(t *testing.T) {
	t.Setenv("KIMI_CODE_API_KEY", "")
	home := t.TempDir()
	t.Setenv("KIMI_CODE_HOME", home)
	if err := os.MkdirAll(filepath.Join(home, "credentials"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "credentials", "kimi-code.json"), []byte(`{"access_token":"cli-token","refresh_token":"r","expires_at":99999999999}`), 0o600); err != nil {
		t.Fatal(err)
	}

	token, source := resolveKimiToken()
	if token != "cli-token" {
		t.Errorf("token = %q, want cli-token", token)
	}
	if source != "kimi-code.json" {
		t.Errorf("source = %q, want kimi-code.json", source)
	}
}

func TestResolveKimiTokenPrefersEnv(t *testing.T) {
	t.Setenv("KIMI_CODE_API_KEY", "env-token")
	t.Setenv("KIMI_CODE_HOME", t.TempDir())

	token, source := resolveKimiToken()
	if token != "env-token" || source != "env:KIMI_CODE_API_KEY" {
		t.Errorf("token/source = %q/%q, want env-token/env:KIMI_CODE_API_KEY", token, source)
	}
}

func TestFetchKimiUsageHTTPError(t *testing.T) {
	t.Setenv("KIMI_CODE_API_KEY", "test-token")
	t.Setenv("KIMI_CODE_HOME", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid token"}`))
	}))
	defer server.Close()

	orig := kimiUsageEndpoint
	kimiUsageEndpoint = server.URL
	t.Cleanup(func() { kimiUsageEndpoint = orig })

	result := FetchKimiUsage(t.Context())
	if result.Status != "error" {
		t.Fatalf("status = %q, want error", result.Status)
	}
	if !strings.Contains(result.Message, "401") {
		t.Errorf("message = %q, want 401 excerpt", result.Message)
	}
}
