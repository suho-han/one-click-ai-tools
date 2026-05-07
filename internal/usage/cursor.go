package usage

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/suho-han/one-click-tools/internal/netclient"
)

func FetchCursorUsage() UsageResult {
	endpoint := strings.TrimSpace(os.Getenv("OCT_CURSOR_USAGE_URL"))
	if endpoint == "" {
		local := FetchCursorLocalUsage()
		local.Message = "No configured Cursor remote usage endpoint; " + local.Message
		return local
	}

	req, _ := http.NewRequest("GET", endpoint, nil)
	if token := strings.TrimSpace(os.Getenv("CURSOR_API_KEY")); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := netclient.DefaultClient.DoWithRetry(req)
	if err != nil {
		local := FetchCursorLocalUsage()
		local.Message = fmt.Sprintf("%s; remote usage failed: %s", local.Message, netclient.FormatError(resp, err))
		return local
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		local := FetchCursorLocalUsage()
		local.Message = fmt.Sprintf("%s; remote usage failed: %s", local.Message, netclient.FormatError(resp, nil))
		return local
	}

	body, _ := io.ReadAll(resp.Body)
	parsed, err := parseCursorUsageResponse(body)
	if err != nil {
		local := FetchCursorLocalUsage()
		local.Message = fmt.Sprintf("%s; remote usage parse failed: %v", local.Message, err)
		return local
	}

	return parsed
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
		modelParts := make([]string, 0, len(models))
		for model, val := range models {
			if modelData, ok := val.(map[string]any); ok {
				if used, ok := firstFloat(modelData, "used", "usedPercent", "utilization"); ok {
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
