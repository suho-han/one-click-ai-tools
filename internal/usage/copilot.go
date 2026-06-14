package usage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
	"github.com/suho-han/one-click-tools/internal/netclient"
)

func FetchCopilotLocalUsage() UsageResult {
	result := UsageResult{
		Provider:   "copilot",
		Plan:       "unknown",
		PlanSource: "github copilot plan not exposed by current api integration",
		Period:     "local",
		Used:       "0",
		Limit:      "n/a",
		Unit:       "msgs",
		Source:     "local",
		Status:     "ok",
	}

	result = withPlanDetection(result, detectCopilotPlan)

	home, _ := os.UserHomeDir()
	sessionDir := filepath.Join(home, ".copilot", "session-state")

	var logFiles []string
	filepath.Walk(sessionDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && filepath.Base(path) == "events.jsonl" {
			logFiles = append(logFiles, path)
		}
		return nil
	})

	if len(logFiles) == 0 {
		result.Message = "No local session logs found in ~/.copilot"
		return result
	}

	var totalMsgs int
	var totalTokens int

	for _, logFile := range logFiles {
		file, err := os.Open(logFile)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			var line struct {
				Type string `json:"type"`
				Data struct {
					OutputTokens int `json:"outputTokens"`
				} `json:"data"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &line); err == nil {
				if line.Type == "assistant.message" {
					totalMsgs++
					if line.Data.OutputTokens > 0 {
						totalTokens += line.Data.OutputTokens
					}
				}
			}
		}
		file.Close()
	}

	result.Used = fmt.Sprintf("%d", totalMsgs)
	if totalTokens > 0 {
		result.Message = fmt.Sprintf("Estimated from local logs (%d total tokens)", totalTokens)
	} else {
		result.Message = "Estimated from local logs"
	}
	return result
}

func FetchCopilotUsage() UsageResult {
	result := UsageResult{
		Provider:   "copilot",
		Plan:       "unknown",
		PlanSource: "github copilot plan not exposed by current api integration",
		Period:     "current",
		Used:       "n/a",
		Limit:      "n/a",
		Unit:       "requests",
		Source:     "api",
		Status:     "error",
	}

	result = withPlanDetection(result, detectCopilotPlan)

	token := viper.GetString("github_api_token")
	if token == "" {
		token = os.Getenv("GITHUB_API_TOKEN")
	}
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" {
		ghToken, err := commandOutput(3*time.Second, "gh", "auth", "token")
		if err == nil && ghToken != "" {
			token = ghToken
			result.Source = "gh-cli"
		} else {
			return FetchCopilotLocalUsage()
		}
	}

	user := viper.GetString("github_user")
	if user == "" {
		user = os.Getenv("GITHUB_USER")
	}
	if user == "" {
		reqUser, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
		reqUser.Header.Set("Accept", "application/vnd.github+json")
		reqUser.Header.Set("Authorization", "Bearer "+token)

		respUser, err := netclient.DefaultClient.DoWithRetry(reqUser)
		if err == nil {
			defer respUser.Body.Close()
			if respUser.StatusCode == http.StatusOK {
				bodyUser, _ := io.ReadAll(respUser.Body)
				var userData struct {
					Login string `json:"login"`
				}
				if json.Unmarshal(bodyUser, &userData) == nil && userData.Login != "" {
					user = userData.Login
				}
			} else {
				result.Message = netclient.FormatError(respUser, nil)
				return result
			}
		} else {
			result.Message = netclient.FormatError(respUser, err)
			return result
		}

		if user == "" {
			result.Message = "GITHUB_USER not set and failed to fetch from /user"
			return result
		}
	}

	endpoint := fmt.Sprintf("https://api.github.com/users/%s/settings/billing/premium_request/usage", user)
	req, _ := http.NewRequest("GET", endpoint, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")

	resp, err := netclient.DefaultClient.DoWithRetry(req)
	if err != nil {
		result.Message = netclient.FormatError(resp, err)
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		result.Message = netclient.FormatError(resp, nil)
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
		result.Status = "ok"
		result.Used = "0.00"
		result.Message = "No Copilot usage items found"
		return result
	}

	result.Status = "ok"
	result.Used = fmt.Sprintf("%.2f", used)
	result.Unit = unit
	result.Message = "Usage fetched from GitHub Billing API"
	return result
}
