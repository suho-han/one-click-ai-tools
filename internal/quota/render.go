package quota

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// RenderBar returns a width-character bar showing `remaining` percent filled,
// clamped to [0, 100].
func RenderBar(remaining float64, width int) string {
	if width < 1 {
		width = 1
	}
	if remaining < 0 {
		remaining = 0
	}
	if remaining > 100 {
		remaining = 100
	}
	filled := int(remaining/100*float64(width) + 0.5)
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// ParseResetTime accepts the reset formats seen across providers: RFC 3339
// ("2026-09-26T08:00:00+09:00"), unix epoch seconds ("1790412359"), or unix
// epoch milliseconds ("1790412359000").
func ParseResetTime(raw string) (time.Time, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, false
	}
	if when, err := time.Parse(time.RFC3339, value); err == nil {
		return when, true
	}
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		if len(value) >= 13 { // milliseconds
			return time.UnixMilli(seconds), true
		}
		return time.Unix(seconds, 0), true
	}
	return time.Time{}, false
}

// FormatCountdown renders a duration like "3h 12m", "2d 4h", or "<1m".
// Negative durations (already reset) render as "<1m".
func FormatCountdown(d time.Duration) string {
	if d < time.Minute {
		return "<1m"
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes)
	default:
		return fmt.Sprintf("%dm", minutes)
	}
}

// HumanizeTokens renders a token count compactly: 742543210 -> "742.5M".
func HumanizeTokens(n int64) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", float64(n)/1_000_000_000)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return strconv.FormatInt(n, 10)
	}
}
