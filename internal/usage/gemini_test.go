package usage

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCountAntigravitySessionsCountsSupportedArtifacts(t *testing.T) {
	root := t.TempDir()
	pathA := filepath.Join(root, "conversations")
	pathB := filepath.Join(root, "cache")
	if err := os.MkdirAll(pathA, 0o755); err != nil {
		t.Fatalf("mkdir pathA failed: %v", err)
	}
	if err := os.MkdirAll(pathB, 0o755); err != nil {
		t.Fatalf("mkdir pathB failed: %v", err)
	}
	for _, rel := range []string{"chat-1.pb", "chat-2.db", "notes.jsonl"} {
		if err := os.WriteFile(filepath.Join(pathA, rel), []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s failed: %v", rel, err)
		}
	}
	if err := os.Mkdir(filepath.Join(pathB, "project-1"), 0o755); err != nil {
		t.Fatalf("mkdir project dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pathB, "ignore.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write ignore.txt failed: %v", err)
	}

	count, matched := countAntigravitySessions([]string{pathA, pathB})
	if count != 4 {
		t.Fatalf("expected 4 artifacts, got %d", count)
	}
	if len(matched) != 2 {
		t.Fatalf("expected 2 matched paths, got %v", matched)
	}
}

func TestFetchAntigravityUsageUsesQuotaAPI(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	credsDir := filepath.Join(tmp, ".gemini")
	if err := os.MkdirAll(credsDir, 0o755); err != nil {
		t.Fatalf("mkdir creds failed: %v", err)
	}
	creds := fmt.Sprintf(`{"access_token":"test-token","refresh_token":"refresh","expiry_date":%d}`, time.Now().Add(time.Hour).UnixMilli())
	if err := os.WriteFile(filepath.Join(credsDir, "oauth_creds.json"), []byte(creds), 0o600); err != nil {
		t.Fatalf("write creds failed: %v", err)
	}

	projectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("project Authorization header did not contain test bearer token")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"cloudaicompanionProject":"project-1"}`)
	}))
	defer projectServer.Close()
	quotaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("quota Authorization header did not contain test bearer token")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"buckets":[{"remainingFraction":0.6,"resetTime":"2026-07-22T00:00:00Z","modelId":"gemini-2.5-pro"}]}`)
	}))
	defer quotaServer.Close()
	t.Setenv("OCT_GEMINI_API_ENDPOINT", projectServer.URL)
	t.Setenv("OCT_GEMINI_USAGE_ENDPOINT", quotaServer.URL)

	result := FetchAntigravityUsage()
	if result.Source != "quota" {
		t.Fatalf("expected quota source, got %q", result.Source)
	}
	if result.Used != "40.0" {
		t.Fatalf("expected 40.0 used percent, got %q", result.Used)
	}
	if result.Buckets["model:Pro"] != "40.0" {
		t.Fatalf("expected Pro model bucket 40.0, got %q", result.Buckets["model:Pro"])
	}
}

func TestFetchAntigravityLocalUsageNoHome(t *testing.T) {
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() {
		if oldHome == "" {
			_ = os.Unsetenv("HOME")
			return
		}
		_ = os.Setenv("HOME", oldHome)
	})
	if err := os.Unsetenv("HOME"); err != nil {
		t.Fatalf("unset HOME failed: %v", err)
	}

	result := FetchAntigravityLocalUsage()
	if result.Provider != "antigravity" {
		t.Fatalf("expected provider antigravity, got %q", result.Provider)
	}
	if result.Status != "warn" {
		t.Fatalf("expected warn status, got %q", result.Status)
	}
}

func TestFetchGeminiUsageDelegatesToAntigravity(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	result := FetchGeminiUsage()
	if result.Provider != "antigravity" {
		t.Fatalf("expected provider antigravity, got %q", result.Provider)
	}
	if !strings.EqualFold(result.Source, "local") {
		t.Fatalf("expected local source, got %q", result.Source)
	}
}
