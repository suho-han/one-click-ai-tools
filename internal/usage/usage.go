package usage

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

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

func getSelectedTools() []update.Tool {
	order := viper.GetStringSlice("agent_order")
	if len(order) == 0 {
		order = []string{"gemini", "claude", "cursor-agent", "copilot", "opencode", "codex"}
	}
	enabledTools := viper.GetStringSlice("enabled_tools")
	orderedTools := update.GetOrderedTools(order)
	return update.GetFilteredTools(enabledTools, orderedTools)
}

func GetUsage() ([]UsageResult, error) {
	selectedTools := getSelectedTools()

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
	displayMode := strings.ToLower(strings.TrimSpace(viper.GetString("usage_display_mode")))
	if displayMode != "used" && displayMode != "remaining" {
		displayMode = "used"
	}

	fmt.Printf("%-16s %-12s %-8s %-8s %-12s %-12s %-10s %-8s %-8s %s\n",
		"provider", "period", "5h", "1w", "used", "limit", "unit", "source", "status", "message")
	for _, r := range results {
		colorPrefix := ""
		p := strings.ToLower(r.Provider)
		if strings.Contains(p, "gemini") {
			colorPrefix = "\x1b[38;2;66;133;244m" // #4285F4
		} else if strings.Contains(p, "claude") {
			colorPrefix = "\x1b[38;2;217;119;87m" // #D97757
		} else if strings.Contains(p, "codex") || strings.Contains(p, "openai") {
			colorPrefix = "\x1b[38;2;0;166;126m" // #00A67E
		} else if strings.Contains(p, "copilot") || strings.Contains(p, "github") {
			colorPrefix = "\x1b[38;2;188;140;242m" // #BC8CF2
		}

		paddedProvider := fmt.Sprintf("%-16s", r.Provider)
		if colorPrefix != "" {
			paddedProvider = fmt.Sprintf("%s%s\x1b[0m", colorPrefix, paddedProvider)
		}

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

		fmt.Printf("%s %-12s %-8s %-8s %-12s %-12s %-10s %-8s %-8s %s\n",
			paddedProvider, r.Period, fiveHour, oneWeek, displayUsed, r.Limit, r.Unit, r.Source, r.Status, r.Message)
	}
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
	type UsageSummary struct {
		Total int `json:"total"`
		OK    int `json:"ok"`
		Warn  int `json:"warn"`
		Error int `json:"error"`
	}
	payload := struct {
		Summary UsageSummary `json:"summary"`
		Results []UsageResult `json:"results"`
	}{
		Summary: UsageSummary{Total: len(results)},
		Results: results,
	}

	for _, r := range results {
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
