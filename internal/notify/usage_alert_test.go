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
	cfg := normalizeConfig(UsageAlertConfig{
		Enabled:          true,
		ThresholdPct:     80,
		GlobalThresholds: map[string]float64{"default": 80, "7d": 70},
	})
	hits := overThresholdKeys(r, cfg)
	if len(hits) != 3 {
		t.Fatalf("expected 3 hits, got %d", len(hits))
	}
}

func TestThresholdPriority(t *testing.T) {
	cfg := normalizeConfig(UsageAlertConfig{
		ThresholdPct:     80,
		GlobalThresholds: map[string]float64{"default": 80, "5h": 85},
		ProviderThreshold: map[string]map[string]float64{
			"codex": {"default": 88, "5h": 92},
		},
	})
	if v := thresholdFor(cfg, "codex", "5h"); v != 92 {
		t.Fatalf("expected provider window threshold 92, got %.1f", v)
	}
	if v := thresholdFor(cfg, "codex", "7d"); v != 88 {
		t.Fatalf("expected provider default threshold 88, got %.1f", v)
	}
	if v := thresholdFor(cfg, "claude", "5h"); v != 85 {
		t.Fatalf("expected global window threshold 85, got %.1f", v)
	}
}

func TestQuietHours(t *testing.T) {
	loc := time.FixedZone("KST", 9*3600)
	inside := time.Date(2026, 5, 9, 1, 30, 0, 0, loc)
	outside := time.Date(2026, 5, 9, 9, 0, 0, 0, loc)
	if !inQuietHours(inside, "00:00-08:00") {
		t.Fatalf("expected inside quiet hours")
	}
	if inQuietHours(outside, "00:00-08:00") {
		t.Fatalf("expected outside quiet hours")
	}
}

func TestCooldownStatePersistence(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	st := alertState{LastSent: map[string]time.Time{"claude:current": time.Now()}, LastThreshold: map[string]float64{"claude:current": 80}, SnoozedUntil: map[string]time.Time{"global": time.Now().Add(1 * time.Hour)}}
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
	if loaded.LastThreshold["claude:current"] != 80 {
		t.Fatalf("expected threshold in state")
	}
}

func TestMaybeSendUsageAlertsWithQuietHoursAndEscalation(t *testing.T) {
	origNotify := notifyFn
	defer func() { notifyFn = origNotify }()
	notifyCount := 0
	notifyFn = func(title, message string) error {
		notifyCount++
		return nil
	}

	statePath := filepath.Join(t.TempDir(), "state.json")
	cfg := UsageAlertConfig{
		Enabled:         true,
		ThresholdPct:    80,
		CooldownMinutes: 120,
		StatePath:       statePath,
		QuietHours:      "00:00-08:00",
		Timezone:        "Asia/Seoul",
		ProviderThreshold: map[string]map[string]float64{
			"codex": {"5h": 85, "default": 80},
		},
	}

	results := []usage.UsageResult{{Provider: "codex", Unit: "percent", Used: "86", Buckets: map[string]string{"5h": "86"}}}
	// quiet hour -> suppress (<95)
	nowQuiet := time.Date(2026, 5, 9, 1, 0, 0, 0, time.FixedZone("KST", 9*3600)).UTC()
	if err := MaybeSendUsageAlerts(results, cfg, nowQuiet); err != nil {
		t.Fatalf("MaybeSendUsageAlerts failed: %v", err)
	}
	if notifyCount != 0 {
		t.Fatalf("expected no notifications during quiet hours")
	}

	// non-quiet -> send once
	now := time.Date(2026, 5, 9, 10, 0, 0, 0, time.FixedZone("KST", 9*3600)).UTC()
	if err := MaybeSendUsageAlerts(results, cfg, now); err != nil {
		t.Fatalf("MaybeSendUsageAlerts failed: %v", err)
	}
	if notifyCount == 0 {
		t.Fatalf("expected notification after quiet hours")
	}
	baseCount := notifyCount

	// same threshold within cooldown -> no send
	if err := MaybeSendUsageAlerts(results, cfg, now.Add(10*time.Minute)); err != nil {
		t.Fatalf("MaybeSendUsageAlerts failed: %v", err)
	}
	if notifyCount != baseCount {
		t.Fatalf("expected no duplicate notification in cooldown")
	}

	// escalated threshold (95) within cooldown -> send
	resultsEsc := []usage.UsageResult{{Provider: "codex", Unit: "percent", Used: "96", Buckets: map[string]string{"5h": "96"}}}
	cfg.ProviderThreshold["codex"]["5h"] = 95
	if err := MaybeSendUsageAlerts(resultsEsc, cfg, now.Add(20*time.Minute)); err != nil {
		t.Fatalf("MaybeSendUsageAlerts failed: %v", err)
	}
	if notifyCount != baseCount+1 {
		t.Fatalf("expected escalation notification within cooldown")
	}
}

func TestSnoozeSuppressionAndCriticalOverride(t *testing.T) {
	origNotify := notifyFn
	defer func() { notifyFn = origNotify }()
	notifyCount := 0
	notifyFn = func(title, message string) error {
		notifyCount++
		return nil
	}

	statePath := filepath.Join(t.TempDir(), "state.json")
	cfg := UsageAlertConfig{
		Enabled:         true,
		ThresholdPct:    80,
		CooldownMinutes: 120,
		StatePath:       statePath,
		CriticalPct:     98,
	}
	now := time.Now()
	if err := SetSnooze(statePath, "", "", now.Add(1*time.Hour)); err != nil {
		t.Fatalf("SetSnooze failed: %v", err)
	}

	belowCrit := []usage.UsageResult{{Provider: "codex", Unit: "percent", Used: "96", Buckets: map[string]string{"5h": "96"}}}
	if err := MaybeSendUsageAlerts(belowCrit, cfg, now); err != nil {
		t.Fatalf("MaybeSendUsageAlerts failed: %v", err)
	}
	if notifyCount != 0 {
		t.Fatalf("expected snooze suppression below critical")
	}

	critical := []usage.UsageResult{{Provider: "codex", Unit: "percent", Used: "99", Buckets: map[string]string{"5h": "99"}}}
	if err := MaybeSendUsageAlerts(critical, cfg, now.Add(1*time.Minute)); err != nil {
		t.Fatalf("MaybeSendUsageAlerts failed: %v", err)
	}
	if notifyCount == 0 {
		t.Fatalf("expected critical override notification")
	}
}
