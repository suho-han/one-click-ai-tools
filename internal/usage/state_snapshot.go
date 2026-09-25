package usage

import (
	"encoding/json"
	"fmt"
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

// LoadSnapshot reads a snapshot previously written by SaveSnapshot (oct
// monitor writes one every cycle). It exists so cheap pollers -- statusbar
// renderers, tray widgets -- can consume the snapshot instead of triggering
// a full provider fan-out.
func LoadSnapshot(path string) (UsageSnapshot, error) {
	if path == "" {
		path = DefaultSnapshotPath()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return UsageSnapshot{}, err
	}
	var snapshot UsageSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return UsageSnapshot{}, fmt.Errorf("parse snapshot %s: %w", path, err)
	}
	return snapshot, nil
}

// writeFileAtomic writes via a temp file + rename so readers (menubar,
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
