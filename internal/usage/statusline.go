package usage

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Severity colors for statusbar renderers (waybar class, polybar %{F#...},
// SwiftBar color=). Shared thresholds live in UsageSeverity; the palette is
// deliberately severity-based (green/amber/red at a glance) rather than
// per-provider brand colors, since these one-letter tokens are tiny.
const (
	severityColorOK    = "#a6e3a1"
	severityColorWarn  = "#f9e2af"
	severityColorError = "#f38ba8"
)

// UsageSeverity classifies one result for narrow statusbar surfaces, using
// the same thresholds as `oct monitor`: a percent result at >= 85 used is
// WARN and >= 95 is CRIT regardless of its own status, "error" status is
// CRIT, "warn" status is WARN, and anything without a usable percent value
// is UNKNOWN (callers decide how to render that). Kept in lockstep with
// cmd/monitor.go's usageSeverity so the statusline never disagrees with the
// monitor table on the same data.
func UsageSeverity(r UsageResult) string {
	status := strings.ToLower(strings.TrimSpace(r.Status))
	switch status {
	case "error":
		return "CRIT"
	case "warn":
		return "WARN"
	}
	if !strings.EqualFold(r.Unit, "percent") {
		return "UNKNOWN"
	}
	maxV := -1.0
	if v, ok := parseSeverityPercent(r.Used); ok {
		maxV = v
	}
	for _, raw := range r.Buckets {
		if v, ok := parseSeverityPercent(raw); ok && v > maxV {
			maxV = v
		}
	}
	if maxV < 0 {
		return "UNKNOWN"
	}
	if maxV >= 95 {
		return "CRIT"
	}
	if maxV >= 85 {
		return "WARN"
	}
	return "OK"
}

func parseSeverityPercent(raw string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(raw), "%")), 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// StatuslineSeverity folds per-result severities into one bar-level class:
// "ok", "warn", or "error". UNKNOWN (non-percent or dataless providers)
// never escalates the bar by itself.
func StatuslineSeverity(results []UsageResult) string {
	class := "ok"
	for _, result := range results {
		switch UsageSeverity(result) {
		case "WARN":
			if class == "ok" {
				class = "warn"
			}
		case "CRIT":
			return "error"
		}
	}
	return class
}

func severityColor(class string) string {
	switch class {
	case "warn":
		return severityColorWarn
	case "error":
		return severityColorError
	default:
		return severityColorOK
	}
}

// statuslineMode normalizes the display mode; statusline surfaces honor
// usage_display_mode the way the menubar title does (only --compact pins
// "remaining").
func statuslineMode(mode string) string {
	return NormalizeDisplayMode(mode)
}

// RenderWaybarJSON writes a one-line waybar custom-module JSON object:
// {"text": "...", "tooltip": "...", "class": "ok|warn|error"}. Point waybar's
// custom/script module at `oct usage --format waybar`.
func RenderWaybarJSON(w io.Writer, results []UsageResult, mode string) error {
	mode = statuslineMode(mode)
	text := CompactTitle(results, mode)
	if strings.TrimSpace(text) == "" {
		text = "-"
	}
	var tooltip strings.Builder
	for i, result := range results {
		if i > 0 {
			tooltip.WriteByte('\n')
		}
		tooltip.WriteString(statuslineProviderLine(result, mode))
	}
	payload := struct {
		Text    string `json:"text"`
		Tooltip string `json:"tooltip"`
		Class   string `json:"class"`
	}{
		Text:    text,
		Tooltip: tooltip.String(),
		Class:   StatuslineSeverity(results),
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}

// RenderPolybarLine writes a one-line polybar custom-script payload: the
// compact title with each token colored by its severity. Literal "%" is
// doubled (polybar format escaping) and ANSI is never emitted.
func RenderPolybarLine(w io.Writer, results []UsageResult, mode string) error {
	mode = statuslineMode(mode)
	var b strings.Builder
	for _, result := range results {
		token := compactProviderLabel(result.Provider) + "-" + compactValueForMode(result, mode)
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		if class := statuslineClassFor(result); class != "ok" {
			b.WriteString("%{F" + severityColor(class) + "}" + polybarEscape(token) + "%{F-}")
			continue
		}
		b.WriteString(polybarEscape(token))
	}
	if b.Len() == 0 {
		b.WriteString("-")
	}
	_, err := fmt.Fprintln(w, b.String())
	return err
}

// polybarEscape doubles literal percent signs so polybar renders them
// instead of interpreting them as format specifiers.
func polybarEscape(s string) string {
	return strings.ReplaceAll(s, "%", "%%")
}

// RenderSwiftBar writes the SwiftBar plugin protocol: the first line is the
// menu-bar title (with a severity color param), "---" separates it from the
// per-provider dropdown lines, each colored by its own severity.
func RenderSwiftBar(w io.Writer, results []UsageResult, mode string) error {
	mode = statuslineMode(mode)
	title := CompactTitle(results, mode)
	if strings.TrimSpace(title) == "" {
		title = "oct"
	}
	if _, err := fmt.Fprintf(w, "%s | color=%s\n", title, severityColor(StatuslineSeverity(results))); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "---"); err != nil {
		return err
	}
	for _, result := range results {
		line := statuslineProviderLine(result, mode)
		if _, err := fmt.Fprintf(w, "%s | color=%s\n", line, severityColor(statuslineClassFor(result))); err != nil {
			return err
		}
	}
	return nil
}

// statuslineClassFor maps one result to the bar-level class names used by
// waybar/SwiftBar ("ok"|"warn"|"error"); UNKNOWN collapses to ok.
func statuslineClassFor(result UsageResult) string {
	switch UsageSeverity(result) {
	case "WARN":
		return "warn"
	case "CRIT":
		return "error"
	default:
		return "ok"
	}
}

// statuslineProviderLine renders the "[status] name (plan) · 5h x · 7d y ·
// 1m z" dropdown line, mirroring the menubar's provider line so the same
// result reads the same across statusbar surfaces.
func statuslineProviderLine(result UsageResult, mode string) string {
	name := strings.TrimSpace(result.Provider)
	if name == "" {
		name = "Unknown"
	}
	if plan := strings.TrimSpace(result.Plan); plan != "" && !strings.EqualFold(plan, "unknown") {
		name += " (" + plan + ")"
	}
	five := statuslineBucket(result, "5h", mode)
	seven := statuslineBucket(result, "7d", mode)
	month := statuslineBucket(result, "1m", mode)
	metrics := fmt.Sprintf("5h %s · 7d %s · 1m %s", five, seven, month)
	if five == "-" && seven == "-" && month == "-" {
		if fallback, ok := FallbackMetricsSummary(result, mode); ok {
			metrics = fallback
		}
	}
	class := statuslineClassFor(result)
	line := fmt.Sprintf("[%s] %s · %s", class, name, metrics)
	// 96 rather than the menubar's tighter budget: these lines are tooltip /
	// dropdown rows where the actionable tail of an error message ("...check
	// your credentials using 'oct config'") is the whole point.
	if msg := strings.TrimSpace(result.Message); msg != "" && class != "ok" {
		line += " · " + truncateText(msg, 96)
	}
	return line
}

// statuslineBucket resolves a bucket's display value with the same
// used/remaining inversion as `oct monitor`'s bucketVal.
func statuslineBucket(result UsageResult, key, mode string) string {
	v := strings.TrimSpace(result.Buckets[key])
	if v == "" {
		return "-"
	}
	if strings.EqualFold(result.Unit, "percent") {
		if mode == DisplayModeRemaining {
			if rem, ok := RemainingFromUsedPercent(v); ok {
				v = rem
			}
		}
		v += "%"
	}
	return v
}
