package usage

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/suho-han/one-click-ai-tools/internal/netclient"
)

// isCodexProviderName reports whether a UsageResult.Provider value is the
// default codex row or one of the codex:<name> account rows. Display code
// uses it instead of an exact "codex" comparison so account rows keep codex
// behavior (7d-first buckets, no phantom 5h).
func isCodexProviderName(provider string) bool {
	p := strings.ToLower(strings.TrimSpace(provider))
	return p == "codex" || strings.HasPrefix(p, "codex:")
}

// CodexLoginEmail reads the account email from the auth.json JWT written by
// `codex login` under home. "" when no auth.json, token, or email claim is
// found — callers fall back to asking the user for a name.
func CodexLoginEmail(home string) string {
	data, err := os.ReadFile(filepath.Join(home, "auth.json"))
	if err != nil {
		return ""
	}
	var auth struct {
		Tokens struct {
			IDToken     string `json:"id_token"`
			AccessToken string `json:"access_token"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(data, &auth); err != nil {
		return ""
	}
	for _, token := range []string{auth.Tokens.IDToken, auth.Tokens.AccessToken} {
		parts := strings.Split(strings.TrimSpace(token), ".")
		if len(parts) < 2 {
			continue
		}
		payload, err := decodeJWTPayload(parts[1])
		if err != nil {
			continue
		}
		if email, ok := payload["email"].(string); ok {
			if email = strings.TrimSpace(email); email != "" {
				return email
			}
		}
	}
	return ""
}

func FetchCodexUsage(ctx context.Context) UsageResult {
	return fetchCodexUsageForHome(ctx, "", "codex")
}

// fetchCodexUsageForHome collects Codex usage from one credential home. An
// empty home resolves through codexHomePath ($CODEX_HOME or ~/.codex); the
// codex:<name> account rows pass their configured directory instead. provider
// is the UsageResult label ("codex", "codex:work", ...).
func fetchCodexUsageForHome(ctx context.Context, home string, provider string) UsageResult {
	result := UsageResult{
		Provider:   provider,
		Plan:       "unknown",
		PlanSource: "codex auth unavailable",
		Period:     "current",
		Used:       "n/a",
		Limit:      "100",
		Unit:       "percent",
		Source:     "local",
		Status:     "error",
	}

	result = withPlanDetection(ctx, result, func(ctx context.Context) (string, string) {
		return detectCodexPlanForHome(ctx, home)
	})

	if backendResult, ok := fetchCodexBackendUsage(ctx, result, home); ok {
		return backendResult
	}

	codexHome, ok := resolveCodexHome(home)
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
		result.Message = "No .jsonl session logs found in " + filepath.Join(codexHome, "sessions")
		return result
	}

	// collectCodexLogFiles returns names sorted by timestamp, so the latest
	// log is the last entry.
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
	if osDebugEnabled() {
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

func fetchCodexBackendUsage(ctx context.Context, base UsageResult, home string) (UsageResult, bool) {
	endpoint := strings.TrimSpace(os.Getenv("OCT_CODEX_USAGE_ENDPOINT"))
	if endpoint == "" {
		endpoint = "https://chatgpt.com/backend-api/wham/usage"
	}

	auth, hasAuth := readCodexBackendAuthFromHome(home)
	if !hasAuth && os.Getenv("OCT_CODEX_USAGE_ENDPOINT") == "" {
		return base, false
	}

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
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

	body, err := readAllCapped(resp.Body)
	if err != nil {
		return base, false
	}
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
	result.BucketResets = map[string]string{}

	if strings.TrimSpace(payload.PlanType) != "" {
		result.Plan = strings.TrimSpace(payload.PlanType)
		result.PlanSource = "codex backend wham/usage"
	}

	addCodexBackendWindow(result.Buckets, result.BucketResets, payload.RateLimit.PrimaryWindow)
	addCodexBackendWindow(result.Buckets, result.BucketResets, payload.RateLimit.SecondaryWindow)

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

	if osDebugEnabled() {
		result.SourceDetail = joinSourceDetails(
			codexBucketSourceDetail(result.Buckets),
			codexLocalModelSourceDetailFromHome(home, 50),
		)
	}
	return result, true
}

func addCodexBackendWindow(buckets map[string]string, resets map[string]string, window *codexBackendRateLimitWindow) {
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
	if window.ResetAt != nil && *window.ResetAt > 0 {
		resets["7d"] = fmt.Sprintf("%d", *window.ResetAt)
	}
}

// describeCodexCredential probes the Codex auth chain: auth.json under
// $CODEX_HOME (or ~/.codex), then the local session-log fallback that needs
// no credential.
func describeCodexCredential() CredentialStatus {
	return describeCodexCredentialForHome("")
}

func describeCodexCredentialForHome(home string) CredentialStatus {
	resolved, _ := resolveCodexHome(home)
	authPath := ""
	authFound := false
	if resolved != "" {
		authPath = filepath.Join(resolved, "auth.json")
		_, authFound = readCodexBackendAuthFromHome(home)
	}
	sessionsPath := ""
	sessionsFound := false
	if resolved != "" {
		sessionsPath = filepath.Join(resolved, "sessions")
		if logs, err := collectCodexLogFiles(sessionsPath); err == nil {
			sessionsFound = len(logs) > 0
		}
	}
	status := credentialStatus([]CredentialSource{
		{
			Kind:     CredentialKindFile,
			Location: authPath,
			Found:    authFound,
			Note:     "tokens.access_token written by `codex login`",
		},
		{
			Kind:     CredentialKindLocal,
			Location: sessionsPath,
			Found:    sessionsFound,
			Note:     "local session-log estimate needs no credential",
		},
	})
	if status.Status == CredentialStatusMissing {
		if strings.TrimSpace(home) != "" {
			status.Note = "run 'codex login' with CODEX_HOME pointing at this directory"
		} else {
			status.Note = "run 'codex login' to write ~/.codex/auth.json"
		}
	}
	return status
}

// resolveCodexHome resolves an explicit account home, falling back to the
// standard codexHomePath chain for the default row ("").
func resolveCodexHome(home string) (string, bool) {
	if strings.TrimSpace(home) != "" {
		return home, true
	}
	return codexHomePath()
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
	return readCodexBackendAuthFromHome(home)
}

func readCodexBackendAuthFromHome(home string) (codexBackendAuth, bool) {
	resolved, ok := resolveCodexHome(home)
	if !ok {
		return codexBackendAuth{}, false
	}

	data, err := os.ReadFile(filepath.Join(resolved, "auth.json"))
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

func codexLocalModelSourceDetailFromHome(home string, maxFiles int) string {
	codexHome, ok := resolveCodexHome(home)
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
	if !osDebugEnabled() || len(logFiles) == 0 || maxFiles <= 0 {
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
