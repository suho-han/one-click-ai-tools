package quota

import (
	"testing"
	"time"
)

func TestRenderBar(t *testing.T) {
	cases := []struct {
		name      string
		remaining float64
		width     int
		want      string
	}{
		{"empty", 0, 4, "░░░░"},
		{"full", 100, 4, "████"},
		{"half", 50, 4, "██░░"},
		{"rounds to nearest", 87.5, 8, "███████░"},
		{"clamps negative", -5, 3, "░░░"},
		{"clamps overflow", 150, 3, "███"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RenderBar(tc.remaining, tc.width); got != tc.want {
				t.Fatalf("RenderBar(%v, %d) = %q, want %q", tc.remaining, tc.width, got, tc.want)
			}
		})
	}
}

func TestParseResetTime(t *testing.T) {
	cases := []struct {
		name  string
		raw   string
		want  time.Time
		valid bool
	}{
		{"rfc3339", "2026-09-26T08:00:00+09:00", time.Date(2026, 9, 26, 8, 0, 0, 0, tzoneFixed(+9)), true},
		{"epoch seconds", "1790412359", time.Unix(1790412359, 0), true},
		{"epoch millis", "1790412359000", time.UnixMilli(1790412359000), true},
		{"empty", "", time.Time{}, false},
		{"garbage", "soon", time.Time{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, valid := ParseResetTime(tc.raw)
			if valid != tc.valid {
				t.Fatalf("valid = %t, want %t", valid, tc.valid)
			}
			if valid && !got.Equal(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func tzoneFixed(hours int) *time.Location {
	return time.FixedZone("test", hours*3600)
}

func TestFormatCountdown(t *testing.T) {
	cases := []struct {
		name string
		in   time.Duration
		want string
	}{
		{"already passed", -time.Hour, "<1m"},
		{"seconds", 30 * time.Second, "<1m"},
		{"minutes", 5 * time.Minute, "5m"},
		{"hours+minutes", 3*time.Hour + 12*time.Minute, "3h 12m"},
		{"days+hours", 50 * time.Hour, "2d 2h"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FormatCountdown(tc.in); got != tc.want {
				t.Fatalf("FormatCountdown(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestHumanizeTokens(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{500, "500"},
		{7400, "7.4K"},
		{3_310_000, "3.3M"},
		{742_500_000, "742.5M"},
		{1_500_000_000, "1.5B"},
	}
	for _, tc := range cases {
		if got := HumanizeTokens(tc.in); got != tc.want {
			t.Fatalf("HumanizeTokens(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
