package usage

import (
	"encoding/json"
	"fmt"
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
	fmt.Printf("%-12s %-12s %-12s %-12s %-10s %-8s %-8s %s\n",
		"provider", "period", "used", "limit", "unit", "source", "status", "message")
	for _, r := range results {
		fmt.Printf("%-12s %-12s %-12s %-12s %-10s %-8s %-8s %s\n",
			r.Provider, r.Period, r.Used, r.Limit, r.Unit, r.Source, r.Status, r.Message)
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
