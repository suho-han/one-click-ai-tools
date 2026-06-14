package usage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

var userHomeDir = os.UserHomeDir

func FetchOpenCodeUsage() UsageResult {
	result := UsageResult{
		Provider:   "opencode",
		Plan:       "unknown",
		PlanSource: "local opencode session logs do not expose plan",
		Period:     "current",
		Used:       "0",
		Limit:      "100",
		Unit:       "percent",
		Source:     "local",
		Status:     "warn",
		Message:    "No data: No local OpenCode session logs found",
	}

	result = withPlanDetection(result, detectOpenCodePlan)

	logFiles := collectOpenCodeLogFiles()
	if len(logFiles) == 0 {
		return result
	}

	sort.Strings(logFiles)
	latest := logFiles[len(logFiles)-1]

	used, weekly, ok, err := parseOpenCodeUsageFromJSONL(latest)
	if err != nil {
		result.Status = "error"
		result.Used = "n/a"
		result.Message = fmt.Sprintf("Parse error: failed to parse OpenCode log: %v", err)
		return result
	}
	if !ok {
		result.Message = "No data: Latest OpenCode session log has no usage metrics"
		if os.Getenv("OCT_USAGE_DEBUG") == "1" {
			result.SourceDetail = "latest_log=" + latest
		}
		return result
	}

	result.Buckets = map[string]string{}
	if used != "" {
		result.Buckets["5h"] = used
		result.Used = used
	}
	if weekly != "" {
		result.Buckets["7d"] = weekly
		if result.Used == "0" || result.Used == "" {
			result.Used = weekly
		}
	}
	result.Status = "ok"
	result.Message = "Fetched from local OpenCode session logs"
	if os.Getenv("OCT_USAGE_DEBUG") == "1" {
		result.SourceDetail = "latest_log=" + latest
	}
	return result
}

func collectOpenCodeLogFiles() []string {
	home, _ := userHomeDir()
	candidates := []string{
		filepath.Join(home, ".opencode", "sessions"),
		filepath.Join(home, ".config", "opencode", "sessions"),
		filepath.Join(home, ".local", "share", "opencode", "sessions"),
	}

	var out []string
	for _, root := range candidates {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			name := strings.ToLower(info.Name())
			if strings.HasSuffix(name, ".jsonl") || strings.HasSuffix(name, ".json") {
				out = append(out, path)
			}
			return nil
		})
	}
	return out
}

func parseOpenCodeUsageFromJSONL(path string) (used string, weekly string, ok bool, err error) {
	file, err := os.Open(path)
	if err != nil {
		return "", "", false, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		var raw map[string]any
		if jerr := json.Unmarshal(line, &raw); jerr != nil {
			continue
		}

		if v, found := findNumberByKeys(raw,
			"used_percent", "usedPercent", "utilization", "usage_percent", "usagePercent",
		); found {
			used = trimFloat(v)
			ok = true
		}

		if sec, found := findNestedMap(raw, "secondary"); found {
			window, _ := findNumberByKeys(sec, "window_minutes", "windowMinutes")
			if wv, foundW := findNumberByKeys(sec,
				"used_percent", "usedPercent", "utilization", "usage_percent", "usagePercent",
			); foundW && window >= 10080 {
				weekly = trimFloat(wv)
				ok = true
			}
		}

		if rl, found := findNestedMap(raw, "rate_limits"); found {
			if prim, okPrim := findNestedMap(rl, "primary"); okPrim {
				if pv, foundP := findNumberByKeys(prim, "used_percent", "usedPercent", "utilization"); foundP {
					used = trimFloat(pv)
					ok = true
				}
			}
			if sec, okSec := findNestedMap(rl, "secondary"); okSec {
				window, _ := findNumberByKeys(sec, "window_minutes", "windowMinutes")
				if sv, foundS := findNumberByKeys(sec, "used_percent", "usedPercent", "utilization"); foundS && window >= 10080 {
					weekly = trimFloat(sv)
					ok = true
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", "", false, err
	}
	return used, weekly, ok, nil
}

func findNumberByKeys(m map[string]any, keys ...string) (float64, bool) {
	for _, key := range keys {
		if val, exists := m[key]; exists {
			switch x := val.(type) {
			case float64:
				return x, true
			case float32:
				return float64(x), true
			case int:
				return float64(x), true
			case int64:
				return float64(x), true
			case string:
				f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
				if err == nil {
					return f, true
				}
			}
		}
	}
	return 0, false
}

func findNestedMap(m map[string]any, key string) (map[string]any, bool) {
	if v, ok := m[key]; ok {
		if out, ok := v.(map[string]any); ok {
			return out, true
		}
	}
	if payload, ok := m["payload"].(map[string]any); ok {
		if v, ok := payload[key]; ok {
			if out, ok := v.(map[string]any); ok {
				return out, true
			}
		}
	}
	return nil, false
}
