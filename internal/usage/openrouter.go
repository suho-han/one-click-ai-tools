package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/suho-han/one-click-ai-tools/internal/netclient"
)

// OpenRouter spend tracking. The /key endpoint works with a normal inference
// key (account credits require a management key, which most users do not
// have), reporting the key's own spend and optional spending limit in USD.
// OpenRouter has no CLI of its own, so it runs as a standalone (opt-in)
// provider.
//
// Endpoint: GET https://openrouter.ai/api/v1/key
// Auth:     Authorization: Bearer <OPENROUTER_API_KEY>
// Response: {"data": {"usage": 25.5, "usage_weekly": 3.2, "usage_monthly": 12.8,
//
//	"limit": 100, "limit_remaining": 74.5, "limit_reset": "monthly",
//	"is_free_tier": false}}
const openRouterKeyURL = "https://openrouter.ai/api/v1/key"

// openRouterKeyEndpoint allows overriding the API URL for testing.
var openRouterKeyEndpoint = openRouterKeyURL

type openRouterKeyResponse struct {
	Data struct {
		Usage          float64  `json:"usage"`
		UsageDaily     float64  `json:"usage_daily"`
		UsageWeekly    float64  `json:"usage_weekly"`
		UsageMonthly   float64  `json:"usage_monthly"`
		Limit          *float64 `json:"limit"`
		LimitRemaining *float64 `json:"limit_remaining"`
		LimitReset     string   `json:"limit_reset"`
		IsFreeTier     bool     `json:"is_free_tier"`
		Label          string   `json:"label"`
	} `json:"data"`
}

func formatUSD(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

// FetchOpenRouterUsage reports OpenRouter key spend against its spending limit.
func FetchOpenRouterUsage(ctx context.Context) UsageResult {
	result := UsageResult{
		Provider: "openrouter",
		Period:   "7d/1m",
		Unit:     "USD",
		Source:   "remote",
		Status:   "warn",
		Message:  "No data: OPENROUTER_API_KEY not set",
	}

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return result
	}

	endpoint := os.Getenv("OCT_OPENROUTER_USAGE_ENDPOINT")
	if endpoint == "" {
		endpoint = openRouterKeyEndpoint
	}
	result.SourceDetail = "auth_source=env:OPENROUTER_API_KEY"

	resp, err := fetchOpenRouterKey(ctx, endpoint, apiKey)
	if err != nil {
		result.Status = "error"
		result.Used = "n/a"
		result.Message = fmt.Sprintf("API error: %v", err)
		if osDebugEnabled() {
			result.SourceDetail = fmt.Sprintf("auth_source=env:OPENROUTER_API_KEY endpoint=%s", endpoint)
		}
		return result
	}

	data := resp.Data
	result.Status = "ok"
	result.Buckets = map[string]string{
		"7d": formatUSD(data.UsageWeekly),
		"1m": formatUSD(data.UsageMonthly),
	}
	result.Used = formatUSD(data.Usage)
	if data.Limit != nil {
		result.Limit = formatUSD(*data.Limit)
	}
	if data.IsFreeTier {
		result.Plan = "free"
	} else {
		result.Plan = "paid"
	}
	result.PlanSource = "remote_api"
	result.Message = "Fetched from OpenRouter key API"
	if data.LimitRemaining != nil {
		result.Message += fmt.Sprintf(" (%s remaining)", formatUSD(*data.LimitRemaining))
	}
	if osDebugEnabled() {
		result.SourceDetail = fmt.Sprintf("auth_source=env:OPENROUTER_API_KEY endpoint=%s", endpoint)
	}

	return result
}

// fetchOpenRouterKey calls the OpenRouter current-key endpoint.
func fetchOpenRouterKey(ctx context.Context, endpoint, apiKey string) (*openRouterKeyResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
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

	var parsed openRouterKeyResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &parsed, nil
}
