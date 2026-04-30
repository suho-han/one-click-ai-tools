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

	if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
		result.Message = "Codex session directory not found"
		return result
	}

	files, err := os.ReadDir(sessionDir)
	if err != nil {
		result.Message = fmt.Sprintf("Failed to read session directory: %v", err)
		return result
	}

	var logFiles []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".jsonl") {
			logFiles = append(logFiles, filepath.Join(sessionDir, f.Name()))
		}
	}

	if len(logFiles) == 0 {
		result.Message = "No .jsonl session logs found"
		return result
	}

	// Sort to get the latest file (assuming naming convention or just take the last modified)
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
		var entry struct {
			UsedPercent float64 `json:"used_percent"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err == nil && entry.UsedPercent > 0 {
			lastUsedPercent = fmt.Sprintf("%.1f", entry.UsedPercent)
		}
	}

	if lastUsedPercent == "" {
		result.Message = "Could not find used_percent in session logs"
		return result
	}

	result.Status = "ok"
	result.Used = lastUsedPercent
	result.Message = fmt.Sprintf("Extracted from %s", filepath.Base(latestLog))
	return result
}
