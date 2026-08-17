package usage

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// userHomeDir is a variable for testing (allows mocking in tests)
var userHomeDir = os.UserHomeDir

const openCodeGoUsageURL = "https://opencode.ai/zen/go/v1/usage"

// openCodeGoAuth represents an auth entry from ~/.local/share/opencode/auth.json.
// Format: {"opencode-go": {"type": "api", "key": "..."}}
type openCodeGoAuth struct {
	Type string `json:"type"`
	Key  string `json:"key"`
}

// openCodeGoUsageResponse is the API response from https://opencode.ai/zen/go/v1/usage
// Example: {"usage": {"rolling": {"status":"ok","percent":42,"resetsAt":"..."}, ...}}
type openCodeGoUsageResponse struct {
	Usage map[string]openCodeGoWindowUsage `json:"usage"`
}

type openCodeGoWindowUsage struct {
	Status   string  `json:"status"`
	Percent  float64 `json:"percent"`
	ResetsAt string  `json:"resetsAt"`
}

// openCodeGoUsageEndpoint allows overriding the API URL for testing.
var openCodeGoUsageEndpoint = openCodeGoUsageURL

// resolveOpenCodeGoAPIKey tries to find the OpenCode Go API key.
// Priority: OPENCODE_API_KEY env -> ~/.local/share/opencode/auth.json (opencode-go.key)
func resolveOpenCodeGoAPIKey() (string, string) {
	if key := os.Getenv("OPENCODE_API_KEY"); key != "" {
		return key, "env:OPENCODE_API_KEY"
	}

	home, _ := userHomeDir()
	if home == "" {
		return "", ""
	}

	authPath := filepath.Join(home, ".local", "share", "opencode", "auth.json")
	data, err := os.ReadFile(authPath)
	if err != nil {
		return "", ""
	}

	var auths map[string]openCodeGoAuth
	if err := json.Unmarshal(data, &auths); err != nil {
		return "", ""
	}

	if auth, ok := auths["opencode-go"]; ok && auth.Key != "" {
		return auth.Key, "auth.json"
	}

	return "", ""
}

// FetchOpenCodeUsage fetches OpenCode Go quota from the remote API.
// It uses the OpenCode Go usage endpoint with the API key from auth.json or env.
func FetchOpenCodeUsage() UsageResult {
	result := UsageResult{
		Provider:   "opencode",
		Plan:       "opencode-go",
		PlanSource: "remote_api",
		Period:     "current",
		Used:       "0",
		Limit:      "100",
		Unit:       "percent",
		Source:     "remote",
		Status:     "warn",
		Message:    "No data: OpenCode Go API key not found",
	}

	apiKey, source := resolveOpenCodeGoAPIKey()
	if apiKey == "" {
		result.Message = "No data: OpenCode Go API key not found (set OPENCODE_API_KEY or run opencode auth login)"
		return result
	}

	result.SourceDetail = fmt.Sprintf("auth_source=%s", source)

	// Allow endpoint override for testing
	endpoint := os.Getenv("OCT_OPENCODE_USAGE_ENDPOINT")
	if endpoint == "" {
		endpoint = openCodeGoUsageEndpoint
	}

	resp, err := fetchOpenCodeGoUsage(endpoint, apiKey)
	if err != nil {
		result.Status = "error"
		result.Used = "n/a"
		result.Message = fmt.Sprintf("API error: %v", err)
		return result
	}

	result.Status = "ok"
	result.Buckets = map[string]string{}
	result.BucketResets = map[string]string{}

	if rolling, ok := resp.Usage["rolling"]; ok && rolling.Status == "ok" {
		result.Buckets["5h"] = fmt.Sprintf("%.0f", rolling.Percent)
		result.BucketResets["5h"] = rolling.ResetsAt
		result.Used = fmt.Sprintf("%.0f", rolling.Percent)
	}

	if weekly, ok := resp.Usage["weekly"]; ok && weekly.Status == "ok" {
		result.Buckets["7d"] = fmt.Sprintf("%.0f", weekly.Percent)
		result.BucketResets["7d"] = weekly.ResetsAt
	}

	if monthly, ok := resp.Usage["monthly"]; ok && monthly.Status == "ok" {
		result.Buckets["30d"] = fmt.Sprintf("%.0f", monthly.Percent)
		result.BucketResets["30d"] = monthly.ResetsAt
	}

	result.Message = "Fetched from OpenCode Go API"
	if os.Getenv("OCT_USAGE_DEBUG") == "1" {
		result.SourceDetail = fmt.Sprintf("auth_source=%s endpoint=%s", source, endpoint)
	}

	return result
}

// fetchOpenCodeGoUsage calls the OpenCode Go usage API and parses the response.
func fetchOpenCodeGoUsage(endpoint, apiKey string) (*openCodeGoUsageResponse, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "one-click-tools/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var usageResp openCodeGoUsageResponse
	if err := json.Unmarshal(body, &usageResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(usageResp.Usage) == 0 {
		return nil, fmt.Errorf("empty usage data in response")
	}

	return &usageResp, nil
}
