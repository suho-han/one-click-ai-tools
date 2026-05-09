package usage

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFetchCursorUsageRemote(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"used":42.5,"limit":100,"buckets":{"5h":42.5},"models":{"gpt-4.1":{"used":12.0}}}`)
	}))
	defer server.Close()

	prevEndpoint := os.Getenv("OCT_CURSOR_USAGE_URL")
	prevDebug := os.Getenv("OCT_USAGE_DEBUG")
	t.Cleanup(func() {
		_ = os.Setenv("OCT_CURSOR_USAGE_URL", prevEndpoint)
		_ = os.Setenv("OCT_USAGE_DEBUG", prevDebug)
	})

	_ = os.Setenv("OCT_CURSOR_USAGE_URL", server.URL)
	_ = os.Setenv("OCT_USAGE_DEBUG", "1")

	result := FetchCursorUsage()
	if result.Status != "ok" {
		t.Fatalf("expected ok status, got %q", result.Status)
	}
	if result.Source != "remote" {
		t.Fatalf("expected remote source, got %q", result.Source)
	}
	if result.Used != "42.5" {
		t.Fatalf("expected used=42.5, got %q", result.Used)
	}
	if result.Buckets["5h"] != "42.5" {
		t.Fatalf("expected 5h bucket, got %#v", result.Buckets)
	}
	if result.Buckets["model:gpt-4.1"] != "12.0" {
		t.Fatalf("expected normalized model bucket, got %#v", result.Buckets)
	}
	if !strings.Contains(result.SourceDetail, "gpt-4.1=12.0") {
		t.Fatalf("expected model detail, got %q", result.SourceDetail)
	}
}

func TestFetchCursorUsageLocalFallback(t *testing.T) {
	tempHome := t.TempDir()
	workspaceRoot := filepath.Join(tempHome, ".config", "Cursor", "User", "workspaceStorage")
	if runtime.GOOS == "windows" {
		workspaceRoot = filepath.Join(tempHome, "AppData", "Roaming", "Cursor", "User", "workspaceStorage")
	}
	workspaceDir := filepath.Join(workspaceRoot, "session-1")
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	prevEndpoint := os.Getenv("OCT_CURSOR_USAGE_URL")
	prevHome := os.Getenv("HOME")
	prevUserProfile := os.Getenv("USERPROFILE")
	prevAppData := os.Getenv("APPDATA")
	t.Cleanup(func() {
		_ = os.Setenv("OCT_CURSOR_USAGE_URL", prevEndpoint)
		_ = os.Setenv("HOME", prevHome)
		_ = os.Setenv("USERPROFILE", prevUserProfile)
		_ = os.Setenv("APPDATA", prevAppData)
	})

	_ = os.Setenv("OCT_CURSOR_USAGE_URL", "")
	_ = os.Setenv("HOME", tempHome)
	_ = os.Setenv("USERPROFILE", tempHome)
	if runtime.GOOS == "windows" {
		_ = os.Setenv("APPDATA", filepath.Join(tempHome, "AppData", "Roaming"))
	}

	result := FetchCursorUsage()
	if result.Source != "local" {
		t.Fatalf("expected local source, got %q", result.Source)
	}
	if result.Status != "warn" {
		t.Fatalf("expected warn status when remote endpoint is missing, got %q", result.Status)
	}
	if result.Used != "1" {
		t.Fatalf("expected local session estimate of 1, got %q", result.Used)
	}
	if !strings.Contains(result.Message, "reason=local_auth_missing") {
		t.Fatalf("expected standardized reason, got %q", result.Message)
	}
	if !strings.Contains(result.Message, "workspace storage") {
		t.Fatalf("expected workspace storage message, got %q", result.Message)
	}
}

func TestFetchCursorUsageLocalAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"gpt-4":{"numRequestsTotal":10,"maxRequestUsage":500},"claude-3-5-sonnet":{"numRequestsTotal":5,"maxRequestUsage":null},"startOfMonth":"2026-05-01T00:00:00.000Z"}`)
	}))
	defer server.Close()

	tempHome := t.TempDir()
	authDir := filepath.Join(tempHome, ".config", "cursor")
	if err := os.MkdirAll(authDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	authData := `{"accessToken":"test-token","refreshToken":"test-refresh"}`
	if err := os.WriteFile(filepath.Join(authDir, "auth.json"), []byte(authData), 0600); err != nil {
		t.Fatalf("write auth.json failed: %v", err)
	}

	prevEndpoint := os.Getenv("OCT_CURSOR_USAGE_URL")
	prevAPIURL := os.Getenv("OCT_CURSOR_API_USAGE_URL")
	prevHome := os.Getenv("HOME")
	prevDebug := os.Getenv("OCT_USAGE_DEBUG")
	t.Cleanup(func() {
		_ = os.Setenv("OCT_CURSOR_USAGE_URL", prevEndpoint)
		_ = os.Setenv("OCT_CURSOR_API_USAGE_URL", prevAPIURL)
		_ = os.Setenv("HOME", prevHome)
		_ = os.Setenv("OCT_USAGE_DEBUG", prevDebug)
	})

	_ = os.Setenv("OCT_CURSOR_USAGE_URL", "")
	_ = os.Setenv("OCT_CURSOR_API_USAGE_URL", server.URL)
	_ = os.Setenv("HOME", tempHome)
	_ = os.Setenv("OCT_USAGE_DEBUG", "1")

	result := FetchCursorUsage()
	if result.Status != "ok" {
		t.Fatalf("expected ok status, got %q (msg: %s)", result.Status, result.Message)
	}
	if result.Source != "local-auth" {
		t.Fatalf("expected local-auth source, got %q", result.Source)
	}
	if result.Used != "15" {
		t.Fatalf("expected used=15 (10+5), got %q", result.Used)
	}
	if result.Limit != "500" {
		t.Fatalf("expected limit=500, got %q", result.Limit)
	}
	if result.Unit != "requests" {
		t.Fatalf("expected unit=requests, got %q", result.Unit)
	}
	if !strings.Contains(result.Period, "2026-05") {
		t.Fatalf("expected period to contain 2026-05, got %q", result.Period)
	}
	if result.Buckets["gpt-4"] != "10" {
		t.Fatalf("expected gpt-4 bucket=10, got %#v", result.Buckets)
	}
	if result.Buckets["claude-3-5-sonnet"] != "5" {
		t.Fatalf("expected claude-3-5-sonnet bucket=5, got %#v", result.Buckets)
	}
	if !strings.Contains(result.SourceDetail, "gpt-4=10") {
		t.Fatalf("expected gpt-4=10 in SourceDetail, got %q", result.SourceDetail)
	}
}

func TestFetchCursorUsageRemoteFailureFallsBackWithReason(t *testing.T) {
	prevEndpoint := os.Getenv("OCT_CURSOR_USAGE_URL")
	t.Cleanup(func() {
		_ = os.Setenv("OCT_CURSOR_USAGE_URL", prevEndpoint)
	})

	_ = os.Setenv("OCT_CURSOR_USAGE_URL", "http://127.0.0.1:1/unreachable")
	result := FetchCursorUsage()

	if result.Source != "local" {
		t.Fatalf("expected local fallback source, got %q", result.Source)
	}
	if result.Status != "warn" {
		t.Fatalf("expected warn on remote failure, got %q", result.Status)
	}
	if !strings.Contains(result.Message, "reason=remote_request_failed") {
		t.Fatalf("expected standardized failure reason, got %q", result.Message)
	}
}
