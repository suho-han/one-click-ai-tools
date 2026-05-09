package notify

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/suho-han/one-click-tools/internal/usage"
)

type UsageAlertConfig struct {
	Enabled         bool
	ThresholdPct    float64 // legacy global threshold fallback
	CooldownMinutes int
	StatePath       string

	Timezone          string
	QuietHours        string // HH:MM-HH:MM
	GlobalThresholds  map[string]float64
	ProviderThreshold map[string]map[string]float64 // provider -> window -> threshold
	CriticalPct       float64
}

type alertState struct {
	LastSent      map[string]time.Time `json:"last_sent"`
	LastThreshold map[string]float64   `json:"last_threshold,omitempty"`
	SnoozedUntil  map[string]time.Time `json:"snoozed_until,omitempty"`
}

type alertHit struct {
	Window    string
	Value     float64
	Threshold float64
}

type alertPriority string

const (
	alertPriorityNormal   alertPriority = "normal"
	alertPriorityHigh     alertPriority = "high"
	alertPriorityCritical alertPriority = "critical"
)

var notifyFn = sendOSNotification

func MaybeSendUsageAlerts(results []usage.UsageResult, cfg UsageAlertConfig, now time.Time) error {
	if !cfg.Enabled {
		return nil
	}
	cfg = normalizeConfig(cfg)

	loc := time.Local
	if strings.TrimSpace(cfg.Timezone) != "" {
		if l, err := time.LoadLocation(cfg.Timezone); err == nil {
			loc = l
		}
	}
	localNow := now.In(loc)

	st, _ := loadState(cfg.StatePath)
	if st.LastSent == nil {
		st.LastSent = map[string]time.Time{}
	}
	if st.LastThreshold == nil {
		st.LastThreshold = map[string]float64{}
	}
	if st.SnoozedUntil == nil {
		st.SnoozedUntil = map[string]time.Time{}
	}
	stateChanged := cleanupExpiredSnooze(&st, now)

	cooldown := time.Duration(cfg.CooldownMinutes) * time.Minute
	var sentAny bool
	for _, r := range results {
		hits := overThresholdKeys(r, cfg)
		for _, h := range hits {
			priority := computeAlertPriority(h.Value, h.Threshold, cfg.CriticalPct)
			if isSnoozed(st, r.Provider, h.Window, now) && priority != alertPriorityCritical {
				continue
			}
			key := strings.ToLower(r.Provider) + ":" + h.Window
			lastSent, hasSent := st.LastSent[key]
			lastThreshold := st.LastThreshold[key]

			if hasSent && now.Sub(lastSent) < cooldown {
				// Allow immediate notification if higher threshold crossed.
				if h.Threshold <= lastThreshold {
					continue
				}
			}

			if inQuietHours(localNow, cfg.QuietHours) && priority != alertPriorityCritical {
				continue
			}

			msg := fmt.Sprintf("[%s] %s %s usage %.1f%% (threshold %.1f%%)", strings.ToUpper(string(priority)), r.Provider, h.Window, h.Value, h.Threshold)
			if err := notifyFn("oct usage alert", msg); err == nil {
				st.LastSent[key] = now
				st.LastThreshold[key] = h.Threshold
				sentAny = true
			}
		}
	}
	if sentAny || stateChanged {
		return saveState(cfg.StatePath, st)
	}
	return nil
}

func normalizeConfig(cfg UsageAlertConfig) UsageAlertConfig {
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
	if cfg.GlobalThresholds == nil {
		cfg.GlobalThresholds = map[string]float64{}
	}
	if _, ok := cfg.GlobalThresholds["default"]; !ok {
		cfg.GlobalThresholds["default"] = cfg.ThresholdPct
	}
	if cfg.ProviderThreshold == nil {
		cfg.ProviderThreshold = map[string]map[string]float64{}
	}
	if cfg.CriticalPct <= 0 {
		cfg.CriticalPct = 98
	}
	return cfg
}

func overThresholdKeys(r usage.UsageResult, cfg UsageAlertConfig) []alertHit {
	var hits []alertHit
	if strings.ToLower(strings.TrimSpace(r.Unit)) != "percent" {
		return hits
	}

	collect := func(window, raw string) {
		v, ok := parsePercent(raw)
		if !ok {
			return
		}
		thr := thresholdFor(cfg, r.Provider, window)
		if v >= thr {
			hits = append(hits, alertHit{Window: window, Value: v, Threshold: thr})
		}
	}

	collect("current", r.Used)
	keys := make([]string, 0, len(r.Buckets))
	for k := range r.Buckets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		collect(k, r.Buckets[k])
	}
	return hits
}

func thresholdFor(cfg UsageAlertConfig, provider, window string) float64 {
	p := strings.ToLower(strings.TrimSpace(provider))
	w := strings.ToLower(strings.TrimSpace(window))

	if pm, ok := cfg.ProviderThreshold[p]; ok {
		if v, ok := pm[w]; ok && v > 0 {
			return v
		}
		if v, ok := pm["default"]; ok && v > 0 {
			return v
		}
	}
	if v, ok := cfg.GlobalThresholds[w]; ok && v > 0 {
		return v
	}
	if v, ok := cfg.GlobalThresholds["default"]; ok && v > 0 {
		return v
	}
	return cfg.ThresholdPct
}

func snoozeKey(provider, window string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	window = strings.ToLower(strings.TrimSpace(window))
	if provider == "" && window == "" {
		return "global"
	}
	if provider != "" && window == "" {
		return "provider:" + provider
	}
	if provider == "" {
		return "window:" + window
	}
	return "provider:" + provider + ":window:" + window
}

func isSnoozed(st alertState, provider, window string, now time.Time) bool {
	keys := []string{snoozeKey(provider, window), snoozeKey(provider, ""), snoozeKey("", window), snoozeKey("", "")}
	for _, k := range keys {
		if until, ok := st.SnoozedUntil[k]; ok && now.Before(until) {
			return true
		}
	}
	return false
}

func cleanupExpiredSnooze(st *alertState, now time.Time) bool {
	if st == nil || st.SnoozedUntil == nil {
		return false
	}
	changed := false
	for k, until := range st.SnoozedUntil {
		if !now.Before(until) {
			delete(st.SnoozedUntil, k)
			changed = true
		}
	}
	return changed
}

func SetSnooze(path, provider, window string, until time.Time) error {
	st, _ := loadState(path)
	if st.SnoozedUntil == nil {
		st.SnoozedUntil = map[string]time.Time{}
	}
	st.SnoozedUntil[snoozeKey(provider, window)] = until
	return saveState(path, st)
}

func ClearSnooze(path, provider, window string) error {
	st, _ := loadState(path)
	if st.SnoozedUntil == nil {
		return nil
	}
	delete(st.SnoozedUntil, snoozeKey(provider, window))
	return saveState(path, st)
}

func GetSnooze(path string) (map[string]time.Time, error) {
	st, err := loadState(path)
	if err != nil {
		return map[string]time.Time{}, nil
	}
	if st.SnoozedUntil == nil {
		return map[string]time.Time{}, nil
	}
	return st.SnoozedUntil, nil
}

func inQuietHours(now time.Time, quiet string) bool {
	quiet = strings.TrimSpace(quiet)
	if quiet == "" {
		return false
	}
	parts := strings.Split(quiet, "-")
	if len(parts) != 2 {
		return false
	}
	start, ok1 := parseHHMM(parts[0])
	end, ok2 := parseHHMM(parts[1])
	if !ok1 || !ok2 {
		return false
	}
	cur := now.Hour()*60 + now.Minute()
	if start == end {
		return true
	}
	if start < end {
		return cur >= start && cur < end
	}
	return cur >= start || cur < end
}

func parseHHMM(s string) (int, bool) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, false
	}
	h, errH := strconv.Atoi(parts[0])
	m, errM := strconv.Atoi(parts[1])
	if errH != nil || errM != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

func parsePercent(s string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(s, "%")), 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func computeAlertPriority(value, threshold, criticalPct float64) alertPriority {
	if value >= criticalPct {
		return alertPriorityCritical
	}
	if value >= threshold {
		return alertPriorityHigh
	}
	return alertPriorityNormal
}

func loadState(path string) (alertState, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return alertState{LastSent: map[string]time.Time{}, LastThreshold: map[string]float64{}, SnoozedUntil: map[string]time.Time{}}, err
	}
	var st alertState
	if err := json.Unmarshal(b, &st); err != nil {
		return alertState{LastSent: map[string]time.Time{}, LastThreshold: map[string]float64{}, SnoozedUntil: map[string]time.Time{}}, err
	}
	if st.LastSent == nil {
		st.LastSent = map[string]time.Time{}
	}
	if st.LastThreshold == nil {
		st.LastThreshold = map[string]float64{}
	}
	if st.SnoozedUntil == nil {
		st.SnoozedUntil = map[string]time.Time{}
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
