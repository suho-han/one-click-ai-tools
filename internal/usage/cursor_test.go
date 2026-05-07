package usage

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	if !strings.Contains(result.SourceDetail, "gpt-4.1=12.0") {
		t.Fatalf("expected model detail, got %q", result.SourceDetail)
	}
}

func TestFetchCursorUsageLocalFallback(t *testing.T) {
	tempHome := t.TempDir()
	workspaceDir := filepath.Join(tempHome, ".config", "Cursor", "User", "workspaceStorage", "session-1")
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	prevEndpoint := os.Getenv("OCT_CURSOR_USAGE_URL")
	prevHome := os.Getenv("HOME")
	t.Cleanup(func() {
		_ = os.Setenv("OCT_CURSOR_USAGE_URL", prevEndpoint)
		_ = os.Setenv("HOME", prevHome)
	})

	_ = os.Setenv("OCT_CURSOR_USAGE_URL", "")
	_ = os.Setenv("HOME", tempHome)

	result := FetchCursorUsage()
	if result.Source != "local" {
		t.Fatalf("expected local source, got %q", result.Source)
	}
	if result.Used != "1" {
		t.Fatalf("expected local session estimate of 1, got %q", result.Used)
	}
	if !strings.Contains(result.Message, "workspace storage") {
		t.Fatalf("expected workspace storage message, got %q", result.Message)
	}
}
