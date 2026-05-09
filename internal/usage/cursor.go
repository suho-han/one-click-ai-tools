package usage

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/suho-han/one-click-tools/internal/netclient"
)

const cursorDefaultAPIURL = "https://api2.cursor.sh/auth/usage"

func cursorAPIUsageURL() string {
	if override := strings.TrimSpace(os.Getenv("OCT_CURSOR_API_USAGE_URL")); override != "" {
		return override
	}
	return cursorDefaultAPIURL
}

func FetchCursorUsage() UsageResult {
	// 1. User-supplied custom endpoint takes priority
	if endpoint := strings.TrimSpace(os.Getenv("OCT_CURSOR_USAGE_URL")); endpoint != "" {
		return fetchCursorCustomEndpoint(endpoint)
	}

	// 2. Local auth token → known Cursor API
	if token := readCursorAuthToken(); token != "" {
		result, err := fetchCursorAPIUsage(token)
		if err == nil {
			return result
		}
		local := FetchCursorLocalUsage()
		local.Status = "warn"
		local.Message = cursorReasonMessage("local_auth_api_failed", fmt.Sprintf("%s; API call failed: %v", local.Message, err))
		return local
	}

	// 3. Workspace storage count fallback
	local := FetchCursorLocalUsage()
	local.Status = "warn"
	local.Message = cursorReasonMessage("local_auth_missing", "No Cursor auth token found; "+local.Message)
	return local
}

func fetchCursorCustomEndpoint(endpoint string) UsageResult {
	local := FetchCursorLocalUsage()
	req, _ := http.NewRequest("GET", endpoint, nil)
	if token := strings.TrimSpace(os.Getenv("CURSOR_API_KEY")); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := netclient.DefaultClient.DoWithRetry(req)
	if err != nil {
		local.Status = "warn"
		local.Message = cursorReasonMessage("remote_request_failed", fmt.Sprintf("%s; %s", local.Message, netclient.FormatError(resp, err)))
		return local
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		local.Status = "warn"
		local.Message = cursorReasonMessage("remote_http_error", fmt.Sprintf("%s; %s", local.Message, netclient.FormatError(resp, nil)))
		return local
	}

	body, _ := io.ReadAll(resp.Body)
	parsed, err := parseCursorUsageResponse(body)
	if err != nil {
		local.Status = "warn"
		local.Message = cursorReasonMessage("remote_parse_failed", fmt.Sprintf("%s; %v", local.Message, err))
		return local
	}

	if local.Used != "0" {
		parsed.Source = "remote+local"
		parsed.SourceDetail = appendCursorDetail(parsed.SourceDetail, "local_sessions="+local.Used)
	}

	return parsed
}

func readCursorAuthToken() string {
	home, _ := os.UserHomeDir()
	for _, path := range cursorAuthPaths(home) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var auth struct {
			AccessToken string `json:"accessToken"`
		}
		if json.Unmarshal(data, &auth) == nil && strings.TrimSpace(auth.AccessToken) != "" {
			return strings.TrimSpace(auth.AccessToken)
		}
	}
	return ""
}

func cursorAuthPaths(home string) []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			filepath.Join(home, ".config", "cursor", "auth.json"),
			filepath.Join(home, "Library", "Application Support", "cursor", "auth.json"),
		}
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return []string{filepath.Join(appData, "cursor", "auth.json")}
		}
		return nil
	default:
		return []string{filepath.Join(home, ".config", "cursor", "auth.json")}
	}
}

func fetchCursorAPIUsage(token string) (UsageResult, error) {
	req, err := http.NewRequest("GET", cursorAPIUsageURL(), nil)
	if err != nil {
		return UsageResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := netclient.DefaultClient.DoWithRetry(req)
	if err != nil {
		return UsageResult{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return UsageResult{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	return parseCursorAPIResponse(body)
}

func parseCursorAPIResponse(body []byte) (UsageResult, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return UsageResult{}, fmt.Errorf("parse failed")
	}

	result := UsageResult{
		Provider: "cursor",
		Period:   "monthly",
		Unit:     "requests",
		Source:   "local-auth",
		Status:   "ok",
		Message:  "Usage fetched from Cursor API",
		Buckets:  map[string]string{},
	}

	if rawSOM, ok := raw["startOfMonth"]; ok {
		var som string
		if json.Unmarshal(rawSOM, &som) == nil && len(som) >= 7 {
			result.Period = "monthly:" + som[:7]
		}
	}

	totalUsed := 0
	totalLimit := 0
	hasLimit := false
	var modelParts []string

	for key, val := range raw {
		if key == "startOfMonth" {
			continue
		}
		var m struct {
			NumRequestsTotal int  `json:"numRequestsTotal"`
			MaxRequestUsage  *int `json:"maxRequestUsage"`
		}
		if json.Unmarshal(val, &m) != nil {
			continue
		}
		totalUsed += m.NumRequestsTotal
		if m.MaxRequestUsage != nil {
			totalLimit += *m.MaxRequestUsage
			hasLimit = true
		}
		result.Buckets[key] = strconv.Itoa(m.NumRequestsTotal)
		modelParts = append(modelParts, fmt.Sprintf("%s=%d", key, m.NumRequestsTotal))
	}

	result.Used = strconv.Itoa(totalUsed)
	if hasLimit {
		result.Limit = strconv.Itoa(totalLimit)
	} else {
		result.Limit = "n/a"
	}
	if os.Getenv("OCT_USAGE_DEBUG") == "1" {
		result.SourceDetail = strings.Join(modelParts, ";")
	}

	return result, nil
}

func FetchCursorLocalUsage() UsageResult {
	result := UsageResult{
		Provider: "cursor",
		Period:   "local",
		Used:     "0",
		Limit:    "n/a",
		Unit:     "sessions",
		Source:   "local",
		Status:   "ok",
		Message:  "No local Cursor workspace storage found",
	}

	count, paths := countCursorWorkspaceStorage()
	if count > 0 {
		result.Used = strconv.Itoa(count)
		result.Message = "Estimated from local Cursor workspace storage"
		if os.Getenv("OCT_USAGE_DEBUG") == "1" {
			result.SourceDetail = strings.Join(paths, ";")
		}
	}

	return result
}

func parseCursorUsageResponse(body []byte) (UsageResult, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return UsageResult{}, fmt.Errorf("failed to parse Cursor usage response")
	}

	result := UsageResult{
		Provider: "cursor",
		Period:   "current",
		Used:     "0",
		Limit:    "100",
		Unit:     "percent",
		Source:   "remote",
		Status:   "ok",
		Message:  "Usage fetched from Cursor remote endpoint",
		Buckets:  map[string]string{},
	}

	if used, ok := firstFloat(raw,
		"used",
		"usedPercent",
		"used_percent",
		"usage",
		"utilization",
	); ok {
		result.Used = fmt.Sprintf("%.1f", used)
	}
	if limit, ok := firstFloat(raw,
		"limit",
		"limitPercent",
		"limit_percent",
		"quota",
	); ok && limit > 0 {
		result.Limit = trimFloat(limit)
	}
	if unit, ok := firstString(raw, "unit", "units"); ok {
		result.Unit = unit
	}
	if period, ok := firstString(raw, "period", "window"); ok {
		result.Period = period
	}
	if msg, ok := firstString(raw, "message", "statusMessage"); ok {
		result.Message = msg
	}

	if buckets, ok := raw["buckets"].(map[string]any); ok {
		for key, val := range buckets {
			if v, ok := anyToFloat(val); ok {
				result.Buckets[key] = fmt.Sprintf("%.1f", v)
			}
		}
	}

	if models, ok := raw["models"].(map[string]any); ok {
		modelNames := make([]string, 0, len(models))
		for model := range models {
			modelNames = append(modelNames, model)
		}
		sort.Strings(modelNames)

		modelParts := make([]string, 0, len(models))
		for _, model := range modelNames {
			val := models[model]
			if modelData, ok := val.(map[string]any); ok {
				if used, ok := firstFloat(modelData, "used", "usedPercent", "utilization"); ok {
					result.Buckets["model:"+model] = fmt.Sprintf("%.1f", used)
					modelParts = append(modelParts, fmt.Sprintf("%s=%.1f", model, used))
				}
			}
		}
		if len(modelParts) > 0 && os.Getenv("OCT_USAGE_DEBUG") == "1" {
			result.SourceDetail = strings.Join(modelParts, ";")
		}
	}

	if len(result.Buckets) == 0 && result.Used != "" && result.Used != "n/a" {
		result.Buckets["current"] = result.Used
	}

	return result, nil
}

func countCursorWorkspaceStorage() (int, []string) {
	var paths []string
	var roots []string
	home, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "darwin":
		roots = append(roots, filepath.Join(home, "Library", "Application Support", "Cursor", "User", "workspaceStorage"))
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			roots = append(roots, filepath.Join(appData, "Cursor", "User", "workspaceStorage"))
		}
	default:
		roots = append(roots, filepath.Join(home, ".config", "Cursor", "User", "workspaceStorage"))
	}

	count := 0
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		paths = append(paths, root)
		for _, entry := range entries {
			if entry.IsDir() {
				count++
			}
		}
	}

	return count, paths
}

func firstFloat(m map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		if v, ok := anyToFloat(m[key]); ok {
			return v, true
		}
	}
	return 0, false
}

func firstString(m map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		if val, ok := m[key]; ok {
			if s, ok := val.(string); ok && strings.TrimSpace(s) != "" {
				return s, true
			}
		}
	}
	return "", false
}

func anyToFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func trimFloat(v float64) string {
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10)
	}
	return fmt.Sprintf("%.1f", v)
}

func cursorReasonMessage(reason, detail string) string {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return "reason=" + reason
	}
	return "reason=" + reason + "; " + detail
}

func appendCursorDetail(existing, item string) string {
	existing = strings.TrimSpace(existing)
	item = strings.TrimSpace(item)
	if existing == "" {
		return item
	}
	if item == "" {
		return existing
	}
	return existing + ";" + item
}
