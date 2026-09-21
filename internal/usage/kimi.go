package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/suho-han/one-click-ai-tools/internal/netclient"
)

// Kimi Code (Moonshot) subscription quota. The Kimi Code CLI stores its OAuth
// token under ~/.kimi-code; the coding API exposes weekly + 5-hour request
// windows for the subscription. oct converts each window to a percentage of
// its own limit so remaining-mode display, compact output, and threshold
// alerts treat it like the other percent-unit providers.
//
// Endpoint: GET https://api.kimi.com/coding/v1/usages
// Auth:     Authorization: Bearer <KIMI_CODE_API_KEY | credentials access_token>
// Response (values are absolute request counts as strings):
//
//	{"usage": {"limit":"2048","used":"214","remaining":"1834","resetTime":"..."},
//	 "limits": [{"window":{"duration":300,"timeUnit":"TIME_UNIT_MINUTE"},
//	             "detail":{"limit":"200","used":"139","remaining":"61","resetTime":"..."}}],
//	 "planName": "Moderato"}
const kimiUsageURL = "https://api.kimi.com/coding/v1/usages"

// kimiUsageEndpoint allows overriding the API URL for testing.
var kimiUsageEndpoint = kimiUsageURL

type kimiUsageResponse struct {
	Usage    kimiWindowDetail `json:"usage"`
	Limits   []kimiLimit      `json:"limits"`
	PlanName string           `json:"planName"`
}

// kimiWindowDetail is one window's absolute request counts. The API returns
// them as JSON strings, not numbers.
type kimiWindowDetail struct {
	Limit     string `json:"limit"`
	Used      string `json:"used"`
	Remaining string `json:"remaining"`
	ResetTime string `json:"resetTime"`
}

type kimiLimit struct {
	Window kimiWindow       `json:"window"`
	Detail kimiWindowDetail `json:"detail"`
}

type kimiWindow struct {
	Duration int    `json:"duration"`
	TimeUnit string `json:"timeUnit"`
}

// resolveKimiToken finds the Kimi Code bearer token.
// Priority: KIMI_CODE_API_KEY env -> credentials/kimi-code.json, then the
// newest credentials/kimi-code-env-<id>.json OAuth env-file under
// KIMI_CODE_HOME, defaulting to ~/.kimi-code (the Kimi Code CLI's OAuth
// credential; oct reads it, never refreshes it — the CLI's access tokens are
// short-lived (~15 min), so the token is only fresh shortly after a kimi run).
func resolveKimiToken() (string, string) {
	if key := os.Getenv("KIMI_CODE_API_KEY"); key != "" {
		return key, "env:KIMI_CODE_API_KEY"
	}

	home := os.Getenv("KIMI_CODE_HOME")
	if home == "" {
		if userHome, err := userHomeDir(); err == nil {
			home = filepath.Join(userHome, ".kimi-code")
		}
	}
	if home == "" {
		return "", ""
	}

	credDir := filepath.Join(home, "credentials")
	if data, err := os.ReadFile(filepath.Join(credDir, "kimi-code.json")); err == nil {
		if token := parseKimiAccessToken(data); token != "" {
			return token, "kimi-code.json"
		}
	}

	// The current CLI rotates OAuth env-files (kimi-code-env-<id>.json);
	// pick the most recently written one.
	envFiles, _ := filepath.Glob(filepath.Join(credDir, "kimi-code-env-*.json"))
	newest, newestTime := "", time.Time{}
	for _, path := range envFiles {
		if info, err := os.Stat(path); err == nil && info.ModTime().After(newestTime) {
			newest, newestTime = path, info.ModTime()
		}
	}
	if newest != "" {
		if data, err := os.ReadFile(newest); err == nil {
			if token := parseKimiAccessToken(data); token != "" {
				source := "kimi-code-env-*.json"
				if kimiTokenExpired(data) {
					source += " (access_token expired; run kimi once to refresh)"
				}
				return token, source
			}
		}
	}

	return "", ""
}

// describeKimiCredential probes the Kimi token chain in fetch priority order:
// KIMI_CODE_API_KEY -> credentials/kimi-code.json -> the newest rotated
// kimi-code-env-*.json, annotating expired OAuth tokens.
func describeKimiCredential() CredentialStatus {
	envSource := CredentialSource{
		Kind:     CredentialKindEnv,
		Location: "KIMI_CODE_API_KEY",
		Found:    credentialEnvFound("KIMI_CODE_API_KEY"),
	}

	home := os.Getenv("KIMI_CODE_HOME")
	if home == "" {
		home = credentialHomePath(".kimi-code")
	}
	credDir := ""
	if home != "" {
		credDir = filepath.Join(home, "credentials")
	}

	staticSource := CredentialSource{
		Kind:     CredentialKindFile,
		Location: filepath.Join(credDir, "kimi-code.json"),
	}
	rotatedSource := CredentialSource{
		Kind:     CredentialKindFile,
		Location: filepath.Join(credDir, "kimi-code-env-*.json"),
		Note:     "newest rotated OAuth env-file wins",
	}
	if credDir != "" {
		if path := newestKimiCredentialPath(credDir, "kimi-code.json"); path != "" {
			staticSource.Found = credentialJSONFileFound(path, "access_token")
		}
		if path := newestKimiCredentialPath(credDir, "kimi-code-env-*.json"); path != "" {
			if data, err := os.ReadFile(path); err == nil {
				rotatedSource.Found = parseKimiAccessToken(data) != ""
				if rotatedSource.Found && kimiTokenExpired(data) {
					rotatedSource.Note = "access_token expired; run kimi once to refresh"
				}
			}
		}
	}

	status := credentialStatus([]CredentialSource{envSource, staticSource, rotatedSource})
	if status.Status == CredentialStatusMissing {
		status.Note = "run 'kimi login' or set KIMI_CODE_API_KEY"
	}
	return status
}

// newestKimiCredentialPath picks the most recently modified file matching the
// glob under credDir, "" when nothing matches.
func newestKimiCredentialPath(credDir, pattern string) string {
	candidates, _ := filepath.Glob(filepath.Join(credDir, pattern))
	newest, newestTime := "", time.Time{}
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && info.ModTime().After(newestTime) {
			newest, newestTime = path, info.ModTime()
		}
	}
	return newest
}

func parseKimiAccessToken(data []byte) string {
	var cred struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(data, &cred); err != nil {
		return ""
	}
	return cred.AccessToken
}

// kimiTokenExpired reports whether the credential file's expires_at (unix
// seconds, the new env-file format) is in the past. Files without the field
// count as not expired.
func kimiTokenExpired(data []byte) bool {
	var cred struct {
		ExpiresAt int64 `json:"expires_at"`
	}
	if err := json.Unmarshal(data, &cred); err != nil || cred.ExpiresAt == 0 {
		return false
	}
	return time.Now().Unix() >= cred.ExpiresAt
}

// kimiPercent converts one window's used/limit request counts to a rounded
// percentage string, or "" when either side is missing or unparsable.
func kimiPercent(used, limit string) string {
	u, errU := strconv.ParseFloat(strings.TrimSpace(used), 64)
	l, errL := strconv.ParseFloat(strings.TrimSpace(limit), 64)
	if errU != nil || errL != nil || l <= 0 {
		return ""
	}
	return fmt.Sprintf("%.0f", u/l*100)
}

// FetchKimiUsage fetches the Kimi Code subscription usage (weekly + 5-hour
// request windows, normalized to percentages). Problems travel in the
// UsageResult, never as an error.
func FetchKimiUsage(ctx context.Context) UsageResult {
	result := UsageResult{
		Provider: "kimi",
		Period:   "5h/7d",
		Unit:     "percent",
		Source:   "remote",
		Status:   "warn",
		Message:  "No data: Kimi Code token not found (run 'kimi login' or set KIMI_CODE_API_KEY)",
	}

	token, source := resolveKimiToken()
	if token == "" {
		return result
	}
	result.SourceDetail = fmt.Sprintf("auth_source=%s", source)

	endpoint := os.Getenv("OCT_KIMI_USAGE_ENDPOINT")
	if endpoint == "" {
		endpoint = kimiUsageEndpoint
	}

	resp, err := fetchKimiUsage(ctx, endpoint, token)
	if err != nil {
		result.Status = "error"
		result.Used = "n/a"
		result.Message = fmt.Sprintf("API error: %v", err)
		if osDebugEnabled() {
			result.SourceDetail = fmt.Sprintf("auth_source=%s endpoint=%s", source, endpoint)
		}
		return result
	}

	result.Status = "ok"
	result.Buckets = map[string]string{}
	result.BucketResets = map[string]string{}
	result.Plan = strings.TrimSpace(resp.PlanName)
	if result.Plan != "" {
		result.PlanSource = "remote_api"
	}

	// Top-level usage is the weekly window; the 300-minute limits[] entry is
	// the rolling 5-hour window. Each window is normalized to a percentage of
	// its own limit (Limit stays "100" on the percent scale).
	if v := kimiPercent(resp.Usage.Used, resp.Usage.Limit); v != "" {
		result.Buckets["7d"] = v
		result.BucketResets["7d"] = strings.TrimSpace(resp.Usage.ResetTime)
	}
	for _, l := range resp.Limits {
		if l.Window.Duration == 300 && l.Window.TimeUnit == "TIME_UNIT_MINUTE" {
			if v := kimiPercent(l.Detail.Used, l.Detail.Limit); v != "" {
				result.Buckets["5h"] = v
				result.BucketResets["5h"] = strings.TrimSpace(l.Detail.ResetTime)
			}
			break
		}
	}

	result.Used = firstNonEmpty(result.Buckets["7d"], result.Buckets["5h"])
	result.Limit = "100"
	result.Message = "Fetched from Kimi Code API"
	if osDebugEnabled() {
		result.SourceDetail = fmt.Sprintf("auth_source=%s endpoint=%s", source, endpoint)
	}

	return result
}

// fetchKimiUsage calls the Kimi Code usage API and parses the response.
func fetchKimiUsage(ctx context.Context, endpoint, token string) (*kimiUsageResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "one-click-tools/1.0")

	resp, err := netclient.DefaultClient.DoWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := readAllCapped(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var usageResp kimiUsageResponse
	if err := json.Unmarshal(body, &usageResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if strings.TrimSpace(usageResp.Usage.Used) == "" && len(usageResp.Limits) == 0 {
		return nil, fmt.Errorf("empty usage data in response")
	}

	return &usageResp, nil
}
