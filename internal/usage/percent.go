package usage

import (
	"fmt"
	"strconv"
	"strings"
)

// This file is the single home for percentage math shared across the usage
// table, compact titles, monitor columns, and provider fetchers. Inputs
// differ in meaning (a "used" percentage vs a "remaining" one); each helper
// states which it expects, so callers must not pre-invert.

// ParsePercent parses a percentage string, tolerating surrounding whitespace
// and a trailing "%" (stored values are raw numbers; display strings with a
// suffix are accepted defensively).
func ParsePercent(raw string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(raw), "%"), 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// ClampPercent clamps v to the 0..100 percentage range.
func ClampPercent(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// InvertPercentClamped converts a used percentage to a remaining percentage
// (100 - used), clamped to 0..100.
func InvertPercentClamped(used float64) float64 {
	return ClampPercent(100 - used)
}

// UsedFromRemainingPercent converts a remaining percentage to the
// clamped used percentage.
func UsedFromRemainingPercent(remaining float64) float64 {
	return ClampPercent(100 - remaining)
}

// RemainingFromUsedPercent converts a raw used-percentage string to a
// one-decimal remaining-percentage string ("45.0"); ok=false when the input
// is not a number. Values above 100 stay above 100 (only negatives floor to
// 0), matching the long-standing table/monitor rendering.
func RemainingFromUsedPercent(used string) (string, bool) {
	v, ok := ParsePercent(used)
	if !ok {
		return "", false
	}
	remaining := 100 - v
	if remaining < 0 {
		remaining = 0
	}
	return fmt.Sprintf("%.1f", remaining), true
}

// PercentLabel formats a raw (always "used") percentage string as a bare
// "NN%" remaining label (used -> remaining inversion). Callers must pass the
// raw stored value, never an already-formatted display string, or the
// inversion doubles up.
func PercentLabel(used string) (string, bool) {
	v, ok := ParsePercent(used)
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%.0f%%", ClampPercent(100-v)), true
}
