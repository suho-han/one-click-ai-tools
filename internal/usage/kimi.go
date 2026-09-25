package usage

import (
	"context"
	"encoding/json"
	"errors"
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

// errKimiNoUsageData marks a 200 response whose parsed payload carries no
// usage windows. Kimi is known to return an empty payload while the current
// billing window has nothing recorded yet (schema confirmation still
// pending), so it means "authenticated, nothing to report" — a warning, not
// a fetch error.
var errKimiNoUsageData = errors.New("no usage data in response")

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
// The third return reports whether the chosen token's expires_at has passed,
// so the fetch can short-circuit instead of spending a doomed API call.
func resolveKimiToken() (token, source string, expired bool) {
	if key := os.Getenv("KIMI_CODE_API_KEY"); key != "" {
		return key, "env:KIMI_CODE_API_KEY", false
	}

	home := os.Getenv("KIMI_CODE_HOME")
	if home == "" {
		if userHome, err := userHomeDir(); err == nil {
			home = filepath.Join(userHome, ".kimi-code")
		}
	}
	if home == "" {
		return "", "", false
	}

	return kimiTokenFromCredentials(filepath.Join(home, "credentials"))
}

// resolveKimiTokenForHome resolves the token for an explicit credential home
// (a kimi:<name> account row). Unlike the default row, the global
// KIMI_CODE_API_KEY env is deliberately skipped so two rows can never report
// the same account.
func resolveKimiTokenForHome(home string) (token, source string, expired bool) {
	if strings.TrimSpace(home) == "" {
		return resolveKimiToken()
	}
	return kimiTokenFromCredentials(filepath.Join(home, "credentials"))
}

// kimiTokenFromCredentials reads the static credential file, falling back to
// the newest rotated OAuth env-file, under one credentials directory.
func kimiTokenFromCredentials(credDir string) (token, source string, expired bool) {
	if data, err := os.ReadFile(filepath.Join(credDir, "kimi-code.json")); err == nil {
		if token := parseKimiAccessToken(data); token != "" {
			return token, "kimi-code.json", false
		}
	}

	// The current CLI rotates OAuth env-files (kimi-code-env-<id>.json);
	// pick the most recently written one.
	if newest := newestKimiCredentialPath(credDir, "kimi-code-env-*.json"); newest != "" {
		if data, err := os.ReadFile(newest); err == nil {
			if token := parseKimiAccessToken(data); token != "" {
				source := "kimi-code-env-*.json"
				expired := kimiTokenExpired(data)
				if expired {
					source += " (access_token expired; run kimi once to refresh)"
				}
				return token, source, expired
			}
		}
	}

	return "", "", false
}

// describeKimiCredential probes the Kimi token chain in fetch priority order:
// KIMI_CODE_API_KEY -> credentials/kimi-code.json -> the newest rotated
// kimi-code-env-*.json, annotating expired OAuth tokens.
func describeKimiCredential() CredentialStatus {
	return describeKimiCredentialForHome("")
}

// describeKimiCredentialForHome probes one credential home; an explicit home
// (kimi:<name> account row) skips the global env source.
func describeKimiCredentialForHome(home string) CredentialStatus {
	resolved := strings.TrimSpace(home)
	sources := make([]CredentialSource, 0, 3)
	if resolved == "" {
		sources = append(sources, CredentialSource{
			Kind:     CredentialKindEnv,
			Location: "KIMI_CODE_API_KEY",
			Found:    credentialEnvFound("KIMI_CODE_API_KEY"),
		})
		home = os.Getenv("KIMI_CODE_HOME")
		if home == "" {
			home = credentialHomePath(".kimi-code")
		}
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
	sources = append(sources, staticSource, rotatedSource)

	status := credentialStatus(sources)
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
	return fetchKimiUsageForHome(ctx, "", "kimi")
}

// fetchKimiUsageForHome is FetchKimiUsage for one credential home; an empty
// home resolves the default token chain, a kimi:<name> account row passes its
// configured directory.
func fetchKimiUsageForHome(ctx context.Context, home string, provider string) UsageResult {
	result := UsageResult{
		Provider: provider,
		Period:   "5h/7d",
		Unit:     "percent",
		Source:   "remote",
		Status:   "warn",
		Message:  "No data: Kimi Code token not found (run 'kimi login' or set KIMI_CODE_API_KEY)",
	}

	token, source, expired := resolveKimiTokenForHome(home)
	if token == "" {
		return result
	}
	result.SourceDetail = fmt.Sprintf("auth_source=%s", source)

	// An expired OAuth env-file can only produce a 401; short-circuit with
	// the actionable fix instead of surfacing the API's raw error body.
	if expired {
		result.Used = "n/a"
		result.Message = "Kimi Code OAuth token expired; run 'kimi' once to refresh it, or set KIMI_CODE_API_KEY"
		result.SourceDetail = fmt.Sprintf("auth_source=%s expired=true", source)
		return result
	}

	endpoint := os.Getenv("OCT_KIMI_USAGE_ENDPOINT")
	if endpoint == "" {
		endpoint = kimiUsageEndpoint
	}

	resp, err := fetchKimiUsage(ctx, endpoint, token)
	if err != nil {
		if errors.Is(err, errKimiNoUsageData) {
			// Authenticated and reachable, but the billing window has no
			// usage yet — degrade to warn instead of an alarming error.
			result.Status = "warn"
			result.Used = "n/a"
			result.Message = "Kimi API returned no usage data (window may not have started yet)"
			if osDebugEnabled() {
				result.SourceDetail = fmt.Sprintf("auth_source=%s endpoint=%s empty_payload=true", source, endpoint)
			}
			return result
		}
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
		return nil, errKimiNoUsageData
	}

	return &usageResp, nil
}
