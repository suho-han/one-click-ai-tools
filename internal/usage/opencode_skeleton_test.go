package usage

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestParseOpenCodeUsageFromJSONL_RateLimitsShape(t *testing.T) {
	// Legacy test: the old local JSONL parsing has been replaced by remote API calls.
	// This test now verifies the API-based approach with a mock rate-limits-shaped response.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openCodeGoUsageResponse{
			Usage: map[string]openCodeGoWindowUsage{
				"rolling": {Status: "ok", Percent: 22.2, ResetsAt: "2026-08-17T21:00:00Z"},
				"weekly":  {Status: "ok", Percent: 66.6, ResetsAt: "2026-08-24T00:00:00Z"},
			},
		})
	}))
	defer server.Close()

	prev := os.Getenv("OPENCODE_API_KEY")
	t.Cleanup(func() { os.Setenv("OPENCODE_API_KEY", prev) })
	os.Setenv("OPENCODE_API_KEY", "test-key")

	prevEndpoint := openCodeGoUsageEndpoint
	openCodeGoUsageEndpoint = server.URL + "/zen/go/v1/usage"
	t.Cleanup(func() { openCodeGoUsageEndpoint = prevEndpoint })

	prevHome := userHomeDir
	tempHome := t.TempDir()
	userHomeDir = func() (string, error) { return tempHome, nil }
	t.Cleanup(func() { userHomeDir = prevHome })

	result := FetchOpenCodeUsage()
	if result.Status != "ok" {
		t.Fatalf("expected ok, got %q", result.Status)
	}
	if result.Buckets["5h"] != "22" {
		t.Fatalf("expected 5h=22, got %q", result.Buckets["5h"])
	}
	if result.Buckets["7d"] != "67" {
		t.Fatalf("expected 7d=67, got %q", result.Buckets["7d"])
	}
}

func TestParseOpenCodeUsageFromJSONL_FlatShape(t *testing.T) {
	// Legacy test: now verifies API approach with a flat response shape.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openCodeGoUsageResponse{
			Usage: map[string]openCodeGoWindowUsage{
				"rolling": {Status: "ok", Percent: 15.5, ResetsAt: "2026-08-17T21:00:00Z"},
				"weekly":  {Status: "ok", Percent: 45.0, ResetsAt: "2026-08-24T00:00:00Z"},
			},
		})
	}))
	defer server.Close()

	prev := os.Getenv("OPENCODE_API_KEY")
	t.Cleanup(func() { os.Setenv("OPENCODE_API_KEY", prev) })
	os.Setenv("OPENCODE_API_KEY", "test-key")

	prevEndpoint := openCodeGoUsageEndpoint
	openCodeGoUsageEndpoint = server.URL + "/zen/go/v1/usage"
	t.Cleanup(func() { openCodeGoUsageEndpoint = prevEndpoint })

	prevHome := userHomeDir
	tempHome := t.TempDir()
	userHomeDir = func() (string, error) { return tempHome, nil }
	t.Cleanup(func() { userHomeDir = prevHome })

	result := FetchOpenCodeUsage()
	if result.Status != "ok" {
		t.Fatalf("expected ok, got %q", result.Status)
	}
	if result.Buckets["5h"] != "16" {
		t.Fatalf("expected 5h=16, got %q", result.Buckets["5h"])
	}
	if result.Buckets["7d"] != "45" {
		t.Fatalf("expected 7d=45, got %q", result.Buckets["7d"])
	}
}

func TestFetchOpenCodeUsage_NoLogs(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	result := FetchOpenCodeUsage()
	if result.Provider != "opencode" {
		t.Fatalf("expected provider=opencode, got %s", result.Provider)
	}
	if result.Status != "warn" {
		t.Fatalf("expected status=warn, got %s", result.Status)
	}
	if result.Source != "remote" {
		t.Fatalf("expected source=remote, got %s", result.Source)
	}
	if !strings.HasPrefix(result.Message, "No data:") {
		t.Fatalf("expected no-data message, got %q", result.Message)
	}
	if result.Used != "0" {
		t.Fatalf("expected used=0, got %s", result.Used)
	}
}

func TestFetchOpenCodeUsage_FromLocalLogs(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	// Legacy test: now uses mock API server instead of local logs.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openCodeGoUsageResponse{
			Usage: map[string]openCodeGoWindowUsage{
				"rolling": {Status: "ok", Percent: 31.0, ResetsAt: "2026-08-17T21:00:00Z"},
				"weekly":  {Status: "ok", Percent: 78.0, ResetsAt: "2026-08-24T00:00:00Z"},
			},
		})
	}))
	defer server.Close()

	prev := os.Getenv("OPENCODE_API_KEY")
	t.Cleanup(func() { os.Setenv("OPENCODE_API_KEY", prev) })
	os.Setenv("OPENCODE_API_KEY", "test-key")

	prevEndpoint := openCodeGoUsageEndpoint
	openCodeGoUsageEndpoint = server.URL + "/zen/go/v1/usage"
	t.Cleanup(func() { openCodeGoUsageEndpoint = prevEndpoint })

	result := FetchOpenCodeUsage()
	if result.Status != "ok" {
		t.Fatalf("expected status=ok, got %s (%s)", result.Status, result.Message)
	}
	if result.Used != "31" {
		t.Fatalf("expected used=31, got %s", result.Used)
	}
	if result.Buckets["5h"] != "31" {
		t.Fatalf("expected 5h bucket=31, got %s", result.Buckets["5h"])
	}
	if result.Buckets["7d"] != "78" {
		t.Fatalf("expected 7d bucket=78, got %s", result.Buckets["7d"])
	}
}
