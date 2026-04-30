package usage

import (
	"encoding/json"
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
		Limit:    "n/a",
		Unit:     "requests",
		Source:   "cli",
		Status:   "ok",
	}

	if _, err := os.Stat(credsFile); err == nil {
		data, _ := os.ReadFile(credsFile)
		var creds struct {
			AccessToken string `json:"access_token"`
		}
		if err := json.Unmarshal(data, &creds); err == nil && creds.AccessToken != "" {
			result.Source = "oauth"
			result.Message = "Token found in ~/.claude/.credentials.json"
		}
	}

	return result
}
