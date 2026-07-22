package usage

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/suho-han/one-click-ai-tools/internal/netclient"
)

func FetchClaudeUsage() UsageResult {
	home, _ := os.UserHomeDir()
	credsFile := filepath.Join(home, ".claude", ".credentials.json")

	result := UsageResult{
		Provider:   "claude-code",
		Plan:       "unknown",
		PlanSource: "claude plan not exposed",
		Period:     "current",
		Used:       "n/a",
		Limit:      "100",
		Unit:       "percent",
		Source:     "cli",
		Status:     "error",
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
		plan, source := detectClaudePlan(token)
		result = withPlan(result, plan, source)
		result.Status = "ok"
		result.Used = "0"
		result.Message = "No Claude OAuth token found (check ~/.claude/.credentials.json or CLAUDE_API_TOKEN)"
		return result
	}

	plan, source := detectClaudePlan(token)
	result = withPlan(result, plan, source)

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
			if cached, ok := fetchClaudeCachedUsage(result, home, "API rate limited"); ok {
				return cached
			}
			result.Status = "warn"
			result.Used = "n/a"
			result.Message = "Claude usage API rate limited and no local cached usage found"
			if os.Getenv("OCT_USAGE_DEBUG") == "1" {
				result.SourceDetail = "http_status=429;cache=missing"
			}
			return result
		}
		result.Message = netclient.FormatError(resp, nil)
		return result
	}

	body, _ := io.ReadAll(resp.Body)
	var data struct {
		FiveHour struct {
			Utilization float64 `json:"utilization"`
			ResetsAt    string  `json:"resets_at"`
		} `json:"five_hour"`
		SevenDay struct {
			Utilization float64 `json:"utilization"`
			ResetsAt    string  `json:"resets_at"`
		} `json:"seven_day"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		result.Message = "Failed to parse API response"
		return result
	}

	result.Buckets = make(map[string]string)
	result.BucketResets = make(map[string]string)
	if data.FiveHour.Utilization > 0 {
		result.Buckets["5h"] = fmt.Sprintf("%.1f", data.FiveHour.Utilization)
		if data.FiveHour.ResetsAt != "" {
			result.BucketResets["5h"] = data.FiveHour.ResetsAt
		}
	}
	if data.SevenDay.Utilization > 0 {
		result.Buckets["7d"] = fmt.Sprintf("%.1f", data.SevenDay.Utilization)
		if data.SevenDay.ResetsAt != "" {
			result.BucketResets["7d"] = data.SevenDay.ResetsAt
		}
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

type claudeCachedUsageFile struct {
	CachedUsageUtilization struct {
		FetchedAtMs int64 `json:"fetchedAtMs"`
		Utilization struct {
			FiveHour *claudeCachedUsageWindow `json:"five_hour"`
			SevenDay *claudeCachedUsageWindow `json:"seven_day"`
		} `json:"utilization"`
	} `json:"cachedUsageUtilization"`
}

type claudeCachedUsageWindow struct {
	Utilization *float64 `json:"utilization"`
	ResetsAt    string   `json:"resets_at"`
}

func fetchClaudeCachedUsage(base UsageResult, home string, reason string) (UsageResult, bool) {
	data, err := os.ReadFile(filepath.Join(home, ".claude.json"))
	if err != nil {
		return base, false
	}
	var cached claudeCachedUsageFile
	if err := json.Unmarshal(data, &cached); err != nil {
		return base, false
	}

	result := base
	result.Status = "ok"
	result.Source = "cache"
	result.Unit = "percent"
	result.Limit = "100"
	result.Buckets = map[string]string{}
	result.BucketResets = map[string]string{}

	fiveHour := cached.CachedUsageUtilization.Utilization.FiveHour
	sevenDay := cached.CachedUsageUtilization.Utilization.SevenDay
	if fiveHour != nil && fiveHour.Utilization != nil {
		result.Buckets["5h"] = fmt.Sprintf("%.1f", *fiveHour.Utilization)
		if fiveHour.ResetsAt != "" {
			result.BucketResets["5h"] = fiveHour.ResetsAt
		}
	}
	if sevenDay != nil && sevenDay.Utilization != nil {
		result.Buckets["7d"] = fmt.Sprintf("%.1f", *sevenDay.Utilization)
		if sevenDay.ResetsAt != "" {
			result.BucketResets["7d"] = sevenDay.ResetsAt
		}
	}

	if result.Buckets["5h"] != "" {
		result.Used = result.Buckets["5h"]
	} else if result.Buckets["7d"] != "" {
		result.Used = result.Buckets["7d"]
	} else {
		return base, false
	}

	result.Message = "Cached Claude usage from ~/.claude.json"
	if strings.TrimSpace(reason) != "" {
		result.Message += " (" + reason + ")"
	}
	if os.Getenv("OCT_USAGE_DEBUG") == "1" {
		details := []string{
			"cache_fetched_at_ms=" + fmt.Sprintf("%d", cached.CachedUsageUtilization.FetchedAtMs),
		}
		if fiveHour != nil && fiveHour.ResetsAt != "" {
			details = append(details, "5h_resets_at="+fiveHour.ResetsAt)
		}
		if sevenDay != nil && sevenDay.ResetsAt != "" {
			details = append(details, "7d_resets_at="+sevenDay.ResetsAt)
		}
		result.SourceDetail = strings.Join(details, ";")
	}
	return result, true
}
