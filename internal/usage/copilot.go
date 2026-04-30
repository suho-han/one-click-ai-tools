package usage

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

func FetchCopilotUsage() UsageResult {
	result := UsageResult{
		Provider: "copilot",
		Period:   "current",
		Used:     "n/a",
		Limit:    "n/a",
		Unit:     "requests",
		Source:   "api",
		Status:   "error",
	}

	token := os.Getenv("GITHUB_API_TOKEN")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" {
		result.Message = "No GitHub token found (GITHUB_API_TOKEN)"
		return result
	}

	user := os.Getenv("GITHUB_USER")
	if user == "" {
		// In a real scenario, we would fetch the user from /user endpoint
		result.Message = "GITHUB_USER not set"
		return result
	}

	endpoint := fmt.Sprintf("https://api.github.com/users/%s/settings/billing/premium_request/usage", user)
	req, _ := http.NewRequest("GET", endpoint, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")

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
		UsageItems []struct {
			Product     string  `json:"product"`
			NetQuantity float64 `json:"net_quantity"`
			UnitType    string  `json:"unit_type"`
		} `json:"usage_items"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		result.Message = "Failed to parse API response"
		return result
	}

	var used float64
	unit := "requests"
	found := false
	for _, item := range data.UsageItems {
		if item.Product == "copilot" {
			used += item.NetQuantity
			if item.UnitType != "" {
				unit = item.UnitType
			}
			found = true
		}
	}

	if !found {
		result.Message = "No Copilot usage items found"
		return result
	}

	result.Status = "ok"
	result.Used = fmt.Sprintf("%.2f", used)
	result.Unit = unit
	result.Message = "Usage fetched from GitHub Billing API"
	return result
}
