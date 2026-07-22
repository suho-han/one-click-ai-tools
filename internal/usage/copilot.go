package usage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/suho-han/one-click-ai-tools/internal/netclient"
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

	if quotaResult, ok := fetchCopilotQuotaUsage(result, token); ok {
		return quotaResult
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

type copilotUserResponse struct {
	Login             string                          `json:"login"`
	CopilotPlan       string                          `json:"copilot_plan"`
	TokenBasedBilling bool                            `json:"token_based_billing"`
	QuotaResetDateUTC string                          `json:"quota_reset_date_utc"`
	QuotaSnapshots    map[string]copilotQuotaSnapshot `json:"quota_snapshots"`
}

type copilotQuotaSnapshot struct {
	EntitlementRequests int      `json:"entitlementRequests"`
	Entitlement         int      `json:"entitlement"`
	UsedRequests        int      `json:"usedRequests"`
	Used                int      `json:"used"`
	RemainingPercentage *float64 `json:"remainingPercentage"`
	PercentRemaining    *float64 `json:"percent_remaining"`
	ResetDate           string   `json:"resetDate"`
	QuotaResetAt        int64    `json:"quota_reset_at"`
}

func fetchCopilotQuotaUsage(base UsageResult, token string) (UsageResult, bool) {
	req, err := http.NewRequest("GET", copilotUserEndpoint(), nil)
	if err != nil {
		return base, false
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	req.Header.Set("User-Agent", "oct")
	req.Header.Set("Editor-Version", "oct/0.0.0")
	req.Header.Set("Editor-Plugin-Version", "oct/0.0.0")

	resp, err := netclient.DefaultClient.DoWithRetry(req)
	if err != nil || resp == nil {
		return base, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return base, false
	}

	body, _ := io.ReadAll(resp.Body)
	var payload copilotUserResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return base, false
	}

	snapshot, ok := payload.QuotaSnapshots["premium_interactions"]
	if !ok {
		return base, false
	}

	limit := snapshot.limit()
	used, ok := snapshot.used()
	if !ok {
		return base, false
	}

	result := base
	result.Status = "ok"
	result.Source = "quota"
	result.Period = "monthly"
	result.Used = fmt.Sprintf("%.0f", used)
	result.Unit = "AIC"
	if limit > 0 {
		result.Limit = fmt.Sprintf("%d", limit)
		result.Buckets = map[string]string{"quota": fmt.Sprintf("%.1f", used/float64(limit)*100)}
	} else {
		result.Limit = "n/a"
	}
	if strings.TrimSpace(payload.CopilotPlan) != "" {
		result.Plan = strings.TrimSpace(payload.CopilotPlan)
		result.PlanSource = "github copilot_internal/user"
	}
	result.Message = "Usage fetched from GitHub Copilot quota API"
	if os.Getenv("OCT_USAGE_DEBUG") == "1" {
		remaining := snapshot.remainingPercent()
		result.SourceDetail = fmt.Sprintf("premium_interactions_used=%.0f;limit=%d;remaining_percent=%.1f", used, limit, remaining)
	}
	return result, true
}

func (s copilotQuotaSnapshot) limit() int {
	if s.EntitlementRequests > 0 {
		return s.EntitlementRequests
	}
	return s.Entitlement
}

func (s copilotQuotaSnapshot) used() (float64, bool) {
	if s.UsedRequests > 0 {
		return float64(s.UsedRequests), true
	}
	if s.Used > 0 {
		return float64(s.Used), true
	}
	limit := s.limit()
	if limit <= 0 {
		return 0, false
	}
	remaining := s.remainingPercent()
	if remaining < 0 {
		return 0, false
	}
	return float64(limit) * (100 - remaining) / 100, true
}

func (s copilotQuotaSnapshot) remainingPercent() float64 {
	if s.RemainingPercentage != nil {
		return *s.RemainingPercentage
	}
	if s.PercentRemaining != nil {
		return *s.PercentRemaining
	}
	return -1
}

func copilotUserEndpoint() string {
	if endpoint := strings.TrimSpace(os.Getenv("OCT_COPILOT_USER_ENDPOINT")); endpoint != "" {
		return endpoint
	}
	return "https://api.github.com/copilot_internal/user"
}
