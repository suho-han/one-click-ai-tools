package usage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func FetchCodexUsage() UsageResult {
	result := UsageResult{
		Provider: "codex",
		Period:   "current",
		Used:     "n/a",
		Limit:    "100",
		Unit:     "percent",
		Source:   "local",
		Status:   "error",
	}

	home, _ := os.UserHomeDir()
	sessionDir := filepath.Join(home, ".codex", "sessions")

	// Scan ~/.codex/sessions/**/*.jsonl
	var logFiles []string
	err := filepath.Walk(sessionDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".jsonl") {
			logFiles = append(logFiles, path)
		}
		return nil
	})

	if err != nil || len(logFiles) == 0 {
		result.Status = "ok"
		result.Used = "0"
		result.Message = "No .jsonl session logs found in ~/.codex/sessions"
		return result
	}

	// Sort to get the latest file by name (which includes timestamp)
	sort.Strings(logFiles)
	latestLog := logFiles[len(logFiles)-1]

	file, err := os.Open(latestLog)
	if err != nil {
		result.Message = fmt.Sprintf("Failed to open latest log: %v", err)
		return result
	}
	defer file.Close()

	var lastUsedPercent string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var line struct {
			Type    string `json:"type"`
			Payload struct {
				Type string `json:"type"`
				Info struct {
					TotalTokenUsage struct {
						InputTokens int `json:"input_tokens"`
					} `json:"total_token_usage"`
				} `json:"info"`
				RateLimits struct {
					Primary struct {
						UsedPercent float64 `json:"used_percent"`
					} `json:"primary"`
				} `json:"rate_limits"`
			} `json:"payload"`
		}

		if err := json.Unmarshal(scanner.Bytes(), &line); err == nil {
			if line.Type == "event_msg" && line.Payload.Type == "token_count" {
				if line.Payload.RateLimits.Primary.UsedPercent > 0 {
					lastUsedPercent = fmt.Sprintf("%.1f", line.Payload.RateLimits.Primary.UsedPercent)
				}
			}
		}
	}

	if lastUsedPercent == "" {
		result.Status = "ok"
		result.Used = "0"
		result.Message = "No usage metrics found in latest session log"
		return result
	}

	result.Status = "ok"
	result.Used = lastUsedPercent
	result.Message = fmt.Sprintf("Extracted from %s", filepath.Base(latestLog))
	return result
}
