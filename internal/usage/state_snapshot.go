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
	return writeFileAtomic(path, b, 0644)
}

// writeFileAtomic writes via a temp file + rename so readers (menubar,
// monitor) never observe a truncated snapshot after a crash mid-write.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
