package usage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/suho-han/one-click-tools/internal/netclient"
)

func FetchGeminiLocalUsage() UsageResult {
	result := UsageResult{
		Provider: "gemini",
		Period:   "local",
		Used:     "0",
		Limit:    "n/a",
		Unit:     "sessions",
		Source:   "local",
		Status:   "ok",
	}

	home, _ := os.UserHomeDir()
	convDir := filepath.Join(home, ".gemini", "antigravity", "conversations")

	files, err := os.ReadDir(convDir)
	if err != nil || len(files) == 0 {
		result.Message = "No local Gemini conversations found"
		return result
	}

	count := 0
	for _, f := range files {
		if !f.IsDir() && filepath.Ext(f.Name()) == ".pb" {
			count++
		}
	}

	result.Used = fmt.Sprintf("%d", count)
	result.Message = "Total local conversation sessions found"
	return result
}

func FetchGeminiUsage() UsageResult {
	home, _ := os.UserHomeDir()
	oauthFile := filepath.Join(home, ".gemini", "oauth_creds.json")

	result := UsageResult{
		Provider: "gemini",
		Period:   "current",
		Used:     "n/a",
		Limit:    "100",
		Unit:     "percent",
		Source:   "oauth",
		Status:   "error",
	}

	var data []byte
	data, err := os.ReadFile(oauthFile)
	var creds struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiryDate   int64  `json:"expiry_date"`
	}
	if err == nil {
		if err := json.Unmarshal(data, &creds); err != nil {
			result.Message = "Failed to parse OAuth credentials"
			return result
		}
	}

	token := creds.AccessToken
	if token == "" {
		if gcloudToken, gErr := commandOutput(3*time.Second, "gcloud", "auth", "print-access-token"); gErr == nil && gcloudToken != "" {
			token = gcloudToken
			result.Source = "gcloud-cli"
		}
	}
	if token == "" {
		return FetchGeminiLocalUsage()
	}

	if err == nil && creds.ExpiryDate > 0 {
		// Token refresh logic would go here if expiry_date is past.
		nowMs := time.Now().UnixNano() / 1e6
		if creds.ExpiryDate <= (nowMs+60000) && creds.RefreshToken != "" {
			result.Message = "Token expired, refresh required (auto-refresh not fully implemented)"
		}
	}

	// 1. loadCodeAssist to get project ID
	projectID, err := loadGeminiProjectID(token)
	if err != nil {
		// Fallback to local session summary when CLI/API authentication path fails.
		local := FetchGeminiLocalUsage()
		local.Message = fmt.Sprintf("%s; API fallback failed: %v", local.Message, err)
		return local
	}

	// 2. retrieveUserQuota
	used, limit, buckets, debugDetail, err := retrieveGeminiQuota(token, projectID)
	if err != nil {
		local := FetchGeminiLocalUsage()
		local.Message = fmt.Sprintf("%s; quota API failed: %v", local.Message, err)
		return local
	}

	result.Status = "ok"
	result.Used = fmt.Sprintf("%.0f", used)
	result.Limit = fmt.Sprintf("%.0f", limit)
	result.Buckets = buckets
	if os.Getenv("OCT_USAGE_DEBUG") == "1" {
		result.SourceDetail = debugDetail
	}
	result.Message = "Quota fetched via Cloud Code API"
	return result
}

func loadGeminiProjectID(token string) (string, error) {
	url := "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist"
	payload := []byte(`{"metadata":{"ideType":"IDE_UNSPECIFIED","platform":"PLATFORM_UNSPECIFIED","pluginType":"GEMINI"}}`)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := netclient.DefaultClient.DoWithRetry(req)
	if err != nil {
		return "", fmt.Errorf("%s", netclient.FormatError(resp, err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s", netclient.FormatError(resp, nil))
	}

	var data struct {
		ProjectID string `json:"cloudaicompanionProject"`
	}
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("failed to parse loadCodeAssist response")
	}
	if data.ProjectID == "" {
		return "", fmt.Errorf("empty Gemini project ID")
	}
	return data.ProjectID, nil
}

func retrieveGeminiQuota(token, projectID string) (float64, float64, map[string]string, string, error) {
	url := "https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuota"
	payload := []byte(fmt.Sprintf(`{"project":"%s"}`, projectID))

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := netclient.DefaultClient.DoWithRetry(req)
	if err != nil {
		return 0, 0, nil, "", fmt.Errorf("%s", netclient.FormatError(resp, err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, nil, "", fmt.Errorf("%s", netclient.FormatError(resp, nil))
	}

	body, _ := io.ReadAll(resp.Body)
	var raw struct {
		Buckets []map[string]any `json:"buckets"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return 0, 0, nil, "", fmt.Errorf("failed to parse retrieveUserQuota response")
	}

	buckets := make(map[string]string)
	var fallbackUsed float64 = -1
	var seen []string

	for _, b := range raw.Buckets {
		remaining, ok := extractFloat(b, "remainingFraction")
		if !ok {
			continue
		}
		used := (1.0 - remaining) * 100
		key := classifyGeminiBucket(b)
		if key != "" {
			buckets[key] = fmt.Sprintf("%.1f", used)
			seen = append(seen, fmt.Sprintf("%s:%s", key, bucketIdentity(b)))
		} else if fallbackUsed < 0 {
			fallbackUsed = used
			seen = append(seen, "unknown:"+bucketIdentity(b))
		}
	}

	if len(buckets) == 0 && fallbackUsed < 0 {
		return 0, 0, nil, strings.Join(seen, ","), fmt.Errorf("no quota buckets found")
	}

	used := fallbackUsed
	if v, ok := buckets["5h"]; ok {
		fmt.Sscanf(v, "%f", &used)
	} else if v, ok := buckets["7d"]; ok {
		fmt.Sscanf(v, "%f", &used)
	}
	if used < 0 {
		used = 0
	}

	return used, 100, buckets, strings.Join(seen, ","), nil
}

func classifyGeminiBucket(bucket map[string]any) string {
	candidates := []string{
		anyToString(bucket["bucketId"]),
		anyToString(bucket["modelId"]),
		anyToString(bucket["period"]),
		anyToString(bucket["window"]),
		anyToString(bucket["duration"]),
		anyToString(bucket["quotaId"]),
	}
	joined := strings.ToLower(strings.Join(candidates, " "))
	switch {
	case strings.Contains(joined, "5h"), strings.Contains(joined, "five_hour"), strings.Contains(joined, "fivehour"):
		return "5h"
	case strings.Contains(joined, "7d"), strings.Contains(joined, "seven_day"), strings.Contains(joined, "weekly"), strings.Contains(joined, "week"):
		return "7d"
	default:
		return ""
	}
}

func bucketIdentity(bucket map[string]any) string {
	keys := []string{"bucketId", "modelId", "period", "window", "duration", "quotaId"}
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		if v := anyToString(bucket[k]); v != "" {
			parts = append(parts, k+"="+v)
		}
	}
	return strings.Join(parts, "|")
}

func anyToString(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case fmt.Stringer:
		return x.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func extractFloat(m map[string]any, key string) (float64, bool) {
	v, ok := m[key]
	if !ok || v == nil {
		return 0, false
	}
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	default:
		return 0, false
	}
}
