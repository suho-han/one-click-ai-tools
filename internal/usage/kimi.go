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

// Kimi Code (Moonshot) subscription quota. The Kimi Code CLI stores its OAuth
// token under ~/.kimi-code; the coding API exposes weekly + 5-hour request
// windows for the subscription.
//
// Endpoint: GET https://api.kimi.com/coding/v1/usages
// Auth:     Authorization: Bearer <KIMI_CODE_API_KEY | credentials access_token>
// Response (values are absolute request counts as strings):
//
//	{"usage": {"limit":"2048","used":"214","remaining":"1834","resetTime":"..."},
//	 "limits": [{"window":{"duration":300,"timeUnit":"TIME_UNIT_MINUTE"},
//	             "detail":{"limit":"200","used":"139","remaining":"61","resetTime":"..."}}],
//	 "planName": "Moderato"}
const kimiUsageURL = "https://api.kimi.com/coding/v1/usages"

// kimiUsageEndpoint allows overriding the API URL for testing.
var kimiUsageEndpoint = kimiUsageURL

type kimiUsageResponse struct {
	Usage    kimiWindowDetail `json:"usage"`
	Limits   []kimiLimit      `json:"limits"`
	PlanName string           `json:"planName"`
}

// kimiWindowDetail is one window's absolute request counts. The API returns
// them as JSON strings, not numbers.
type kimiWindowDetail struct {
	Limit     string `json:"limit"`
	Used      string `json:"used"`
	Remaining string `json:"remaining"`
	ResetTime string `json:"resetTime"`
}

type kimiLimit struct {
	Window kimiWindow       `json:"window"`
	Detail kimiWindowDetail `json:"detail"`
}

type kimiWindow struct {
	Duration int    `json:"duration"`
	TimeUnit string `json:"timeUnit"`
}

// resolveKimiToken finds the Kimi Code bearer token.
// Priority: KIMI_CODE_API_KEY env -> ~/.kimi-code/credentials/kimi-code.json
// (the Kimi Code CLI's OAuth credential; oct reads it, never refreshes it).
func resolveKimiToken() (string, string) {
	if key := os.Getenv("KIMI_CODE_API_KEY"); key != "" {
		return key, "env:KIMI_CODE_API_KEY"
	}

	home := os.Getenv("KIMI_CODE_HOME")
	if home == "" {
		home, _ = userHomeDir()
	}
	if home == "" {
		return "", ""
	}

	data, err := os.ReadFile(filepath.Join(home, "credentials", "kimi-code.json"))
	if err != nil {
		return "", ""
	}

	var cred struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(data, &cred); err != nil || cred.AccessToken == "" {
		return "", ""
	}
	return cred.AccessToken, "kimi-code.json"
}

// FetchKimiUsage fetches the Kimi Code subscription usage (weekly + 5-hour
// request windows). Problems travel in the UsageResult, never as an error.
func FetchKimiUsage(ctx context.Context) UsageResult {
	result := UsageResult{
		Provider: "kimi",
		Period:   "5h/7d",
		Unit:     "req",
		Source:   "remote",
		Status:   "warn",
		Message:  "No data: Kimi Code token not found (run 'kimi login' or set KIMI_CODE_API_KEY)",
	}

	token, source := resolveKimiToken()
	if token == "" {
		return result
	}
	result.SourceDetail = fmt.Sprintf("auth_source=%s", source)

	endpoint := os.Getenv("OCT_KIMI_USAGE_ENDPOINT")
	if endpoint == "" {
		endpoint = kimiUsageEndpoint
	}

	resp, err := fetchKimiUsage(ctx, endpoint, token)
	if err != nil {
		result.Status = "error"
		result.Used = "n/a"
		result.Message = fmt.Sprintf("API error: %v", err)
		if osDebugEnabled() {
			result.SourceDetail = fmt.Sprintf("auth_source=%s endpoint=%s", source, endpoint)
		}
		return result
	}

	result.Status = "ok"
	result.Buckets = map[string]string{}
	result.BucketResets = map[string]string{}
	result.Plan = strings.TrimSpace(resp.PlanName)
	if result.Plan != "" {
		result.PlanSource = "remote_api"
	}

	// Top-level usage is the weekly window; the 300-minute limits[] entry is
	// the rolling 5-hour window.
	result.Buckets["7d"] = strings.TrimSpace(resp.Usage.Used)
	result.BucketResets["7d"] = strings.TrimSpace(resp.Usage.ResetTime)
	for _, l := range resp.Limits {
		if l.Window.Duration == 300 && l.Window.TimeUnit == "TIME_UNIT_MINUTE" {
			result.Buckets["5h"] = strings.TrimSpace(l.Detail.Used)
			result.BucketResets["5h"] = strings.TrimSpace(l.Detail.ResetTime)
			break
		}
	}

	result.Used = firstNonEmpty(result.Buckets["7d"], result.Buckets["5h"])
	result.Limit = strings.TrimSpace(resp.Usage.Limit)
	result.Message = "Fetched from Kimi Code API"
	if osDebugEnabled() {
		result.SourceDetail = fmt.Sprintf("auth_source=%s endpoint=%s", source, endpoint)
	}

	return result
}

// fetchKimiUsage calls the Kimi Code usage API and parses the response.
func fetchKimiUsage(ctx context.Context, endpoint, token string) (*kimiUsageResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "one-click-tools/1.0")

	resp, err := netclient.DefaultClient.DoWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := readAllCapped(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var usageResp kimiUsageResponse
	if err := json.Unmarshal(body, &usageResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if strings.TrimSpace(usageResp.Usage.Used) == "" && len(usageResp.Limits) == 0 {
		return nil, fmt.Errorf("empty usage data in response")
	}

	return &usageResp, nil
}
