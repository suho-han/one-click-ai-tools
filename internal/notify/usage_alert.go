package notify

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/suho-han/one-click-tools/internal/usage"
)

type UsageAlertConfig struct {
	Enabled         bool
	ThresholdPct    float64
	CooldownMinutes int
	StatePath       string
}

type alertState struct {
	LastSent map[string]time.Time `json:"last_sent"`
}

func MaybeSendUsageAlerts(results []usage.UsageResult, cfg UsageAlertConfig, now time.Time) error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.ThresholdPct <= 0 {
		cfg.ThresholdPct = 80
	}
	if cfg.CooldownMinutes <= 0 {
		cfg.CooldownMinutes = 360
	}
	if cfg.StatePath == "" {
		home, _ := os.UserHomeDir()
		cfg.StatePath = filepath.Join(home, ".oct", "state", "usage-alert-state.json")
	}

	st, _ := loadState(cfg.StatePath)
	if st.LastSent == nil {
		st.LastSent = map[string]time.Time{}
	}

	cooldown := time.Duration(cfg.CooldownMinutes) * time.Minute
	var sentAny bool
	for _, r := range results {
		alerts := overThresholdKeys(r, cfg.ThresholdPct)
		for _, a := range alerts {
			key := r.Provider + ":" + a.Window
			if ts, ok := st.LastSent[key]; ok && now.Sub(ts) < cooldown {
				continue
			}
			msg := fmt.Sprintf("%s %s usage %.1f%% (threshold %.1f%%)", r.Provider, a.Window, a.Value, cfg.ThresholdPct)
			if err := sendOSNotification("oct usage alert", msg); err == nil {
				st.LastSent[key] = now
				sentAny = true
			}
		}
	}
	if sentAny {
		return saveState(cfg.StatePath, st)
	}
	return nil
}

type alertHit struct {
	Window string
	Value  float64
}

func overThresholdKeys(r usage.UsageResult, threshold float64) []alertHit {
	var hits []alertHit
	if strings.ToLower(strings.TrimSpace(r.Unit)) != "percent" {
		return hits
	}
	if v, ok := parsePercent(r.Used); ok && v >= threshold {
		hits = append(hits, alertHit{Window: "current", Value: v})
	}
	for k, raw := range r.Buckets {
		if v, ok := parsePercent(raw); ok && v >= threshold {
			hits = append(hits, alertHit{Window: k, Value: v})
		}
	}
	return hits
}

func parsePercent(s string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(s, "%")), 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func loadState(path string) (alertState, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return alertState{LastSent: map[string]time.Time{}}, err
	}
	var st alertState
	if err := json.Unmarshal(b, &st); err != nil {
		return alertState{LastSent: map[string]time.Time{}}, err
	}
	if st.LastSent == nil {
		st.LastSent = map[string]time.Time{}
	}
	return st, nil
}

func saveState(path string, st alertState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

func sendOSNotification(title, message string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("osascript", "-e", fmt.Sprintf("display notification %q with title %q", message, title)).Run()
	case "linux":
		return exec.Command("notify-send", title, message).Run()
	case "windows":
		ps := fmt.Sprintf("[System.Reflection.Assembly]::LoadWithPartialName('System.Windows.Forms') | Out-Null; [System.Windows.Forms.MessageBox]::Show(%q,%q)", message, title)
		return exec.Command("powershell", "-NoProfile", "-Command", ps).Run()
	default:
		return fmt.Errorf("unsupported OS for notification")
	}
}
