package usage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	return withPlanDetection(FetchAntigravityLocalUsage(), detectAntigravityPlan)
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
