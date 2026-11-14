package usage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)

func sample(ts time.Time, used float64) HistorySample {
	return HistorySample{TS: ts.Format(time.RFC3339), Provider: "codex", Window: "5h", Unit: "percent", Used: used, Limit: 100}
}

func TestAppendHistoryRoundTripAndPerm(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "usage-history.jsonl")
	results := []UsageResult{
		{Provider: "codex", Unit: "percent", Used: "42", Buckets: map[string]string{"5h": "10", "7d": "20"}},
		{Provider: "cursor", Unit: "requests", Used: "5", Buckets: map[string]string{"5h": "1"}},
		{Provider: "bad", Unit: "percent", Used: "n/a"},
	}
	if err := AppendHistory(path, results, t0); err != nil {
		t.Fatal(err)
	}
	got, err := LoadHistory(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 { // codex "", 5h, 7d
		t.Fatalf("samples = %+v, want 3", got)
	}
	if got[0].Window != "" || got[0].Used != 42 || got[1].Window != "5h" || got[2].Window != "7d" {
		t.Fatalf("unexpected samples %+v", got)
	}
	fi, _ := os.Stat(path)
	if fi.Mode().Perm() != 0600 {
		t.Fatalf("perm = %v", fi.Mode().Perm())
	}
	di, _ := os.Stat(filepath.Dir(path))
	if di.Mode().Perm() != 0700 {
		t.Fatalf("dir perm = %v", di.Mode().Perm())
	}
}

func TestHistoryRetention(t *testing.T) {
	tests := []struct {
		name  string
		build func() string
		want  int
	}{
		{"older than 30 days dropped", func() string {
			return sampleLine(t0.Add(-31*24*time.Hour)) + sampleLine(t0.Add(-29*24*time.Hour))
		}, 2}, // old dropped, 29d kept, plus the new append
		{"line cap keeps newest", func() string {
			var b strings.Builder
			for i := 0; i < historyMaxLines+50; i++ {
				b.WriteString(sampleLine(t0.Add(-time.Hour)))
			}
			return b.String()
		}, historyMaxLines},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "h.jsonl")
			if err := os.WriteFile(path, []byte(tc.build()), 0600); err != nil {
				t.Fatal(err)
			}
			if err := AppendHistory(path, []UsageResult{{Provider: "codex", Unit: "percent", Used: "1"}}, t0); err != nil {
				t.Fatal(err)
			}
			got, _ := LoadHistory(path)
			if len(got) != tc.want {
				t.Fatalf("len = %d, want %d", len(got), tc.want)
			}
			if last := got[len(got)-1]; last.Used != 1 {
				t.Fatalf("newest sample lost: %+v", last)
			}
		})
	}
}

func sampleLine(ts time.Time) string {
	return fmt.Sprintf(`{"ts":%q,"provider":"codex","window":"5h","unit":"percent","used":5,"limit":100}`+"\n", ts.Format(time.RFC3339))
}

func TestLoadHistorySkipsCorruptLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "h.jsonl")
	data := sampleLine(t0) + "{not json\n" + sampleLine(t0.Add(time.Minute))
	os.WriteFile(path, []byte(data), 0600)
	got, err := LoadHistory(path)
	if err != nil || len(got) != 2 {
		t.Fatalf("got %d samples, err %v; want 2", len(got), err)
	}
	// Pruning also survives and removes the corrupt line.
	if err := AppendHistory(path, []UsageResult{{Provider: "codex", Unit: "percent", Used: "1"}}, t0.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	got, _ = LoadHistory(path)
	if len(got) != 3 {
		t.Fatalf("after append len = %d, want 3", len(got))
	}
}

func TestComputeBurn(t *testing.T) {
	now := t0
	tests := []struct {
		name     string
		samples  []HistorySample
		resetAt  time.Time
		wantNil  bool
		wantRate float64
		wantETA  string
	}{
		{"linear", []HistorySample{sample(now.Add(-2*time.Hour), 10), sample(now, 30)}, time.Time{}, false, 10, now.Add(7 * time.Hour).Format(time.RFC3339)},
		{"zero usage rate 0 no eta", []HistorySample{sample(now.Add(-time.Hour), 10), sample(now, 10)}, time.Time{}, false, 0, ""},
		{"reset discards earlier segment", []HistorySample{sample(now.Add(-3*time.Hour), 80), sample(now.Add(-2*time.Hour), 5), sample(now.Add(-time.Hour), 10), sample(now, 15)}, time.Time{}, false, 5, now.Add(17 * time.Hour).Format(time.RFC3339)},
		{"reset leaves one sample", []HistorySample{sample(now.Add(-time.Hour), 80), sample(now, 5)}, time.Time{}, true, 0, ""},
		{"single sample", []HistorySample{sample(now, 10)}, time.Time{}, true, 0, ""},
		{"older than 6h ignored", []HistorySample{sample(now.Add(-7*time.Hour), 1), sample(now, 10)}, time.Time{}, true, 0, ""},
		{"eta after window reset omitted", []HistorySample{sample(now.Add(-2*time.Hour), 10), sample(now, 30)}, now.Add(time.Hour), false, 10, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeBurn(tc.samples, "codex", "5h", now, tc.resetAt)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("want nil, got %+v", got)
				}
				return
			}
			if got == nil || got.BurnRatePerHour != tc.wantRate || got.EtaAt != tc.wantETA {
				t.Fatalf("got %+v, want rate %v eta %q", got, tc.wantRate, tc.wantETA)
			}
		})
	}
}
