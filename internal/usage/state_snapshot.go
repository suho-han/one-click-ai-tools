package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type UsageSnapshot struct {
	CapturedAt string        `json:"captured_at"`
	Results    []UsageResult `json:"results"`
}

func DefaultSnapshotPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".oct", "state", "usage-latest.json")
}

func SaveSnapshot(path string, results []UsageResult, now time.Time) error {
	if path == "" {
		path = DefaultSnapshotPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	payload := UsageSnapshot{
		CapturedAt: now.Format(time.RFC3339),
		Results:    results,
	}
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}
