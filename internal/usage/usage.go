package usage

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/spf13/viper"
	"github.com/suho-han/one-click-tools/internal/update"
	"golang.org/x/sync/errgroup"
)

type UsageResult struct {
	Provider     string            `json:"provider"`
	Period       string            `json:"period"`
	Used         string            `json:"used"`
	Limit        string            `json:"limit"`
	Unit         string            `json:"unit"`
	Source       string            `json:"source"`
	Status       string            `json:"status"`
	Message      string            `json:"message"`
	SourceDetail string            `json:"source_detail"`
	Buckets      map[string]string `json:"buckets"` // e.g. {"5h": "10", "7d": "20"}
}

func SelectedTools() []update.Tool {
	order := viper.GetStringSlice("agent_order")
	if len(order) == 0 {
		order = []string{"gemini", "claude", "cursor-agent", "copilot", "opencode", "codex"}
	}
	enabledTools := viper.GetStringSlice("enabled_tools")
	orderedTools := update.GetOrderedTools(order)
	return update.GetFilteredTools(enabledTools, orderedTools)
}

func GetUsage() ([]UsageResult, error) {
	selectedTools := SelectedTools()

	fetchers := map[string]func() UsageResult{
		"gemini":       FetchGeminiUsage,
		"claude":       FetchClaudeUsage,
		"cursor-agent": FetchCursorUsage,
		"copilot":      FetchCopilotUsage,
		"opencode":     FetchOpenCodeUsage,
		"codex":        FetchCodexUsage,
	}

	results := make([]UsageResult, len(selectedTools))
	g := new(errgroup.Group)

	for i, t := range selectedTools {
		i, t := i, t // Capture for goroutine
		if fetcher, ok := fetchers[strings.ToLower(t.BinaryName)]; ok {
			g.Go(func() error {
				results[i] = fetcher()
				return nil
			})
		}
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Filter out empty results if any tools were skipped
	var filtered []UsageResult
	for _, r := range results {
		if r.Provider != "" {
			filtered = append(filtered, r)
		}
	}

	return filtered, nil
}

func PrintTable(results []UsageResult) {
	width := terminalWidth()
	messageWidth := tableMessageWidth(width)

	displayMode := strings.ToLower(strings.TrimSpace(viper.GetString("usage_display_mode")))
	if displayMode != "used" && displayMode != "remaining" {
		displayMode = "used"
	}

	fmt.Printf("%-16s %-12s %-8s %-8s %-12s %-12s %-10s %-8s %-8s %s\n",
		"provider", "period", "5h", "1w", "used", "limit", "unit", "source", "status", "message")
	for _, r := range results {
		providerLabel := providerDisplayLabel(r.Provider)
		paddedProvider := colorizeProvider(padRightRunes(providerLabel, 16), r.Provider)

		fiveHour := "-"
		oneWeek := "-"
		if val, ok := r.Buckets["5h"]; ok && val != "" {
			fiveHour = formatBucketDisplay(r, val, displayMode)
		}
		if val, ok := r.Buckets["7d"]; ok && val != "" {
			oneWeek = formatBucketDisplay(r, val, displayMode)
		}
		// Fallback for providers that currently expose a single percent usage value.
		if fiveHour == "-" && r.Unit == "percent" && r.Used != "" && r.Used != "n/a" {
			provider := strings.ToLower(r.Provider)
			if strings.Contains(provider, "gemini") || strings.Contains(provider, "claude") {
				fiveHour = formatBucketDisplay(r, r.Used, displayMode)
			}
		}

		displayUsed := r.Used
		if displayMode == "remaining" && strings.EqualFold(r.Unit, "percent") {
			if rem, ok := remainingFromUsed(r.Used); ok {
				displayUsed = rem
			}
		}

		statusLabel := colorizeStatus(fmt.Sprintf("%-8s", r.Status), r.Status)
		message := colorizeMessage(truncateText(r.Message, messageWidth), r.Status)

		fmt.Printf("%s %-12s %-8s %-8s %-12s %-12s %-10s %-8s %s %s\n",
			paddedProvider, r.Period, fiveHour, oneWeek, displayUsed, r.Limit, r.Unit, r.Source, statusLabel, message)
	}
}

func colorizeProvider(label string, provider string) string {
	p := strings.ToLower(provider)
	code := ""
	switch {
	case strings.Contains(p, "gemini"):
		code = "94"
	case strings.Contains(p, "claude"):
		code = "93"
	case strings.Contains(p, "codex"), strings.Contains(p, "openai"):
		code = "96"
	case strings.Contains(p, "copilot"), strings.Contains(p, "github"):
		code = "95"
	case strings.Contains(p, "cursor"):
		code = "94"
	case strings.Contains(p, "opencode"):
		code = "97"
	}
	if code == "" {
		return label
	}
	return "\x1b[1;" + code + "m" + label + "\x1b[0m"
}

func colorizeStatus(label string, status string) string {
	s := strings.ToLower(strings.TrimSpace(status))
	switch s {
	case "ok":
		return "\x1b[1;92m" + label + "\x1b[0m"
	case "warn":
		return "\x1b[1;93m" + label + "\x1b[0m"
	default:
		return "\x1b[1;91m" + label + "\x1b[0m"
	}
}

func colorizeMessage(message string, status string) string {
	s := strings.ToLower(strings.TrimSpace(status))
	switch s {
	case "warn":
		return "\x1b[93m" + message + "\x1b[0m"
	case "error":
		return "\x1b[91m" + message + "\x1b[0m"
	default:
		return message
	}
}

func providerDisplayLabel(provider string) string {
	if !supportsProviderIcons() {
		return provider
	}
	p := strings.ToLower(strings.TrimSpace(provider))
	switch {
	case strings.Contains(p, "cursor"):
		return "▣ " + provider
	case strings.Contains(p, "opencode"):
		return "🧩 " + provider
	default:
		return provider
	}
}

func supportsProviderIcons() bool {
	if isTruthy(os.Getenv("OCT_NO_ICONS")) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("TERM")), "dumb") {
		return false
	}
	if os.Getenv("WT_SESSION") != "" {
		return true
	}
	locale := strings.ToUpper(strings.TrimSpace(os.Getenv("LC_ALL")))
	if locale == "" {
		locale = strings.ToUpper(strings.TrimSpace(os.Getenv("LC_CTYPE")))
	}
	if locale == "" {
		locale = strings.ToUpper(strings.TrimSpace(os.Getenv("LANG")))
	}
	return strings.Contains(locale, "UTF-8") || strings.Contains(locale, "UTF8")
}

func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func padRightRunes(s string, width int) string {
	if utf8.RuneCountInString(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-utf8.RuneCountInString(s))
}

func terminalWidth() int {
	if raw := strings.TrimSpace(os.Getenv("COLUMNS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 40 {
			return n
		}
	}
	return 100
}

func tableMessageWidth(width int) int {
	switch {
	case width <= 80:
		return 10
	case width <= 100:
		return 20
	case width <= 120:
		return 28
	default:
		return 40
	}
}

func truncateText(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func formatBucketDisplay(r UsageResult, rawValue, mode string) string {
	value := rawValue
	if mode == "remaining" && strings.EqualFold(r.Unit, "percent") {
		if rem, ok := remainingFromUsed(rawValue); ok {
			value = rem
		}
	}

	// Gemini usage is shown as count-style number in table output.
	if strings.Contains(strings.ToLower(r.Provider), "gemini") {
		return value
	}
	if strings.EqualFold(r.Unit, "percent") {
		return value + "%"
	}
	return value
}

func remainingFromUsed(used string) (string, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(used), 64)
	if err != nil {
		return "", false
	}
	remaining := 100 - v
	if remaining < 0 {
		remaining = 0
	}
	return fmt.Sprintf("%.1f", remaining), true
}

func PrintJSON(results []UsageResult) error {
	type UsageResultCompact struct {
		Provider string            `json:"provider"`
		Status   string            `json:"status"`
		Used     string            `json:"used"`
		Unit     string            `json:"unit"`
		Buckets  map[string]string `json:"buckets,omitempty"`
		Message  string            `json:"message,omitempty"`
	}
	type UsageSummary struct {
		Total int `json:"total"`
		OK    int `json:"ok"`
		Warn  int `json:"warn"`
		Error int `json:"error"`
	}
	payload := struct {
		Summary UsageSummary       `json:"summary"`
		Results []UsageResultCompact `json:"results"`
	}{
		Summary: UsageSummary{Total: len(results)},
		Results: make([]UsageResultCompact, 0, len(results)),
	}

	for _, r := range results {
		payload.Results = append(payload.Results, UsageResultCompact{
			Provider: r.Provider,
			Status:   r.Status,
			Used:     r.Used,
			Unit:     r.Unit,
			Buckets:  r.Buckets,
			Message:  truncateText(r.Message, 48),
		})

		switch classifySummaryStatus(r) {
		case "ok":
			payload.Summary.OK++
		case "warn":
			payload.Summary.Warn++
		default:
			payload.Summary.Error++
		}
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func classifySummaryStatus(r UsageResult) string {
	status := strings.ToLower(strings.TrimSpace(r.Status))
	if status != "ok" {
		if status == "warn" {
			return "warn"
		}
		return "error"
	}

	used := strings.ToLower(strings.TrimSpace(r.Used))
	if used == "" || used == "n/a" {
		return "warn"
	}

	// Many providers report used=0 when local/API usage data is unavailable.
	// Keep strict "ok" only when 0 is a real measured value, not a "not found" signal.
	msg := strings.ToLower(strings.TrimSpace(r.Message))
	if used == "0" {
		if strings.HasPrefix(msg, "no ") || strings.Contains(msg, "not found") || strings.Contains(msg, "no configured") {
			return "warn"
		}
	}

	return "ok"
}
