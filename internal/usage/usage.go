package usage

import (
	"encoding/json"
	"fmt"
	"strings"
)

type UsageResult struct {
	Provider     string `json:"provider"`
	Period       string `json:"period"`
	Used         string `json:"used"`
	Limit        string `json:"limit"`
	Unit         string `json:"unit"`
	Source       string `json:"source"`
	Status       string `json:"status"`
	Message      string `json:"message"`
	SourceDetail string `json:"source_detail"`
}

func GetUsage() ([]UsageResult, error) {
	var results []UsageResult
	
	results = append(results, FetchGeminiUsage())
	results = append(results, FetchClaudeUsage())
	results = append(results, FetchCopilotUsage())
	results = append(results, FetchCodexUsage())
	
	return results, nil
}

func PrintTable(results []UsageResult) {
	fmt.Printf("%-16s %-12s %-12s %-12s %-10s %-8s %-8s %s\n",
		"provider", "period", "used", "limit", "unit", "source", "status", "message")
	for _, r := range results {
		icon := ""
		p := strings.ToLower(r.Provider)
		if strings.Contains(p, "gemini") {
			icon = "✨"
		} else if strings.Contains(p, "claude") {
			icon = "🤖"
		} else if strings.Contains(p, "codex") || strings.Contains(p, "openai") {
			icon = "⚛️"
		} else if strings.Contains(p, "copilot") || strings.Contains(p, "github") {
			icon = "🐙"
		}

		provider := fmt.Sprintf("%s %s", icon, r.Provider)
		if icon == "" {
			provider = r.Provider
		}
		fmt.Printf("%-16s %-12s %-12s %-12s %-10s %-8s %-8s %s\n",
			provider, r.Period, r.Used, r.Limit, r.Unit, r.Source, r.Status, r.Message)
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
