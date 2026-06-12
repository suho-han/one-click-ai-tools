package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/suho-han/one-click-tools/internal/update"
	"github.com/suho-han/one-click-tools/internal/usage"
)

type menubarSnapshot struct {
	Title         string
	Tooltip       string
	SummaryLine   string
	UpdatedLine   string
	ProviderLines []string
}

func buildMenubarLoadingSnapshot(toolNames []string) menubarSnapshot {
	lines := make([]string, 0, len(toolNames))
	for _, name := range toolNames {
		lines = append(lines, fmt.Sprintf("%s · loading…", name))
	}
	return menubarSnapshot{
		Title:         "oct …",
		Tooltip:       "one-click-tools menubar loading",
		SummaryLine:   fmt.Sprintf("Loading usage for %d provider(s)…", len(toolNames)),
		UpdatedLine:   "Last refresh: -",
		ProviderLines: lines,
	}
}

func buildMenubarUsageSnapshot(results []usage.UsageResult, now time.Time) menubarSnapshot {
	okCount, warnCount, errCount := 0, 0, 0
	lines := make([]string, 0, len(results))
	severity := "ok"
	for _, result := range results {
		lines = append(lines, menubarProviderLine(result))
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
		Title:         menubarTitleForSeverity(severity),
		Tooltip:       fmt.Sprintf("%d provider(s): %d ok, %d warn, %d error", len(results), okCount, warnCount, errCount),
		SummaryLine:   fmt.Sprintf("%d providers · %d ok · %d warn · %d error", len(results), okCount, warnCount, errCount),
		UpdatedLine:   "Last refresh: " + menubarTimeLabel(now),
		ProviderLines: lines,
	}
}

func buildMenubarErrorSnapshot(toolNames []string, now time.Time, err error) menubarSnapshot {
	lines := make([]string, 0, len(toolNames))
	for _, name := range toolNames {
		lines = append(lines, fmt.Sprintf("%s · unavailable", name))
	}
	msg := "unknown error"
	if err != nil {
		msg = err.Error()
	}
	return menubarSnapshot{
		Title:         "oct !!",
		Tooltip:       "menubar refresh failed",
		SummaryLine:   "Refresh failed · " + truncateMenubarText(msg, 48),
		UpdatedLine:   "Last refresh: " + menubarTimeLabel(now),
		ProviderLines: lines,
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
	five := bucketVal(result, "5h", "used")
	seven := bucketVal(result, "7d", "used")
	status := classifyMenubarStatus(result.Status)
	line := fmt.Sprintf("%s · 5h %s · 7d %s · %s", provider, five, seven, status)
	if msg := strings.TrimSpace(result.Message); msg != "" && status != "ok" {
		line += " · " + truncateMenubarText(msg, 28)
	}
	return line
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
