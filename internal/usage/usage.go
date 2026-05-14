package usage

import (
	"encoding/json"
	"fmt"
	"log"
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

	for _, fetch := range []func() UsageResult{
		FetchGeminiUsage,
		FetchClaudeUsage,
		FetchCopilotUsage,
		FetchCodexUsage,
	} {
		r := fetch()
		if r.Status == "error" {
			log.Printf("usage error [%s]: %s", r.Provider, r.Message)
		}
		results = append(results, r)
	}

	return results, nil
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
		fmt.Printf("%s %-12s %-12s %-12s %-10s %-8s %-8s %s\n",
			paddedProvider, r.Period, r.Used, r.Limit, r.Unit, r.Source, r.Status, r.Message)
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
