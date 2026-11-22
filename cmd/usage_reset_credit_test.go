package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/suho-han/one-click-ai-tools/internal/usage"
)

func TestResetCreditQueryUsesSoleCodexAccount(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "auth.json"), []byte(`{"tokens":{"access_token":"work-token","account_id":"work"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", t.TempDir())
	viper.Set("accounts", map[string][]map[string]string{"codex": {{"name": "work", "home": home}}})
	t.Cleanup(func() { viper.Set("accounts", nil) })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/rate-limit-reset-credits" || r.Header.Get("Authorization") != "Bearer work-token" {
			t.Errorf("expected read-only listing for the work account, got %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, `{"available_count":1,"credits":[{"title":"Full reset","status":"available","expires_at":"2099-10-29T19:40:37Z"}]}`)
	}))
	defer server.Close()
	t.Setenv("OCT_CODEX_USAGE_ENDPOINT", server.URL+"/usage")

	original := usageResetCreditFlags
	t.Cleanup(func() { usageResetCreditFlags = original })
	previousContext := usageResetCreditCmd.Context()
	usageResetCreditCmd.SetContext(t.Context())
	t.Cleanup(func() { usageResetCreditCmd.SetContext(previousContext) })
	usageResetCreditFlags.provider, usageResetCreditFlags.json, usageResetCreditFlags.yes = "codex", true, false
	var runErr error
	output := captureCommandStdout(t, func() { runErr = runUsageResetCredit(usageResetCreditCmd, nil) })
	if runErr != nil {
		t.Fatal(runErr)
	}
	var listing resetCreditOutput
	if err := json.Unmarshal([]byte(output), &listing); err != nil {
		t.Fatal(err)
	}
	if listing.Consumed || len(listing.Credits) != 1 || listing.Credits[0].Title != "Full reset" || listing.Credits[0].ExpiresAt != "2099-10-29T19:40:37Z" {
		t.Fatalf("unexpected listing: %+v", listing)
	}
}

func TestResetCreditRejectsStaleSoleAccountTarget(t *testing.T) {
	work, fallback := t.TempDir(), t.TempDir()
	for home, account := range map[string]string{work: "work", fallback: "fallback"} {
		data := fmt.Sprintf(`{"tokens":{"access_token":"%[1]s-token","account_id":"%[1]s"}}`, account)
		if err := os.WriteFile(filepath.Join(home, "auth.json"), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("CODEX_HOME", fallback)
	t.Setenv("OCT_TELEMETRY_SOURCE", "menubar")
	viper.Set("accounts", map[string][]map[string]string{"codex": {{"name": "work", "home": work}}})
	t.Cleanup(func() { viper.Set("accounts", nil) })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/usage" || r.Header.Get("Authorization") != "Bearer work-token" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, `{"plan_type":"plus","rate_limit_reset_credits":{"available_count":1}}`)
	}))
	defer server.Close()
	t.Setenv("OCT_CODEX_USAGE_ENDPOINT", server.URL+"/usage")
	result := usage.FetchCodexUsageForHome(t.Context(), work, "codex")
	output := captureCommandStdout(t, func() {
		if err := usage.PrintJSON([]usage.UsageResult{result}); err != nil {
			t.Fatal(err)
		}
	})
	var snapshot struct {
		Results []struct {
			Target string `json:"reset_credit_target"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(output), &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Results) != 1 || snapshot.Results[0].Target == "" || strings.Contains(output, "work-token") {
		t.Fatalf("expected an opaque target in the usage snapshot, got %s", output)
	}
	viper.Set("accounts", map[string][]map[string]string{"codex": {{"name": "work", "home": work}, {"name": "other", "home": fallback}}})
	original := usageResetCreditFlags
	t.Cleanup(func() { usageResetCreditFlags = original })
	previousContext := usageResetCreditCmd.Context()
	usageResetCreditCmd.SetContext(t.Context())
	t.Cleanup(func() { usageResetCreditCmd.SetContext(previousContext) })
	usageResetCreditFlags.provider, usageResetCreditFlags.json, usageResetCreditFlags.yes = "codex", true, true
	usageResetCreditFlags.target = snapshot.Results[0].Target
	if err := runUsageResetCredit(usageResetCreditCmd, nil); err == nil || !strings.Contains(err.Error(), "account or credentials changed") {
		t.Fatalf("expected stale target error before accessing the default account, got %v", err)
	}
}
