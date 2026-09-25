package usage

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

func RenderCompactRemaining(w io.Writer, results []UsageResult) {
	fmt.Fprintln(w, CompactRemainingTitle(results))
}

// CompactRemainingTitle renders the "P-NN%" compact title used by
// `oct usage --compact`. It is intentionally always "remaining" -- the
// documented contract of the --compact flag ("Output compact remaining
// usage").
func CompactRemainingTitle(results []UsageResult) string {
	return CompactTitle(results)
}

// CompactTitle renders the "P-NN%" compact title (always remaining usage).
// It backs both CompactRemainingTitle and the menubar's compact title mode,
// so the status-bar title never disagrees with the popover/dropdown body
// below it.
func CompactTitle(results []UsageResult) string {
	parts := make([]string, 0, len(results))
	for _, result := range results {
		parts = append(parts, compactProviderLabel(result.Provider)+"-"+compactValue(result))
	}
	return strings.Join(parts, " ")
}

func compactProviderLabel(provider string) string {
	// <provider>:<name> account rows share the base provider's letter; the
	// alias's first rune distinguishes them (codex:work -> XW,
	// kimi:personal -> KP) instead of two identical "X-NN%" tokens on
	// statusbar surfaces.
	if base, alias, ok := SplitAccountProviderLabel(provider); ok {
		if entry, ok := matchProvider(base); ok && entry.CompactLabel != "" {
			runes := []rune(alias)
			return entry.CompactLabel + strings.ToUpper(string(runes[0]))
		}
	}
	if entry, ok := matchProvider(provider); ok && entry.CompactLabel != "" {
		return entry.CompactLabel
	}
	p := strings.ToLower(strings.TrimSpace(provider))
	if p == "" {
		return "?"
	}
	return strings.ToUpper(string([]rune(p)[0]))
}

func compactValue(r UsageResult) string {
	// "quota" (e.g. Copilot) is always a pre-computed used-percentage even
	// when r.Unit is a count unit like "AIC", not "percent" -- check it
	// before the percent-unit gate below, matching usageSummaryDisplay's
	// quota handling, or Copilot always renders "?" here while the `oct
	// usage` table and the Swift menubar show a real number for the exact
	// same data.
	if quota := strings.TrimSpace(r.Buckets["quota"]); quota != "" {
		if label, ok := PercentLabel(quota); ok {
			return label
		}
	}
	if !strings.EqualFold(r.Unit, "percent") {
		return "?"
	}
	if raw, ok := compactUsageMetric(r); ok {
		if label, ok := PercentLabel(raw); ok {
			return label
		}
	}
	if label, ok := PercentLabel(r.Used); ok {
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
	// Fall back to the first per-model bucket's raw (used) value; PercentLabel
	// applies the remaining inversion.
	if raw, ok := firstModelBucketValue(r); ok {
		return raw, true
	}
	return "", false
}

// firstModelBucketValue returns the raw stored value of the first "model:*"
// bucket (sorted by key), matching modelBucketDisplays' ordering.
func firstModelBucketValue(r UsageResult) (string, bool) {
	keys := make([]string, 0, len(r.Buckets))
	for key := range r.Buckets {
		if strings.HasPrefix(key, "model:") {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return "", false
	}
	sort.Strings(keys)
	for _, key := range keys {
		if value := strings.TrimSpace(r.Buckets[key]); value != "" {
			return value, true
		}
	}
	return "", false
}
