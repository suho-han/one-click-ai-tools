package usage

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func writeCodexAuthForReset(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("CODEX_HOME", filepath.Join(tmp, ".codex"))
	codexDir := filepath.Join(tmp, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(codexDir, "auth.json"), []byte(`{"tokens":{"access_token":"test-token","account_id":"acct-1"}}`), 0o600); err != nil {
		t.Fatalf("write auth failed: %v", err)
	}
}

func TestFetchCodexResetCreditsListsAvailable(t *testing.T) {
	writeCodexAuthForReset(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rate-limit-reset-credits" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("missing bearer token")
		}
		if r.Header.Get("ChatGPT-Account-Id") != "acct-1" {
			t.Fatalf("missing account id header")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"credits":[{"id":"RateLimitResetCredit_abc","status":"available","title":"Full reset","expires_at":"2026-10-29T19:40:37.242861Z"}],"available_count":2}`)
	}))
	defer server.Close()
	t.Setenv("OCT_CODEX_USAGE_ENDPOINT", server.URL+"/usage")

	credits, err := FetchCodexResetCredits(t.Context(), "")
	if err != nil {
		t.Fatalf("FetchCodexResetCredits failed: %v", err)
	}
	if credits.AvailableCount != 2 {
		t.Fatalf("expected available_count 2, got %d", credits.AvailableCount)
	}
	if len(credits.Credits) != 1 || credits.Credits[0].Title != "Full reset" {
		t.Fatalf("unexpected credits: %+v", credits.Credits)
	}
}

func TestConsumeCodexResetCreditPostsRedeemRequestID(t *testing.T) {
	writeCodexAuthForReset(t)

	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/rate-limit-reset-credits/consume" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("missing json content type")
		}
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"code":"reset","windows_reset":1}`)
	}))
	defer server.Close()
	t.Setenv("OCT_CODEX_USAGE_ENDPOINT", server.URL)

	result, err := ConsumeCodexResetCredit(t.Context(), "", "req-1234")
	if err != nil {
		t.Fatalf("ConsumeCodexResetCredit failed: %v", err)
	}
	if result.Code != "reset" {
		t.Fatalf("expected code reset, got %q", result.Code)
	}
	if result.WindowsReset != 1 {
		t.Fatalf("expected windows_reset 1, got %d", result.WindowsReset)
	}
	if !strings.Contains(gotBody, `"redeem_request_id":"req-1234"`) {
		t.Fatalf("expected redeem_request_id in body, got %s", gotBody)
	}
}

func TestConsumeCodexResetCreditGeneratesUUIDv4(t *testing.T) {
	writeCodexAuthForReset(t)

	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		gotBody = string(body)
		fmt.Fprint(w, `{"code":"reset","windows_reset":1}`)
	}))
	defer server.Close()
	t.Setenv("OCT_CODEX_USAGE_ENDPOINT", server.URL)

	if _, err := ConsumeCodexResetCredit(t.Context(), "", ""); err != nil {
		t.Fatalf("ConsumeCodexResetCredit failed: %v", err)
	}
	uuidV4 := regexp.MustCompile(`"redeem_request_id":"[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}"`)
	if !uuidV4.MatchString(gotBody) {
		t.Fatalf("expected a UUID v4 redeem_request_id, got %s", gotBody)
	}
}

func TestConsumeCodexResetCreditSurfacesConflict(t *testing.T) {
	writeCodexAuthForReset(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		fmt.Fprint(w, `{"error":{"message":"no available credit"}}`)
	}))
	defer server.Close()
	t.Setenv("OCT_CODEX_USAGE_ENDPOINT", server.URL)

	_, err := ConsumeCodexResetCredit(t.Context(), "", "req-1234")
	if err == nil {
		t.Fatalf("expected error on 409")
	}
	if !strings.Contains(err.Error(), "no available credit") {
		t.Fatalf("expected upstream message in error, got %v", err)
	}
}

func TestFetchCodexResetCreditsRequiresAuth(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	t.Setenv("CODEX_HOME", filepath.Join(tmp, ".codex"))

	_, err := FetchCodexResetCredits(t.Context(), "")
	if err == nil || !strings.Contains(err.Error(), "codex auth unavailable") {
		t.Fatalf("expected auth error, got %v", err)
	}
}
