package usage

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func setAccountsConfig(t *testing.T, value interface{}) {
	t.Helper()
	viper.Set("accounts", value)
	t.Cleanup(func() { viper.Set("accounts", nil) })
}

func setLegacyCodexAccountsConfig(t *testing.T, value interface{}) {
	t.Helper()
	viper.Set("codex_accounts", value)
	t.Cleanup(func() { viper.Set("codex_accounts", nil) })
}

func TestAccountsParseListForm(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	setAccountsConfig(t, map[string]interface{}{
		"codex": []interface{}{
			map[string]interface{}{"name": "Work", "home": "~/codex-work"},
			map[string]interface{}{"name": "work", "home": "~/duplicate"},
			map[string]interface{}{"name": "", "home": "~/x"},
			map[string]interface{}{"name": "has space", "home": "~/y"},
			map[string]interface{}{"home": "~/noname"},
		},
		"kimi": []interface{}{
			map[string]interface{}{"name": "personal", "home": tmp + "/kimi-personal"},
		},
		// claude does not support accounts; the entry must be dropped.
		"claude": []interface{}{
			map[string]interface{}{"name": "ghost", "home": "~/claude-ghost"},
		},
	})

	accounts := AccountsForProvider("codex")
	if len(accounts) != 1 {
		t.Fatalf("codex accounts = %+v, want 1", accounts)
	}
	// Duplicate name: the last entry wins.
	if accounts[0].Name != "work" || accounts[0].Home != filepath.Join(tmp, "duplicate") {
		t.Fatalf("accounts[0] = %+v, want work -> %s", accounts[0], filepath.Join(tmp, "duplicate"))
	}

	kimi := AccountsForProvider("kimi")
	if len(kimi) != 1 || kimi[0].Name != "personal" || kimi[0].Home != filepath.Join(tmp, "kimi-personal") {
		t.Fatalf("kimi accounts = %+v", kimi)
	}

	if got := AccountsForProvider("claude"); len(got) != 0 {
		t.Fatalf("claude accounts = %+v, want none (unsupported provider)", got)
	}
}

func TestAccountsShorthandMapFormSorted(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	setAccountsConfig(t, map[string]interface{}{
		"codex": map[string]interface{}{
			"zeta":  "~/codex-zeta",
			"alpha": "~/codex-alpha",
		},
	})

	accounts := AccountsForProvider("codex")
	if len(accounts) != 2 {
		t.Fatalf("accounts = %+v, want 2", accounts)
	}
	if accounts[0].Name != "alpha" || accounts[1].Name != "zeta" {
		t.Fatalf("shorthand order = %s,%s, want alpha,zeta (sorted)", accounts[0].Name, accounts[1].Name)
	}
}

func TestAccountsLegacyCodexAccountsFallback(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	setLegacyCodexAccountsConfig(t, []interface{}{
		map[string]interface{}{"name": "work", "home": "~/codex-work"},
	})

	accounts := AccountsForProvider("codex")
	if len(accounts) != 1 || accounts[0].Name != "work" || accounts[0].Home != filepath.Join(tmp, "codex-work") {
		t.Fatalf("legacy codex_accounts = %+v, want work -> %s", accounts, filepath.Join(tmp, "codex-work"))
	}

	// The accounts key overrides a legacy entry with the same name.
	setAccountsConfig(t, map[string]interface{}{
		"codex": []interface{}{
			map[string]interface{}{"name": "work", "home": "~/codex-work-new"},
		},
	})
	accounts = AccountsForProvider("codex")
	if len(accounts) != 1 || accounts[0].Home != filepath.Join(tmp, "codex-work-new") {
		t.Fatalf("accounts key should override legacy entry, got %+v", accounts)
	}
}

func TestValidateAccountAlias(t *testing.T) {
	valid := []string{"work", "Work", "a", "acct-2", "acct_2", "han***o36", strings.Repeat("a", 32)}
	for _, alias := range valid {
		if err := ValidateAccountAlias(alias); err != nil {
			t.Fatalf("ValidateAccountAlias(%q) = %v, want nil", alias, err)
		}
	}
	invalid := []string{"", "has space", "-lead", "_lead", "*lead", strings.Repeat("a", 33), "codex:work"}
	for _, alias := range invalid {
		if err := ValidateAccountAlias(alias); err == nil {
			t.Fatalf("ValidateAccountAlias(%q) = nil, want error", alias)
		}
	}
}

func TestAccountLabelHelpers(t *testing.T) {
	if got := AccountProviderName("Codex", "Work"); got != "codex:work" {
		t.Fatalf("AccountProviderName = %q, want codex:work", got)
	}
	if provider, alias, ok := SplitAccountProviderLabel("kimi:personal"); !ok || provider != "kimi" || alias != "personal" {
		t.Fatalf("SplitAccountProviderLabel(kimi:personal) = %q,%q,%v", provider, alias, ok)
	}
	// "claude:work" is structurally an account label (claude is a registry
	// name) even though claude cannot be configured for accounts.
	if provider, alias, ok := SplitAccountProviderLabel("claude:work"); !ok || provider != "claude" || alias != "work" {
		t.Fatalf("SplitAccountProviderLabel(claude:work) = %q,%q,%v", provider, alias, ok)
	}
	for _, label := range []string{"codex", "zzz:work", "codexish"} {
		if _, _, ok := SplitAccountProviderLabel(label); ok {
			t.Fatalf("SplitAccountProviderLabel(%q) should not match", label)
		}
	}
	if !ProviderSupportsAccounts("codex") || !ProviderSupportsAccounts("grok") {
		t.Fatal("codex and grok should support accounts")
	}
	if ProviderSupportsAccounts("claude") || ProviderSupportsAccounts("zzz") {
		t.Fatal("unsupported providers must not pass ProviderSupportsAccounts")
	}
}

func TestFetchKimiAccountUsesOwnHome(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "kimi-personal")
	credDir := filepath.Join(home, "credentials")
	if err := os.MkdirAll(credDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(credDir, "kimi-code.json"), []byte(`{"access_token":"account-token"}`), 0o600); err != nil {
		t.Fatalf("write cred failed: %v", err)
	}

	var gotAuth []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = append(gotAuth, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"planName":"Moderato","usage":{"used":"30","limit":"100","resetTime":"2026-10-01T00:00:00Z"}}`)
	}))
	defer server.Close()
	t.Setenv("OCT_KIMI_USAGE_ENDPOINT", server.URL)
	t.Setenv("KIMI_CODE_API_KEY", "global-env-token") // account rows must ignore it

	result := fetchKimiUsageForHome(context.Background(), home, "kimi:personal")
	if len(gotAuth) != 1 || gotAuth[0] != "Bearer account-token" {
		t.Fatalf("Authorization = %v, want the account home's token", gotAuth)
	}
	if result.Provider != "kimi:personal" {
		t.Fatalf("provider = %q, want kimi:personal", result.Provider)
	}
	if result.Status != "ok" || result.Buckets["7d"] != "30" {
		t.Fatalf("status/bucket = %q/%q, want ok/30", result.Status, result.Buckets["7d"])
	}
}

func TestFetchGrokAccountUsesOwnHome(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "grok-second")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	auth := map[string]map[string]string{"https://auth.x.ai::client": {"key": "second-account-token"}}
	data, _ := json.Marshal(auth)
	if err := os.WriteFile(filepath.Join(home, "auth.json"), data, 0o600); err != nil {
		t.Fatalf("write auth failed: %v", err)
	}

	t.Setenv("GROK_OAUTH_TOKEN", "global-env-token") // account rows must ignore it
	t.Setenv("OCT_GROK_USAGE_ENDPOINT", "")
	t.Setenv("OCT_GROK_SETTINGS_ENDPOINT", "")
	origBilling, origSettings := grokBillingEndpoint, grokSettingsEndpoint
	grokBillingEndpoint = grokBillingURL
	grokSettingsEndpoint = grokSettingsURL
	t.Cleanup(func() { grokBillingEndpoint, grokSettingsEndpoint = origBilling, origSettings })

	// The real endpoints would need network; point them at a local stub.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"config":{"creditUsagePercent":55}}`))
	}))
	defer server.Close()
	grokBillingEndpoint = server.URL + "/v1/billing"
	grokSettingsEndpoint = server.URL + "/v1/settings"

	result := fetchGrokUsageForHome(context.Background(), home, "grok:second")
	if result.Status != "ok" || result.Buckets["1m"] != "55" {
		t.Fatalf("status/bucket = %q/%q, want ok/55 (message: %q)", result.Status, result.Buckets["1m"], result.Message)
	}
	if result.Provider != "grok:second" {
		t.Fatalf("provider = %q, want grok:second", result.Provider)
	}
}

func TestFetchQwenAccountUsesOwnHome(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "qwen-second")
	writeQwenUsageFixture(t, home, 12)

	result := fetchQwenUsageForHome(context.Background(), home, "qwen:second")
	// 12 today-dated records plus the fixture's localDate record, matching
	// TestFetchQwenUsageCountsToday's counting rule.
	if result.Status != "ok" || result.Used != "13" {
		t.Fatalf("status/used = %q/%q, want ok/13 (message: %q)", result.Status, result.Used, result.Message)
	}
	if result.Provider != "qwen:second" {
		t.Fatalf("provider = %q, want qwen:second", result.Provider)
	}
}

func TestGetUsageAccountRowsReplaceBaseRow(t *testing.T) {
	tmp := t.TempDir()
	setAccountsConfig(t, map[string]interface{}{
		"codex": []interface{}{
			map[string]interface{}{"name": "work", "home": filepath.Join(tmp, "codex-work")},
			map[string]interface{}{"name": "personal", "home": filepath.Join(tmp, "codex-personal")},
		},
		"kimi": []interface{}{
			map[string]interface{}{"name": "work", "home": filepath.Join(tmp, "kimi-work")},
		},
	})
	swapFetchers(t, map[string]func(context.Context) UsageResult{
		"codex": func(context.Context) UsageResult { return UsageResult{Provider: "codex", Status: "stub"} },
		"kimi":  func(context.Context) UsageResult { return UsageResult{Provider: "kimi", Status: "stub"} },
	})

	selectOnly(t, "codex", "kimi")
	results, err := GetUsage(context.Background())
	if err != nil {
		t.Fatalf("GetUsage() error = %v", err)
	}
	var names []string
	for _, r := range results {
		// No base row may survive: configured accounts replace it.
		if r.Status == "stub" {
			t.Fatalf("base row leaked alongside account rows: %+v", results)
		}
		names = append(names, r.Provider)
	}
	// Multi-account codex keeps the alias suffixes; single-account kimi
	// renders under the plain provider name.
	want := []string{"codex:work", "codex:personal", "kimi"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("results order = %v, want %v", names, want)
	}
}

func TestGetUsageGrokAccountRowsRideStandaloneRow(t *testing.T) {
	tmp := t.TempDir()
	setAccountsConfig(t, map[string]interface{}{
		"grok": []interface{}{
			map[string]interface{}{"name": "second", "home": filepath.Join(tmp, "grok-second")},
		},
	})
	swapFetchers(t, map[string]func(context.Context) UsageResult{
		// Sentinel status: the single grok account replaces the base row, so
		// the emitted "grok" row comes from the account fetcher, not this one.
		"grok": func(context.Context) UsageResult { return UsageResult{Provider: "grok", Status: "stub"} },
	})

	selectOnly(t, "grok")
	results, err := GetUsage(context.Background())
	if err != nil {
		t.Fatalf("GetUsage() error = %v", err)
	}
	var names []string
	for _, r := range results {
		if r.Status == "stub" {
			t.Fatalf("base grok row leaked alongside its account row: %+v", results)
		}
		names = append(names, r.Provider)
	}
	// A single account renders under the plain provider name.
	if strings.Join(names, ",") != "grok" {
		t.Fatalf("results = %v, want [grok]", names)
	}

	// Dropping grok from enabled_tools drops its account rows too.
	selectOnly(t, "codex")
	swapFetchers(t, map[string]func(context.Context) UsageResult{
		"codex": func(context.Context) UsageResult { return UsageResult{Provider: "codex", Status: "ok"} },
	})
	results, err = GetUsage(context.Background())
	if err != nil {
		t.Fatalf("GetUsage() error = %v", err)
	}
	for _, r := range results {
		if strings.HasPrefix(r.Provider, "grok") {
			t.Fatalf("grok account row leaked without the grok row: %+v", results)
		}
	}
}

func TestCompactProviderLabelAccount(t *testing.T) {
	if got := compactProviderLabel("codex:work"); got != "XW" {
		t.Fatalf("compactProviderLabel(codex:work) = %q, want XW", got)
	}
	if got := compactProviderLabel("kimi:personal"); got != "KP" {
		t.Fatalf("compactProviderLabel(kimi:personal) = %q, want KP", got)
	}
	if got := compactProviderLabel("kimi"); got != "K" {
		t.Fatalf("compactProviderLabel(kimi) = %q, want K", got)
	}
}

func TestCanonicalProviderNameAccount(t *testing.T) {
	setAccountsConfig(t, map[string]interface{}{
		"kimi": []interface{}{
			map[string]interface{}{"name": "work", "home": "~/kimi-work"},
		},
	})

	if resolved, ok := CanonicalProviderName("Kimi:Work"); !ok || resolved != "kimi:work" {
		t.Fatalf("CanonicalProviderName(Kimi:Work) = %q,%v, want kimi:work,true", resolved, ok)
	}
	if _, ok := CanonicalProviderName("kimi:ghost"); ok {
		t.Fatal("unconfigured account alias must not canonicalize")
	}
	if _, ok := CanonicalProviderName("codexish"); ok {
		t.Fatal("non-account codex-prefixed name must not canonicalize")
	}
}

func TestAlertProviderNamesIncludesAccounts(t *testing.T) {
	setAccountsConfig(t, map[string]interface{}{
		"codex": []interface{}{
			map[string]interface{}{"name": "work", "home": "~/codex-work"},
		},
		"kimi": []interface{}{
			map[string]interface{}{"name": "personal", "home": "~/kimi-personal"},
		},
	})

	found := map[string]bool{}
	for _, name := range AlertProviderNames() {
		found[name] = true
	}
	if !found["codex:work"] || !found["kimi:personal"] {
		t.Fatalf("AlertProviderNames() missing account rows: %v", found)
	}
}

func TestDescribeProviderCredentialsAccount(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "kimi-work")
	credDir := filepath.Join(home, "credentials")
	if err := os.MkdirAll(credDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(credDir, "kimi-code.json"), []byte(`{"access_token":"tok"}`), 0o600); err != nil {
		t.Fatalf("write cred failed: %v", err)
	}
	setAccountsConfig(t, map[string]interface{}{
		"kimi": []interface{}{
			map[string]interface{}{"name": "work", "home": home},
		},
	})

	status, ok := DescribeProviderCredentials("kimi:work")
	if !ok {
		t.Fatal("DescribeProviderCredentials(kimi:work) not found")
	}
	if status.Status == CredentialStatusMissing {
		t.Fatalf("status = %+v, want found credential", status)
	}

	if _, ok := DescribeProviderCredentials("kimi:ghost"); ok {
		t.Fatal("unconfigured account alias should be unknown")
	}
}

func TestCodexLoginEmail(t *testing.T) {
	tmp := t.TempDir()
	encode := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	token := encode(`{"alg":"RS256"}`) + "." + encode(`{"email":"hansuho36@gmail.com","plan":"pro"}`) + ".sig"

	auth := fmt.Sprintf(`{"tokens":{"id_token":%q,"access_token":"tok"}}`, token)
	if err := os.WriteFile(filepath.Join(tmp, "auth.json"), []byte(auth), 0o600); err != nil {
		t.Fatalf("write auth: %v", err)
	}

	if got := CodexLoginEmail(tmp); got != "hansuho36@gmail.com" {
		t.Fatalf("CodexLoginEmail = %q, want hansuho36@gmail.com", got)
	}

	// A token without an email claim yields "".
	noEmail := encode(`{"alg":"RS256"}`) + "." + encode(`{"sub":"abc"}`) + ".sig"
	auth = fmt.Sprintf(`{"tokens":{"id_token":%q}}`, noEmail)
	if err := os.WriteFile(filepath.Join(tmp, "auth.json"), []byte(auth), 0o600); err != nil {
		t.Fatalf("write auth: %v", err)
	}
	if got := CodexLoginEmail(tmp); got != "" {
		t.Fatalf("CodexLoginEmail without email claim = %q, want empty", got)
	}

	// Missing auth.json yields "".
	if got := CodexLoginEmail(filepath.Join(tmp, "nowhere")); got != "" {
		t.Fatalf("CodexLoginEmail for missing home = %q, want empty", got)
	}
}

func TestAccountFetchJobsLabels(t *testing.T) {
	tmp := t.TempDir()
	setAccountsConfig(t, map[string]interface{}{
		"codex": []interface{}{
			map[string]interface{}{"name": "solo", "home": filepath.Join(tmp, "codex-solo")},
		},
		"kimi": []interface{}{
			map[string]interface{}{"name": "a", "home": filepath.Join(tmp, "kimi-a")},
			map[string]interface{}{"name": "b", "home": filepath.Join(tmp, "kimi-b")},
		},
	})
	providerByName := func(name string) Provider {
		for _, p := range providers {
			if p.Name == name {
				return p
			}
		}
		return Provider{}
	}

	// A single account keeps the plain provider label: there is nothing to
	// disambiguate.
	codexJobs := accountFetchJobs(providerByName("codex"))
	var codexLabels []string
	for _, job := range codexJobs {
		codexLabels = append(codexLabels, job.label)
	}
	if strings.Join(codexLabels, ",") != "codex" {
		t.Fatalf("single-account codex jobs = %v, want [codex]", codexLabels)
	}

	// Several accounts keep their alias suffix.
	kimiJobs := accountFetchJobs(providerByName("kimi"))
	var kimiLabels []string
	for _, job := range kimiJobs {
		kimiLabels = append(kimiLabels, job.label)
	}
	if strings.Join(kimiLabels, ",") != "kimi:a,kimi:b" {
		t.Fatalf("multi-account kimi jobs = %v, want [kimi:a kimi:b]", kimiLabels)
	}
}

func TestGetUsageCodexAccountsReplaceBaseRow(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	setAccountsConfig(t, map[string]interface{}{
		"codex": []interface{}{
			map[string]interface{}{"name": "dup", "home": filepath.Join(tmp, "codex-dup")},
			map[string]interface{}{"name": "unbroken", "home": filepath.Join(tmp, "codex-unbroken")},
		},
	})
	swapFetchers(t, map[string]func(context.Context) UsageResult{
		// Sentinel status: the base row must not appear at all once accounts
		// are configured -- the account rows replace it.
		"codex": func(context.Context) UsageResult { return UsageResult{Provider: "codex", Status: "stub"} },
	})

	selectOnly(t, "codex")
	results, err := GetUsage(context.Background())
	if err != nil {
		t.Fatalf("GetUsage() error = %v", err)
	}
	var names []string
	for _, r := range results {
		if r.Status == "stub" {
			t.Fatalf("base codex row leaked alongside account rows: %+v", results)
		}
		names = append(names, r.Provider)
	}
	want := []string{"codex:dup", "codex:unbroken"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("results = %v, want %v", names, want)
	}
}
