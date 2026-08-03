package usage

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

func RenderCompactRemaining(w io.Writer, results []UsageResult) {
	fmt.Fprintln(w, CompactRemainingTitle(results))
}

func CompactRemainingTitle(results []UsageResult) string {
	parts := make([]string, 0, len(results))
	for _, result := range results {
		parts = append(parts, compactProviderLabel(result.Provider)+"-"+compactRemainingValue(result))
	}
	return strings.Join(parts, " ")
}

func compactProviderLabel(provider string) string {
	p := strings.ToLower(strings.TrimSpace(provider))
	switch {
	case strings.Contains(p, "claude"):
		return "C"
	case strings.Contains(p, "codex"), strings.Contains(p, "openai"):
		return "X"
	case strings.Contains(p, "antigravity"), strings.Contains(p, "gemini"), p == "agy":
		return "G"
	case strings.Contains(p, "cursor"):
		return "R"
	case strings.Contains(p, "copilot"), strings.Contains(p, "github"):
		return "P"
	case strings.Contains(p, "opencode"):
		return "O"
	}
	if p == "" {
		return "?"
	}
	return strings.ToUpper(string([]rune(p)[0]))
}

func compactRemainingValue(r UsageResult) string {
	if !strings.EqualFold(r.Unit, "percent") {
		return "?"
	}
	if raw, ok := compactUsageMetric(r); ok {
		if label, ok := remainingPercentLabel(raw); ok {
			return label
		}
	}
	if label, ok := remainingPercentLabel(r.Used); ok {
		return label
	}
	return "?"
}

func compactUsageMetric(r UsageResult) (string, bool) {
	if len(r.Buckets) == 0 {
		return "", false
	}
	provider := strings.ToLower(strings.TrimSpace(r.Provider))
	labels := []string{"5h", "7d"}
	if strings.Contains(provider, "codex") {
		labels = []string{"7d", "5h"}
	}
	for _, label := range labels {
		if value := strings.TrimSpace(r.Buckets[label]); value != "" {
			return value, true
		}
	}
	if modelParts := modelBucketDisplays(r, "used"); len(modelParts) > 0 {
		fields := strings.Fields(modelParts[0])
		if len(fields) > 0 {
			return fields[len(fields)-1], true
		}
	}
	return "", false
}

func remainingPercentLabel(used string) (string, bool) {
	v, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(used), "%"), 64)
	if err != nil {
		return "", false
	}
	remaining := 100 - v
	if remaining < 0 {
		remaining = 0
	}
	if remaining > 100 {
		remaining = 100
	}
	return fmt.Sprintf("%.0f%%", remaining), true
}
