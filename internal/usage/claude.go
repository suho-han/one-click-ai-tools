package usage

import (
	"encoding/json"
	"fmt"
	"github.com/suho-han/one-click-tools/internal/netclient"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

func FetchClaudeUsage() UsageResult {
	home, _ := os.UserHomeDir()
	credsFile := filepath.Join(home, ".claude", ".credentials.json")

	result := UsageResult{
		Provider: "claude-code",
		Period:   "current",
		Used:     "n/a",
		Limit:    "100",
		Unit:     "percent",
		Source:   "cli",
		Status:   "error",
	}

	var token string

	// Try macOS Keychain first for Claude Code-credentials
	cmd := exec.Command("security", "find-generic-password", "-s", "Claude Code-credentials", "-w")
	out, err := cmd.Output()
	if err == nil && len(out) > 0 {
		var keychainCreds struct {
			ClaudeAiOauth struct {
				AccessToken string `json:"accessToken"`
			} `json:"claudeAiOauth"`
		}
		if json.Unmarshal(out, &keychainCreds) == nil && keychainCreds.ClaudeAiOauth.AccessToken != "" {
			token = keychainCreds.ClaudeAiOauth.AccessToken
		}
	}

	if token == "" {
		if _, err := os.Stat(credsFile); err == nil {
			data, _ := os.ReadFile(credsFile)
			var creds struct {
				AccessToken string `json:"access_token"`
			}
			if err := json.Unmarshal(data, &creds); err == nil && creds.AccessToken != "" {
				token = creds.AccessToken
			}
		}
	}

	if token == "" {
		token = os.Getenv("CLAUDE_API_TOKEN")
	}

	if token == "" {
		result.Status = "ok"
		result.Used = "0"
		result.Message = "No Claude OAuth token found (check ~/.claude/.credentials.json or CLAUDE_API_TOKEN)"
		return result
	}

	endpoint := "https://api.anthropic.com/api/oauth/usage"
	req, _ := http.NewRequest("GET", endpoint, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")

	resp, err := netclient.DefaultClient.DoWithRetry(req)
	if err != nil {
		result.Message = netclient.FormatError(resp, err)
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusTooManyRequests {
			result.Status = "ok"
			result.Used = "100"
			result.Buckets = map[string]string{"5h": "100.0"}
			if os.Getenv("OCT_USAGE_DEBUG") == "1" {
				result.SourceDetail = "http_status=429;fallback_bucket=5h"
			}
			result.Message = "API Rate Limited (assuming 100%)"
			return result
		}
		result.Message = netclient.FormatError(resp, nil)
		return result
	}

	body, _ := io.ReadAll(resp.Body)
	var data struct {
		FiveHour struct {
			Utilization float64 `json:"utilization"`
		} `json:"five_hour"`
		SevenDay struct {
			Utilization float64 `json:"utilization"`
		} `json:"seven_day"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		result.Message = "Failed to parse API response"
		return result
	}

	result.Buckets = make(map[string]string)
	if data.FiveHour.Utilization > 0 {
		result.Buckets["5h"] = fmt.Sprintf("%.1f", data.FiveHour.Utilization)
	}
	if data.SevenDay.Utilization > 0 {
		result.Buckets["7d"] = fmt.Sprintf("%.1f", data.SevenDay.Utilization)
	}
	if os.Getenv("OCT_USAGE_DEBUG") == "1" {
		result.SourceDetail = fmt.Sprintf("five_hour=%.1f;seven_day=%.1f", data.FiveHour.Utilization, data.SevenDay.Utilization)
	}

	if data.FiveHour.Utilization > 0 {
		result.Used = fmt.Sprintf("%.1f", data.FiveHour.Utilization)
		result.Message = "Usage fetched from Anthropic OAuth API (5h bucket)"
	} else if data.SevenDay.Utilization > 0 {
		result.Used = fmt.Sprintf("%.1f", data.SevenDay.Utilization)
		result.Message = "Usage fetched from Anthropic OAuth API (7d bucket)"
	} else {
		result.Used = "0"
		result.Message = "No utilization reported by API"
	}

	result.Status = "ok"
	result.Source = "oauth"
	return result
}
