package usage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/suho-han/one-click-ai-tools/internal/netclient"
)

func FetchCodexUsage() UsageResult {
	result := UsageResult{
		Provider:   "codex",
		Plan:       "unknown",
		PlanSource: "codex auth unavailable",
		Period:     "current",
		Used:       "n/a",
		Limit:      "100",
		Unit:       "percent",
		Source:     "local",
		Status:     "error",
	}

	result = withPlanDetection(result, detectCodexPlan)

	if backendResult, ok := fetchCodexBackendUsage(result); ok {
		return backendResult
	}

	codexHome, ok := codexHomePath()
	if !ok {
		result.Status = "ok"
		result.Used = "0"
		result.Message = "Codex home unavailable"
		return result
	}
	sessionDir := filepath.Join(codexHome, "sessions")

	logFiles, err := collectCodexLogFiles(sessionDir)

	if err != nil || len(logFiles) == 0 {
		result.Status = "ok"
		result.Used = "0"
		result.Message = "No .jsonl session logs found in ~/.codex/sessions"
		return result
	}

	// Sort to get the latest file by name (which includes timestamp)
	sort.Strings(logFiles)
	latestLog := logFiles[len(logFiles)-1]

	file, err := os.Open(latestLog)
	if err != nil {
		result.Message = fmt.Sprintf("Failed to open latest log: %v", err)
		return result
	}
	defer file.Close()

	var lastWeeklyPercent string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var line struct {
			Type    string `json:"type"`
			Payload struct {
				Type string `json:"type"`
				Info struct {
					TotalTokenUsage struct {
						InputTokens int `json:"input_tokens"`
					} `json:"total_token_usage"`
				} `json:"info"`
				RateLimits struct {
					Primary struct {
						UsedPercent float64 `json:"used_percent"`
					} `json:"primary"`
					Secondary struct {
						UsedPercent   float64 `json:"used_percent"`
						WindowMinutes int     `json:"window_minutes"`
					} `json:"secondary"`
				} `json:"rate_limits"`
			} `json:"payload"`
		}

		if err := json.Unmarshal(scanner.Bytes(), &line); err == nil {
			if line.Type == "event_msg" && line.Payload.Type == "token_count" {
				// Codex quota is weekly-only for current backend/account responses; do not surface primary as 5h.
				if line.Payload.RateLimits.Secondary.UsedPercent > 0 && line.Payload.RateLimits.Secondary.WindowMinutes >= 10080 {
					lastWeeklyPercent = fmt.Sprintf("%.1f", line.Payload.RateLimits.Secondary.UsedPercent)
				}
			}
		}
	}

	if lastWeeklyPercent == "" {
		result.Status = "ok"
		result.Used = "0"
		result.Message = "No usage metrics found in latest session log"
		return result
	}

	result.Status = "ok"
	result.Buckets = make(map[string]string)
	result.Used = lastWeeklyPercent
	if lastWeeklyPercent != "" {
		result.Buckets["7d"] = lastWeeklyPercent
	}
	if os.Getenv("OCT_USAGE_DEBUG") == "1" {
		result.SourceDetail = joinSourceDetails(
			codexBucketSourceDetail(map[string]string{"7d": lastWeeklyPercent}),
			codexLocalModelSourceDetail(logFiles, 50),
		)
	}
	result.Message = "Usage extracted from local Codex session logs"
	return result
}

type codexBackendAuth struct {
	AccessToken string
	AccountID   string
}

type codexTokenUsage struct {
	InputTokens           int `json:"input_tokens"`
	CachedInputTokens     int `json:"cached_input_tokens"`
	CacheReadInputTokens  int `json:"cache_read_input_tokens"`
	OutputTokens          int `json:"output_tokens"`
	ReasoningOutputTokens int `json:"reasoning_output_tokens"`
	TotalTokens           int `json:"total_tokens"`
}

func (u codexTokenUsage) total() int {
	if u.TotalTokens > 0 {
		return u.TotalTokens
	}
	return u.InputTokens + u.OutputTokens
}

type codexLocalModelUsage struct {
	Model  string
	Tokens int
	Events int
}

type codexBackendUsageResponse struct {
	PlanType  string `json:"plan_type"`
	RateLimit struct {
		PrimaryWindow   *codexBackendRateLimitWindow `json:"primary_window"`
		SecondaryWindow *codexBackendRateLimitWindow `json:"secondary_window"`
	} `json:"rate_limit"`
}

type codexBackendRateLimitWindow struct {
	UsedPercent        *float64 `json:"used_percent"`
	LimitWindowSeconds *int     `json:"limit_window_seconds"`
	ResetAt            *int64   `json:"reset_at"`
}

func fetchCodexBackendUsage(base UsageResult) (UsageResult, bool) {
	endpoint := strings.TrimSpace(os.Getenv("OCT_CODEX_USAGE_ENDPOINT"))
	if endpoint == "" {
		endpoint = "https://chatgpt.com/backend-api/wham/usage"
	}

	auth, hasAuth := readCodexBackendAuth()
	if !hasAuth && os.Getenv("OCT_CODEX_USAGE_ENDPOINT") == "" {
		return base, false
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return base, false
	}
	if auth.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+auth.AccessToken)
		req.Header.Set("User-Agent", "codex-cli")
		req.Header.Set("OpenAI-Beta", "codex-1")
		req.Header.Set("originator", "Codex Desktop")
	}
	if auth.AccountID != "" {
		req.Header.Set("ChatGPT-Account-Id", auth.AccountID)
	}

	resp, err := netclient.DefaultClient.DoWithRetry(req)
	if err != nil || resp == nil {
		return base, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return base, false
	}

	body, _ := io.ReadAll(resp.Body)
	var payload codexBackendUsageResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return base, false
	}

	result := base
	result.Status = "ok"
	result.Source = "backend"
	result.Unit = "percent"
	result.Limit = "100"
	result.Buckets = map[string]string{}

	if strings.TrimSpace(payload.PlanType) != "" {
		result.Plan = strings.TrimSpace(payload.PlanType)
		result.PlanSource = "codex backend wham/usage"
	}

	addCodexBackendWindow(result.Buckets, payload.RateLimit.PrimaryWindow)
	addCodexBackendWindow(result.Buckets, payload.RateLimit.SecondaryWindow)

	if result.Buckets["7d"] != "" {
		result.Used = result.Buckets["7d"]
		result.Message = "Usage fetched from Codex backend API (weekly bucket)"
	} else {
		if strings.TrimSpace(payload.PlanType) == "" {
			return base, false
		}
		result.Used = "0"
		result.Message = "Usage fetched from Codex backend API (no rate-limit window reported)"
	}

	if os.Getenv("OCT_USAGE_DEBUG") == "1" {
		result.SourceDetail = joinSourceDetails(
			codexBucketSourceDetail(result.Buckets),
			codexLocalModelSourceDetailFromHome(50),
		)
	}
	return result, true
}

func addCodexBackendWindow(buckets map[string]string, window *codexBackendRateLimitWindow) {
	if window == nil || window.UsedPercent == nil {
		return
	}
	if window.LimitWindowSeconds != nil {
		minutes := (*window.LimitWindowSeconds + 59) / 60
		if minutes < 10080 {
			return
		}
	}
	buckets["7d"] = fmt.Sprintf("%.1f", *window.UsedPercent)
}

func codexHomePath() (string, bool) {
	home := strings.TrimSpace(os.Getenv("CODEX_HOME"))
	if home != "" {
		return home, true
	}

	userHome, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(userHome) == "" {
		return "", false
	}
	return filepath.Join(userHome, ".codex"), true
}

func readCodexBackendAuth() (codexBackendAuth, bool) {
	home, ok := codexHomePath()
	if !ok {
		return codexBackendAuth{}, false
	}

	data, err := os.ReadFile(filepath.Join(home, "auth.json"))
	if err != nil {
		return codexBackendAuth{}, false
	}
	var auth struct {
		Tokens struct {
			AccessToken string `json:"access_token"`
			AccountID   string `json:"account_id"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(data, &auth); err != nil {
		return codexBackendAuth{}, false
	}
	accessToken := strings.TrimSpace(auth.Tokens.AccessToken)
	accountID := strings.TrimSpace(auth.Tokens.AccountID)
	if accessToken == "" {
		return codexBackendAuth{}, false
	}
	return codexBackendAuth{AccessToken: accessToken, AccountID: accountID}, true
}

func codexBucketSourceDetail(buckets map[string]string) string {
	parts := make([]string, 0, 2)
	for _, key := range []string{"5h", "7d"} {
		if value := strings.TrimSpace(buckets[key]); value != "" {
			parts = append(parts, fmt.Sprintf("bucket_%s=%s", key, value))
		}
	}
	return strings.Join(parts, ";")
}

func collectCodexLogFiles(sessionDir string) ([]string, error) {
	var logFiles []string
	err := filepath.Walk(sessionDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".jsonl") {
			logFiles = append(logFiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(logFiles)
	return logFiles, nil
}

func codexLocalModelSourceDetailFromHome(maxFiles int) string {
	codexHome, ok := codexHomePath()
	if !ok {
		return ""
	}
	logFiles, err := collectCodexLogFiles(filepath.Join(codexHome, "sessions"))
	if err != nil {
		return ""
	}
	return codexLocalModelSourceDetail(logFiles, maxFiles)
}

func codexLocalModelSourceDetail(logFiles []string, maxFiles int) string {
	if os.Getenv("OCT_USAGE_DEBUG") != "1" || len(logFiles) == 0 || maxFiles <= 0 {
		return ""
	}

	start := len(logFiles) - maxFiles
	if start < 0 {
		start = 0
	}
	usage := map[string]*codexLocalModelUsage{}
	for _, path := range logFiles[start:] {
		addCodexLocalModelUsage(path, usage)
	}
	if len(usage) == 0 {
		return ""
	}

	rows := make([]codexLocalModelUsage, 0, len(usage))
	for _, row := range usage {
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Tokens == rows[j].Tokens {
			return rows[i].Model < rows[j].Model
		}
		return rows[i].Tokens > rows[j].Tokens
	})

	parts := make([]string, 0, len(rows))
	sparkIncluded := false
	for _, row := range rows {
		isSpark := strings.Contains(strings.ToLower(row.Model), "spark")
		if len(parts) >= 4 && !isSpark {
			continue
		}
		if isSpark {
			sparkIncluded = true
		}
		parts = append(parts, fmt.Sprintf("%s:%dt/%de", row.Model, row.Tokens, row.Events))
	}
	if !sparkIncluded {
		for _, row := range rows {
			if strings.Contains(strings.ToLower(row.Model), "spark") {
				parts = append(parts, fmt.Sprintf("%s:%dt/%de", row.Model, row.Tokens, row.Events))
				break
			}
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "local_recent_models=" + strings.Join(parts, ",")
}

func addCodexLocalModelUsage(path string, usage map[string]*codexLocalModelUsage) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	currentModel := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var line struct {
			Type    string `json:"type"`
			Payload struct {
				Type  string `json:"type"`
				Model string `json:"model"`
				Info  struct {
					Model           string          `json:"model"`
					LastTokenUsage  codexTokenUsage `json:"last_token_usage"`
					TotalTokenUsage codexTokenUsage `json:"total_token_usage"`
				} `json:"info"`
			} `json:"payload"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}
		if strings.TrimSpace(line.Payload.Model) != "" {
			currentModel = strings.TrimSpace(line.Payload.Model)
		}
		if line.Type != "event_msg" || line.Payload.Type != "token_count" {
			continue
		}

		model := strings.TrimSpace(line.Payload.Info.Model)
		if model == "" {
			model = currentModel
		}
		if model == "" {
			continue
		}

		tokenUsage := line.Payload.Info.LastTokenUsage
		if tokenUsage.total() == 0 {
			tokenUsage = line.Payload.Info.TotalTokenUsage
		}
		total := tokenUsage.total()
		if total <= 0 {
			continue
		}

		row := usage[model]
		if row == nil {
			row = &codexLocalModelUsage{Model: model}
			usage[model] = row
		}
		row.Tokens += total
		row.Events++
	}
}

func joinSourceDetails(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(strings.Trim(part, ";"))
		if part != "" {
			out = append(out, part)
		}
	}
	return strings.Join(out, ";")
}
