package notify

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/suho-han/one-click-tools/internal/usage"
)

func TestOverThresholdKeys(t *testing.T) {
	r := usage.UsageResult{
		Provider: "claude-code",
		Unit:     "percent",
		Used:     "82.5",
		Buckets: map[string]string{
			"5h": "90",
			"7d": "75",
		},
	}
	hits := overThresholdKeys(r, 80)
	if len(hits) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(hits))
	}
}

func TestCooldownStatePersistence(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	cfg := UsageAlertConfig{Enabled: true, ThresholdPct: 80, CooldownMinutes: 60, StatePath: statePath}

	// no notify binary in CI is okay: function only persists when send succeeds.
	// So here we only validate state load/save helpers deterministically.
	st := alertState{LastSent: map[string]time.Time{"claude:current": time.Now()}}
	if err := saveState(statePath, st); err != nil {
		t.Fatalf("saveState failed: %v", err)
	}
	loaded, err := loadState(statePath)
	if err != nil {
		t.Fatalf("loadState failed: %v", err)
	}
	if len(loaded.LastSent) != 1 {
		t.Fatalf("expected one key in state, got %d", len(loaded.LastSent))
	}

	_ = cfg
}
