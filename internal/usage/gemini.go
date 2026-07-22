package usage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/suho-han/one-click-ai-tools/internal/netclient"
)

func FetchAntigravityLocalUsage() UsageResult {
	result := UsageResult{
		Provider:   "antigravity",
		Plan:       "unknown",
		PlanSource: "antigravity cli does not expose tier; see app settings",
		Period:     "local",
		Used:       "0",
		Limit:      "n/a",
		Unit:       "sessions",
		Source:     "local",
		Status:     "ok",
	}

	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		result.Status = "warn"
		result.Message = "Home directory unavailable; could not inspect local Antigravity sessions"
		return result
	}

	paths := antigravitySessionPaths(home)
	count, matched := countAntigravitySessions(paths)
	result.Used = fmt.Sprintf("%d", count)
	if len(matched) == 0 {
		result.Status = "warn"
		result.Message = "No local Antigravity sessions found"
	} else {
		result.Message = fmt.Sprintf("Estimated from %d local Antigravity session artifact(s)", count)
	}
	if os.Getenv("OCT_USAGE_DEBUG") == "1" {
		result.SourceDetail = strings.Join(matched, ",")
	}
	return result
}

func FetchAntigravityUsage() UsageResult {
	result := withPlanDetection(FetchAntigravityLocalUsage(), detectAntigravityPlan)
	if quotaResult, ok := fetchAntigravityQuotaUsage(result); ok {
		return quotaResult
	}
	return result
}

func FetchGeminiLocalUsage() UsageResult {
	return withPlanDetection(FetchAntigravityLocalUsage(), detectAntigravityPlan)
}

func FetchGeminiUsage() UsageResult {
	return FetchAntigravityUsage()
}

func antigravitySessionPaths(home string) []string {
	return []string{
		filepath.Join(home, ".gemini", "antigravity", "conversations"),
		filepath.Join(home, ".gemini", "antigravity-cli", "cache"),
		filepath.Join(home, ".gemini", "antigravity-cli", "projects"),
	}
}

func countAntigravitySessions(paths []string) (int, []string) {
	total := 0
	matched := make([]string, 0, len(paths))
	seen := map[string]bool{}

	for _, root := range paths {
		entries, err := os.ReadDir(root)
		if err != nil || len(entries) == 0 {
			continue
		}
		pathCount := 0
		for _, entry := range entries {
			name := strings.ToLower(entry.Name())
			switch {
			case entry.IsDir():
				pathCount++
			case strings.HasSuffix(name, ".pb"), strings.HasSuffix(name, ".db"), strings.HasSuffix(name, ".sqlite"), strings.HasSuffix(name, ".json"), strings.HasSuffix(name, ".jsonl"):
				pathCount++
			}
		}
		if pathCount == 0 {
			continue
		}
		total += pathCount
		clean := filepath.Clean(root)
		if !seen[clean] {
			seen[clean] = true
			matched = append(matched, clean)
		}
	}

	sort.Strings(matched)
	return total, matched
}

type antigravityOAuthCredentials struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiryDate   int64  `json:"expiry_date"`
}

type antigravityQuotaBucket struct {
	RemainingFraction float64 `json:"remainingFraction"`
	ResetTime         string  `json:"resetTime"`
	ModelID           string  `json:"modelId"`
}

type antigravityQuotaResponse struct {
	Buckets []antigravityQuotaBucket `json:"buckets"`
}

func fetchAntigravityQuotaUsage(base UsageResult) (UsageResult, bool) {
	creds, ok := readAntigravityOAuthCredentials()
	if !ok || strings.TrimSpace(creds.AccessToken) == "" {
		return base, false
	}

	projectID, err := loadAntigravityProjectID(creds.AccessToken)
	if err != nil || strings.TrimSpace(projectID) == "" {
		return base, false
	}

	buckets, err := retrieveAntigravityQuota(creds.AccessToken, projectID)
	if err != nil || len(buckets) == 0 {
		return base, false
	}

	result := base
	result.Period = "current"
	result.Source = "quota"
	result.Status = "ok"
	result.Unit = "percent"
	result.Limit = "100"
	result.Buckets = map[string]string{}

	maxUsed := 0.0
	modelParts := make([]string, 0, len(buckets))
	for _, bucket := range dedupeAntigravityQuotaBuckets(buckets) {
		used := antigravityUsedPercent(bucket.RemainingFraction)
		if used > maxUsed {
			maxUsed = used
		}
		name := antigravityBucketName(bucket.ModelID)
		result.Buckets["model:"+name] = fmt.Sprintf("%.1f", used)
		modelParts = append(modelParts, fmt.Sprintf("%s=%.1f", name, used))
	}

	result.Used = fmt.Sprintf("%.1f", maxUsed)
	result.Message = "Usage fetched from Google Code Assist quota API"
	if os.Getenv("OCT_USAGE_DEBUG") == "1" {
		result.SourceDetail = strings.Join(modelParts, ";")
	}
	return result, true
}

func readAntigravityOAuthCredentials() (antigravityOAuthCredentials, bool) {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return antigravityOAuthCredentials{}, false
	}
	data, err := os.ReadFile(filepath.Join(home, ".gemini", "oauth_creds.json"))
	if err != nil {
		return antigravityOAuthCredentials{}, false
	}
	var creds antigravityOAuthCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return antigravityOAuthCredentials{}, false
	}
	if creds.ExpiryDate > 0 && creds.ExpiryDate < time.Now().Add(-time.Minute).UnixMilli() {
		return antigravityOAuthCredentials{}, false
	}
	return creds, strings.TrimSpace(creds.AccessToken) != ""
}

func loadAntigravityProjectID(accessToken string) (string, error) {
	body := []byte(`{"metadata":{"ideType":"GEMINI_CLI","pluginType":"GEMINI"}}`)
	req, err := http.NewRequest("POST", antigravityProjectEndpoint(), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := netclient.DefaultClient.DoWithRetry(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("load project failed: HTTP %d", resp.StatusCode)
	}

	raw, _ := io.ReadAll(resp.Body)
	var payload struct {
		CloudAICompanionProject string `json:"cloudaicompanionProject"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", err
	}
	return strings.TrimSpace(payload.CloudAICompanionProject), nil
}

func retrieveAntigravityQuota(accessToken string, projectID string) ([]antigravityQuotaBucket, error) {
	payload, _ := json.Marshal(map[string]string{"project": projectID})
	req, err := http.NewRequest("POST", antigravityQuotaEndpoint(), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := netclient.DefaultClient.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("quota fetch failed: HTTP %d", resp.StatusCode)
	}

	raw, _ := io.ReadAll(resp.Body)
	var wrapped antigravityQuotaResponse
	if err := json.Unmarshal(raw, &wrapped); err == nil && len(wrapped.Buckets) > 0 {
		return filterAntigravityQuotaBuckets(wrapped.Buckets), nil
	}
	var direct []antigravityQuotaBucket
	if err := json.Unmarshal(raw, &direct); err != nil {
		return nil, err
	}
	return filterAntigravityQuotaBuckets(direct), nil
}

func filterAntigravityQuotaBuckets(buckets []antigravityQuotaBucket) []antigravityQuotaBucket {
	filtered := make([]antigravityQuotaBucket, 0, len(buckets))
	for _, bucket := range buckets {
		if strings.TrimSpace(bucket.ModelID) == "" {
			continue
		}
		if bucket.RemainingFraction < 0 || bucket.RemainingFraction > 1 {
			continue
		}
		filtered = append(filtered, bucket)
	}
	return filtered
}

func dedupeAntigravityQuotaBuckets(buckets []antigravityQuotaBucket) []antigravityQuotaBucket {
	result := make([]antigravityQuotaBucket, 0, len(buckets))
	seen := map[string]int{}
	for _, bucket := range buckets {
		key := fmt.Sprintf("%.4f|%s", bucket.RemainingFraction, bucket.ResetTime)
		if idx, ok := seen[key]; ok {
			if len(antigravityBucketName(bucket.ModelID)) < len(antigravityBucketName(result[idx].ModelID)) {
				result[idx] = bucket
			}
			continue
		}
		seen[key] = len(result)
		result = append(result, bucket)
	}
	return result
}

func antigravityUsedPercent(remainingFraction float64) float64 {
	used := (1 - remainingFraction) * 100
	if used < 0 {
		return 0
	}
	if used > 100 {
		return 100
	}
	return used
}

func antigravityProjectEndpoint() string {
	if endpoint := strings.TrimSpace(os.Getenv("OCT_GEMINI_API_ENDPOINT")); endpoint != "" {
		return endpoint
	}
	return "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist"
}

func antigravityQuotaEndpoint() string {
	if endpoint := strings.TrimSpace(os.Getenv("OCT_GEMINI_USAGE_ENDPOINT")); endpoint != "" {
		return endpoint
	}
	return "https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuota"
}

func antigravityBucketName(modelID string) string {
	switch strings.ToLower(strings.TrimSpace(modelID)) {
	case "gemini-3.1-pro":
		return "3.1 Pro"
	case "gemini-3.1-flash":
		return "3.1 Flash"
	case "gemini-3.1-flash-lite":
		return "3.1 Flash Lite"
	case "gemini-3.0-pro":
		return "3.0 Pro"
	case "gemini-3.0-flash":
		return "3.0 Flash"
	case "gemini-2.5-pro":
		return "Pro"
	case "gemini-2.5-flash":
		return "Flash"
	case "gemini-2.5-flash-lite":
		return "Flash Lite"
	default:
		trimmed := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(modelID)), "gemini-")
		parts := strings.Split(trimmed, "-")
		for i, part := range parts {
			if part == "" {
				continue
			}
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
		return strings.Join(parts, " ")
	}
}
