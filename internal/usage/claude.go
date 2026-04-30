package usage

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
	if _, err := os.Stat(credsFile); err == nil {
		data, _ := os.ReadFile(credsFile)
		var creds struct {
			AccessToken string `json:"access_token"`
		}
		if err := json.Unmarshal(data, &creds); err == nil && creds.AccessToken != "" {
			token = creds.AccessToken
		}
	}

	if token == "" {
		token = os.Getenv("CLAUDE_API_TOKEN")
	}

	if token == "" {
		result.Message = "No Claude OAuth token found (check ~/.claude/.credentials.json or CLAUDE_API_TOKEN)"
		return result
	}

	endpoint := "https://api.anthropic.com/api/oauth/usage"
	req, _ := http.NewRequest("GET", endpoint, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		result.Message = fmt.Sprintf("API request failed: %v", err)
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		result.Message = fmt.Sprintf("API HTTP %d", resp.StatusCode)
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
