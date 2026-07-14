package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/suho-han/one-click-ai-tools/internal/update"
	"github.com/suho-han/one-click-ai-tools/internal/usage"
)

type menubarSnapshot struct {
	Title           string
	Tooltip         string
	SummaryLine     string
	UpdatedLine     string
	LastRefreshAt   time.Time
	ProviderLines   []string
	ProviderDetails [][]string
}

func menubarOverviewTitle() string {
	return "Usage Overview"
}

func menubarProviderSectionTitle(count int) string {
	if count <= 0 {
		return "Providers"
	}
	return fmt.Sprintf("Providers (%d)", count)
}

func buildMenubarLoadingSnapshot(toolNames []string) menubarSnapshot {
	lines := make([]string, 0, len(toolNames))
	details := make([][]string, 0, len(toolNames))
	for _, name := range toolNames {
		lines = append(lines, fmt.Sprintf("%s · loading…", name))
		details = append(details, []string{
			"Provider: " + name,
			"Status: loading",
		})
	}
	return menubarSnapshot{
		Title:           "oct …",
		Tooltip:         "one-click-tools menubar loading",
		SummaryLine:     fmt.Sprintf("Loading usage for %d provider(s)…", len(toolNames)),
		UpdatedLine:     "Last refresh: -",
		LastRefreshAt:   time.Time{},
		ProviderLines:   lines,
		ProviderDetails: details,
	}
}

func buildMenubarUsageSnapshot(results []usage.UsageResult, now time.Time) menubarSnapshot {
	okCount, warnCount, errCount := 0, 0, 0
	lines := make([]string, 0, len(results))
	details := make([][]string, 0, len(results))
	severity := "ok"
	for _, result := range results {
		lines = append(lines, menubarProviderLine(result))
		details = append(details, menubarProviderDetails(result))
		switch classifyMenubarStatus(result.Status) {
		case "ok":
			okCount++
		case "warn":
			warnCount++
			if severity == "ok" {
				severity = "warn"
			}
		default:
			errCount++
			severity = "error"
		}
	}

	return menubarSnapshot{
		Title:           menubarTitleForSeverity(severity),
		Tooltip:         fmt.Sprintf("%d provider(s): %d ok, %d warn, %d error", len(results), okCount, warnCount, errCount),
		SummaryLine:     fmt.Sprintf("%d providers · %d ok · %d warn · %d error", len(results), okCount, warnCount, errCount),
		UpdatedLine:     "Last refresh: " + menubarTimeLabel(now),
		LastRefreshAt:   now,
		ProviderLines:   lines,
		ProviderDetails: details,
	}
}

func buildMenubarErrorSnapshot(toolNames []string, now time.Time, err error) menubarSnapshot {
	lines := make([]string, 0, len(toolNames))
	details := make([][]string, 0, len(toolNames))
	for _, name := range toolNames {
		lines = append(lines, fmt.Sprintf("%s · unavailable", name))
		details = append(details, []string{
			"Provider: " + name,
			"Status: error",
			"Message: unavailable during refresh",
		})
	}
	msg := "unknown error"
	if err != nil {
		msg = err.Error()
	}
	return menubarSnapshot{
		Title:           "oct !!",
		Tooltip:         "menubar refresh failed",
		SummaryLine:     "Refresh failed · " + truncateMenubarText(msg, 48),
		UpdatedLine:     "Last refresh: " + menubarTimeLabel(now),
		LastRefreshAt:   now,
		ProviderLines:   lines,
		ProviderDetails: details,
	}
}

func menubarTitleForSeverity(severity string) string {
	switch severity {
	case "error":
		return "oct !!"
	case "warn":
		return "oct !"
	case "loading":
		return "oct …"
	default:
		return "oct"
	}
}

func menubarProviderLine(result usage.UsageResult) string {
	provider := strings.TrimSpace(result.Provider)
	if provider == "" {
		provider = lookupToolName(result.Provider)
	}
	if provider == "" {
		provider = "Unknown"
	}
	if plan := strings.TrimSpace(result.Plan); plan != "" && !strings.EqualFold(plan, "unknown") {
		provider += " (" + plan + ")"
	}
	five := bucketVal(result, "5h", "used")
	seven := bucketVal(result, "7d", "used")
	status := classifyMenubarStatus(result.Status)
	line := fmt.Sprintf("[%s] %s · 5h %s · 7d %s", status, provider, five, seven)
	if msg := strings.TrimSpace(result.Message); msg != "" && status != "ok" {
		line += " · " + truncateMenubarText(msg, 28)
	}
	return line
}

func menubarProviderDetails(result usage.UsageResult) []string {
	provider := strings.TrimSpace(result.Provider)
	if provider == "" {
		provider = lookupToolName(result.Provider)
	}
	if provider == "" {
		provider = "Unknown"
	}

	details := []string{
		"Provider: " + provider,
		"Status: " + classifyMenubarStatus(result.Status),
		"5h: " + bucketVal(result, "5h", "used"),
		"7d: " + bucketVal(result, "7d", "used"),
	}
	if plan := strings.TrimSpace(result.Plan); plan != "" {
		details = append(details, "Plan: "+plan)
	}
	if source := strings.TrimSpace(result.PlanSource); source != "" {
		details = append(details, "Plan source: "+truncateMenubarText(source, 48))
	}
	if used := strings.TrimSpace(result.Used); used != "" {
		details = append(details, "Used: "+used)
	}
	if limit := strings.TrimSpace(result.Limit); limit != "" {
		details = append(details, "Limit: "+limit)
	}
	if source := strings.TrimSpace(result.Source); source != "" {
		details = append(details, "Source: "+source)
	}
	if detail := strings.TrimSpace(result.SourceDetail); detail != "" {
		details = append(details, "Detail: "+truncateMenubarText(detail, 48))
	}
	if msg := strings.TrimSpace(result.Message); msg != "" {
		details = append(details, "Message: "+truncateMenubarText(msg, 48))
	}
	return details
}

func classifyMenubarStatus(status string) string {
	s := strings.ToLower(strings.TrimSpace(status))
	switch s {
	case "", "ok":
		return "ok"
	case "warn":
		return "warn"
	default:
		return "error"
	}
}

func lookupToolName(provider string) string {
	normalized := update.NormalizeToolName(provider)
	for _, tool := range update.Tools {
		if tool.MatchesName(normalized) {
			return tool.Name
		}
	}
	return strings.TrimSpace(provider)
}

func menubarTimeLabel(now time.Time) string {
	if now.IsZero() {
		return "-"
	}
	return now.Format("15:04:05")
}

func truncateMenubarText(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func menubarRefreshInterval(raw string) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Minute
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return time.Minute
	}
	return d
}

func menubarAutoRefreshLabel(interval time.Duration) string {
	if interval <= 0 {
		interval = time.Minute
	}
	return "Auto refresh: every " + interval.String()
}

func menubarNextRefreshLabel(lastRefresh time.Time, interval time.Duration) string {
	if lastRefresh.IsZero() {
		return "Next refresh: pending"
	}
	if interval <= 0 {
		interval = time.Minute
	}
	return "Next refresh: " + menubarTimeLabel(lastRefresh.Add(interval))
}

func shellQuote(arg string) string {
	if arg == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
}

func buildMenubarExecCommand(execPath string, args ...string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, shellQuote(execPath))
	for _, arg := range args {
		parts = append(parts, shellQuote(arg))
	}
	return strings.Join(parts, " ")
}

func buildTerminalAppleScript(command string) string {
	return "tell application \"Terminal\"\n" +
		"activate\n" +
		"do script " + strconv.Quote(command) + "\n" +
		"end tell"
}
