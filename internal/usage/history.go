package usage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/suho-han/one-click-ai-tools/internal/fsutil"
)

const (
	historyMaxAge   = 30 * 24 * time.Hour
	historyMaxLines = 20000
	trendWindow     = 6 * time.Hour
)

// HistorySample is one JSONL line: one (provider, window) reading.
type HistorySample struct {
	TS       string  `json:"ts"`
	Provider string  `json:"provider"`
	Window   string  `json:"window"`
	Unit     string  `json:"unit"`
	Used     float64 `json:"used"`
	Limit    float64 `json:"limit"`
}

// HistoryTrend is the per-result forecast exposed as the JSON "history" object.
type HistoryTrend struct {
	BurnRatePerHour float64 `json:"burn_rate_per_hour"`
	EtaAt           string  `json:"eta_at,omitempty"`
	Samples         int     `json:"samples"`
}

func DefaultHistoryPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".oct", "state", "usage-history.jsonl")
}

// HistoryDisabledByEnv reports OCT_USAGE_HISTORY=0 (or false/off/no).
func HistoryDisabledByEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("OCT_USAGE_HISTORY"))) {
	case "0", "false", "off", "no":
		return true
	}
	return false
}

// samplesFromResults extracts recordable percent samples from live results.
func samplesFromResults(results []UsageResult, now time.Time) []HistorySample {
	ts := now.UTC().Format(time.RFC3339)
	var out []HistorySample
	for _, r := range results {
		if !strings.EqualFold(strings.TrimSpace(r.Unit), "percent") {
			continue
		}
		if used, ok := ParsePercent(r.Used); ok {
			limit := 100.0
			if l := strings.TrimSpace(r.Limit); l != "" {
				if v, ok := ParsePercent(l); ok {
					limit = v
				}
			}
			out = append(out, HistorySample{TS: ts, Provider: r.Provider, Window: "", Unit: "percent", Used: used, Limit: limit})
		}
		for w, v := range r.Buckets {
			if used, ok := ParsePercent(v); ok {
				out = append(out, HistorySample{TS: ts, Provider: r.Provider, Window: w, Unit: "percent", Used: used, Limit: 100})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Provider != out[j].Provider {
			return out[i].Provider < out[j].Provider
		}
		return out[i].Window < out[j].Window
	})
	return out
}

// LoadHistory reads the history file, skipping unparseable lines.
func LoadHistory(path string) ([]HistorySample, error) {
	if path == "" {
		path = DefaultHistoryPath()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []HistorySample
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var s HistorySample
		if json.Unmarshal(line, &s) != nil || s.Provider == "" {
			continue
		}
		if _, err := time.Parse(time.RFC3339, s.TS); err != nil {
			continue
		}
		out = append(out, s)
	}
	return out, sc.Err()
}

// AppendHistory records the live results and then enforces retention
// (30 days, 20000 lines). Appends use O_APPEND; truncation swaps a temp file.
func AppendHistory(path string, results []UsageResult, now time.Time) error {
	if path == "" {
		path = DefaultHistoryPath()
	}
	samples := samplesFromResults(results, now)
	if len(samples) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	var buf bytes.Buffer
	for _, s := range samples {
		b, err := json.Marshal(s)
		if err != nil {
			return err
		}
		buf.Write(b)
		buf.WriteByte('\n')
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, werr := f.Write(buf.Bytes())
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return werr
	}
	return pruneHistory(path, now)
}

func pruneHistory(path string, now time.Time) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	cutoff := now.Add(-historyMaxAge)
	var kept [][]byte
	dropped := false
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var s HistorySample
		if json.Unmarshal(line, &s) != nil {
			dropped = true
			continue
		}
		t, err := time.Parse(time.RFC3339, s.TS)
		if err != nil || t.Before(cutoff) {
			dropped = true
			continue
		}
		kept = append(kept, line)
	}
	if len(kept) > historyMaxLines {
		kept = kept[len(kept)-historyMaxLines:]
		dropped = true
	}
	if !dropped {
		return nil
	}
	out := bytes.Join(kept, []byte{'\n'})
	if len(kept) > 0 {
		out = append(out, '\n')
	}
	return fsutil.WriteFileAtomic(path, out, 0600)
}

// ComputeBurn derives the burn rate and exhaustion ETA for one
// (provider, window) from samples. resetAt may be zero. Returns nil when no
// forecast is possible (fewer than 2 samples in the last 6h after discarding
// segments before a usage reset).
func ComputeBurn(samples []HistorySample, provider, window string, now, resetAt time.Time) *HistoryTrend {
	type pt struct {
		t time.Time
		u float64
	}
	var pts []pt
	for _, s := range samples {
		if s.Provider != provider || s.Window != window {
			continue
		}
		t, err := time.Parse(time.RFC3339, s.TS)
		if err != nil || t.Before(now.Add(-trendWindow)) || t.After(now) {
			continue
		}
		pts = append(pts, pt{t, s.Used})
	}
	sort.SliceStable(pts, func(i, j int) bool { return pts[i].t.Before(pts[j].t) })
	// Decreasing usage means a reset/window switch: keep only what follows it.
	start := 0
	for i := 1; i < len(pts); i++ {
		if pts[i].u < pts[i-1].u {
			start = i
		}
	}
	pts = pts[start:]
	if len(pts) < 2 {
		return nil
	}
	first, last := pts[0], pts[len(pts)-1]
	hours := last.t.Sub(first.t).Hours()
	if hours <= 0 {
		return nil
	}
	rate := (last.u - first.u) / hours
	trend := &HistoryTrend{BurnRatePerHour: rate, Samples: len(pts)}
	remaining := 100 - last.u
	if rate <= 0 || remaining <= 0 {
		return trend
	}
	eta := now.Add(time.Duration(remaining / rate * float64(time.Hour)))
	if !resetAt.IsZero() && eta.After(resetAt) {
		return trend // window resets before exhaustion: no ETA
	}
	trend.EtaAt = eta.UTC().Format(time.RFC3339)
	return trend
}

// ComputeTrends returns one trend (or nil) per result, picking the window with
// the earliest ETA, else the highest burn rate.
func ComputeTrends(samples []HistorySample, results []UsageResult, now time.Time) []*HistoryTrend {
	out := make([]*HistoryTrend, len(results))
	for i, r := range results {
		windows := []string{""}
		for w := range r.Buckets {
			windows = append(windows, w)
		}
		sort.Strings(windows)
		var best *HistoryTrend
		for _, w := range windows {
			var resetAt time.Time
			if v, ok := r.BucketResets[w]; ok {
				resetAt, _ = time.Parse(time.RFC3339, v)
			}
			t := ComputeBurn(samples, r.Provider, w, now, resetAt)
			if t == nil {
				continue
			}
			if best == nil || trendBetter(t, best) {
				best = t
			}
		}
		out[i] = best
	}
	return out
}

func trendBetter(a, b *HistoryTrend) bool {
	if (a.EtaAt != "") != (b.EtaAt != "") {
		return a.EtaAt != ""
	}
	if a.EtaAt != "" {
		return a.EtaAt < b.EtaAt
	}
	return a.BurnRatePerHour > b.BurnRatePerHour
}

// LoadTrends reads the default history and computes trends for results.
// Errors yield nil trends: a trend is never worth failing a usage report.
func LoadTrends(path string, results []UsageResult, now time.Time) []*HistoryTrend {
	samples, err := LoadHistory(path)
	if err != nil || len(samples) == 0 {
		return nil
	}
	return ComputeTrends(samples, results, now)
}

func formatETA(t *HistoryTrend, now time.Time) string {
	if t == nil || t.EtaAt == "" {
		return "-"
	}
	eta, err := time.Parse(time.RFC3339, t.EtaAt)
	if err != nil {
		return "-"
	}
	d := eta.Sub(now)
	if d < time.Minute {
		return "<1m"
	}
	return "~" + strconv.FormatFloat(d.Hours(), 'f', 1, 64) + "h (" + eta.Local().Format("15:04") + ")"
}
