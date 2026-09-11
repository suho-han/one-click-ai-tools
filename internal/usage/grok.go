package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/suho-han/one-click-ai-tools/internal/netclient"
)

// xAI SuperGrok credits. Tokens are issued by `grok login` (the xAI CLI
// refreshes them itself; oct only reads the cached credential) and the billing
// proxy exposes the plan's credit usage as a monthly window.
//
// Endpoints:
//
//	GET https://cli-chat-proxy.grok.com/v1/billing?format=credits
//	GET https://cli-chat-proxy.grok.com/v1/settings   (plan tier enrichment)
//	Auth: Authorization: Bearer <token>, x-xai-token-auth: xai-grok-cli
//
// Billing response: {"config": {"creditUsagePercent": 42,
//
//	"currentPeriod": {"end": "..."}, "billingPeriodEnd": "..."}}
//
// Credentials: GROK_OAUTH_TOKEN env, or ~/.grok/auth.json whose top-level keys
// are OIDC scope URLs ("https://auth.x.ai::<client-id>" preferred,
// "https://accounts.x.ai/sign-in" fallback) and whose values carry "key".
const (
	grokBillingURL    = "https://cli-chat-proxy.grok.com/v1/billing?format=credits"
	grokSettingsURL   = "https://cli-chat-proxy.grok.com/v1/settings"
	grokTokenAuthHdr  = "xai-grok-cli"
	grokAuthPreferred = "auth.x.ai"
	grokAuthFallback  = "accounts.x.ai/sign-in"
)

// grokBillingEndpoint allows overriding the API URL for testing.
var (
	grokBillingEndpoint  = grokBillingURL
	grokSettingsEndpoint = grokSettingsURL
)

type grokBillingResponse struct {
	Config struct {
		CreditUsagePercent *flexNum `json:"creditUsagePercent"`
		CurrentPeriod      struct {
			End string `json:"end"`
		} `json:"currentPeriod"`
		BillingPeriodEnd string `json:"billingPeriodEnd"`
	} `json:"config"`
}

type grokSettingsResponse struct {
	SubscriptionTierDisplay string `json:"subscription_tier_display"`
	Settings                struct {
		SubscriptionTierDisplay string `json:"subscription_tier_display"`
	} `json:"settings"`
}

// resolveGrokToken finds the SuperGrok bearer token.
// Priority: GROK_OAUTH_TOKEN env -> ~/.grok/auth.json (cached `grok login`
// credential; oct never refreshes it).
func resolveGrokToken() (string, string) {
	if key := os.Getenv("GROK_OAUTH_TOKEN"); key != "" {
		return key, "env:GROK_OAUTH_TOKEN"
	}

	home := os.Getenv("GROK_HOME")
	if home != "" {
		// GROK_HOME replaces the ~/.grok directory entirely, matching the CLI.
	} else {
		home, _ = userHomeDir()
		if home == "" {
			return "", ""
		}
		home = filepath.Join(home, ".grok")
	}

	data, err := os.ReadFile(filepath.Join(home, "auth.json"))
	if err != nil {
		return "", ""
	}

	var auths map[string]struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(data, &auths); err != nil {
		return "", ""
	}

	lookup := func(substr string) string {
		for scope, auth := range auths {
			if strings.Contains(scope, substr) && auth.Key != "" {
				return auth.Key
			}
		}
		return ""
	}
	if key := lookup(grokAuthPreferred); key != "" {
		return key, "grok:auth.json"
	}
	if key := lookup(grokAuthFallback); key != "" {
		return key, "grok:auth.json"
	}
	return "", ""
}

// FetchGrokUsage reports SuperGrok credit usage over its billing period.
func FetchGrokUsage(ctx context.Context) UsageResult {
	result := UsageResult{
		Provider: "grok",
		Period:   "1m",
		Unit:     "percent",
		Source:   "remote",
		Status:   "warn",
		Message:  "No data: xAI token not found (run 'grok login' or set GROK_OAUTH_TOKEN)",
	}

	token, source := resolveGrokToken()
	if token == "" {
		return result
	}
	result.SourceDetail = fmt.Sprintf("auth_source=%s", source)

	billingEndpoint := os.Getenv("OCT_GROK_USAGE_ENDPOINT")
	if billingEndpoint == "" {
		billingEndpoint = grokBillingEndpoint
	}

	billing, err := fetchGrokBilling(ctx, billingEndpoint, token)
	if err != nil {
		result.Status = "error"
		result.Used = "n/a"
		result.Message = fmt.Sprintf("API error: %v", err)
		return result
	}

	percent := billing.Config.CreditUsagePercent
	if percent == nil {
		result.Status = "warn"
		result.Message = "No creditUsagePercent in Grok billing response"
		if osDebugEnabled() {
			result.SourceDetail = fmt.Sprintf("auth_source=%s endpoint=%s", source, billingEndpoint)
		}
		return result
	}

	result.Status = "ok"
	result.Buckets = map[string]string{"1m": fmt.Sprintf("%.0f", float64(*percent))}
	result.Used = result.Buckets["1m"]

	reset := billing.Config.CurrentPeriod.End
	if reset == "" {
		reset = billing.Config.BillingPeriodEnd
	}
	if reset != "" {
		result.BucketResets = map[string]string{"1m": reset}
	}

	// Plan tier is enrichment only: a settings failure must not fail the row.
	settingsEndpoint := os.Getenv("OCT_GROK_SETTINGS_ENDPOINT")
	if settingsEndpoint == "" {
		settingsEndpoint = grokSettingsEndpoint
	}
	if settings, err := fetchGrokSettings(ctx, settingsEndpoint, token); err == nil {
		if tier := settings.tier(); tier != "" {
			result.Plan = tier
			result.PlanSource = "remote_api"
		}
	}

	result.Message = "Fetched from Grok billing API"
	if osDebugEnabled() {
		result.SourceDetail = fmt.Sprintf("auth_source=%s endpoint=%s", source, billingEndpoint)
	}

	return result
}

func (s grokSettingsResponse) tier() string {
	if s.SubscriptionTierDisplay != "" {
		return s.SubscriptionTierDisplay
	}
	return s.Settings.SubscriptionTierDisplay
}

func grokRequest(ctx context.Context, endpoint, token string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("x-xai-token-auth", grokTokenAuthHdr)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "one-click-tools/1.0")

	resp, err := netclient.DefaultClient.DoWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := readAllCapped(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return resp, nil
}

func fetchGrokBilling(ctx context.Context, endpoint, token string) (*grokBillingResponse, error) {
	resp, err := grokRequest(ctx, endpoint, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var parsed grokBillingResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &parsed, nil
}

func fetchGrokSettings(ctx context.Context, endpoint, token string) (*grokSettingsResponse, error) {
	resp, err := grokRequest(ctx, endpoint, token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var parsed grokSettingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &parsed, nil
}
