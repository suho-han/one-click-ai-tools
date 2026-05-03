package usage

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/viper"
	"github.com/suho-han/one-click-tools/internal/update"
	"golang.org/x/sync/errgroup"
)

type UsageResult struct {
	Provider     string `json:"provider"`
	Period       string `json:"period"`
	Used         string `json:"used"`
	Limit        string `json:"limit"`
	Unit         string `json:"unit"`
	Source       string `json:"source"`
	Status       string            `json:"status"`
	Message      string            `json:"message"`
	SourceDetail string            `json:"source_detail"`
	Buckets      map[string]string `json:"buckets"` // e.g. {"5h": "10", "7d": "20"}
}

func GetUsage() ([]UsageResult, error) {
	order := viper.GetStringSlice("agent_order")
	if len(order) == 0 {
		order = []string{"gemini", "claude", "copilot", "codex"}
	}

	orderedTools := update.GetOrderedTools(order)
	
	fetchers := map[string]func() UsageResult{
		"gemini":  FetchGeminiUsage,
		"claude":  FetchClaudeUsage,
		"copilot": FetchCopilotUsage,
		"codex":   FetchCodexUsage,
	}

	results := make([]UsageResult, len(orderedTools))
	g := new(errgroup.Group)

	for i, t := range orderedTools {
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
	fmt.Printf("%-16s %-12s %-12s %-12s %-10s %-8s %-8s %s\n",
		"provider", "period", "used", "limit", "unit", "source", "status", "message")
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

		usedDisplay := r.Used
		if len(r.Buckets) > 1 {
			var parts []string
			// Deterministic order: 5h, then others
			if val, ok := r.Buckets["5h"]; ok {
				parts = append(parts, fmt.Sprintf("%s%%(5h)", val))
			}
			if val, ok := r.Buckets["7d"]; ok {
				parts = append(parts, fmt.Sprintf("%s%%(7d)", val))
			}
			// Append any others
			for k, v := range r.Buckets {
				if k != "5h" && k != "7d" {
					parts = append(parts, fmt.Sprintf("%s%%(%s)", v, k))
				}
			}
			usedDisplay = strings.Join(parts, " / ")
		}

		fmt.Printf("%s %-12s %-12s %-12s %-10s %-8s %-8s %s\n",
			paddedProvider, r.Period, usedDisplay, r.Limit, r.Unit, r.Source, r.Status, r.Message)
	}
}

func PrintJSON(results []UsageResult) error {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
