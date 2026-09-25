package usage

import (
	"testing"
)

func TestParsePercent(t *testing.T) {
	tests := []struct {
		raw    string
		want   float64
		wantOK bool
	}{
		{raw: "45.5", want: 45.5, wantOK: true},
		{raw: " 80 ", want: 80, wantOK: true},
		{raw: "33%", want: 33, wantOK: true},
		{raw: "-5", want: -5, wantOK: true},
		{raw: "150", want: 150, wantOK: true},
		{raw: "", wantOK: false},
		{raw: "n/a", wantOK: false},
	}
	for _, tt := range tests {
		got, ok := ParsePercent(tt.raw)
		if ok != tt.wantOK {
			t.Errorf("ParsePercent(%q) ok = %v, want %v", tt.raw, ok, tt.wantOK)
			continue
		}
		if ok && got != tt.want {
			t.Errorf("ParsePercent(%q) = %v, want %v", tt.raw, got, tt.want)
		}
	}
}

func TestClampPercent(t *testing.T) {
	tests := []struct{ in, want float64 }{
		{-1, 0}, {0, 0}, {45.5, 45.5}, {100, 100}, {150, 100},
	}
	for _, tt := range tests {
		if got := ClampPercent(tt.in); got != tt.want {
			t.Errorf("ClampPercent(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestUsedFromRemainingPercent(t *testing.T) {
	tests := []struct{ in, want float64 }{
		{100, 0}, {87.5, 12.5}, {0, 100}, {-20, 100}, {120, 0},
	}
	for _, tt := range tests {
		if got := UsedFromRemainingPercent(tt.in); got != tt.want {
			t.Errorf("UsedFromRemainingPercent(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestRemainingFromUsedPercent(t *testing.T) {
	tests := []struct {
		used   string
		want   string
		wantOK bool
	}{
		{used: "55", want: "45.0", wantOK: true},
		{used: "100", want: "0.0", wantOK: true},
		{used: "0", want: "100.0", wantOK: true},
		{used: "-5", want: "105.0", wantOK: true}, // pinned: no upper clamp
		{used: "105", want: "0.0", wantOK: true},  // pinned: negative floors to 0
		{used: "n/a", wantOK: false},
		{used: "", wantOK: false},
	}
	for _, tt := range tests {
		got, ok := RemainingFromUsedPercent(tt.used)
		if ok != tt.wantOK {
			t.Errorf("RemainingFromUsedPercent(%q) ok = %v, want %v", tt.used, ok, tt.wantOK)
			continue
		}
		if ok && got != tt.want {
			t.Errorf("RemainingFromUsedPercent(%q) = %q, want %q", tt.used, got, tt.want)
		}
	}
}

func TestPercentLabel(t *testing.T) {
	tests := []struct {
		used string
		want string
		ok   bool
	}{
		{used: "45.5", want: "54%", ok: true}, // 100-45.5=54.5, %.0f rounds-to-even
		{used: "100", want: "0%", ok: true},
		{used: "0", want: "100%", ok: true},
		{used: "-10", want: "100%", ok: true},  // clamped
		{used: "120", want: "0%", ok: true},    // clamped
		{used: "12.3%", want: "88%", ok: true}, // suffix tolerated
		{used: "n/a", ok: false},
	}
	for _, tt := range tests {
		got, ok := PercentLabel(tt.used)
		if ok != tt.ok {
			t.Errorf("PercentLabel(%q) ok = %v, want %v", tt.used, ok, tt.ok)
			continue
		}
		if ok && got != tt.want {
			t.Errorf("PercentLabel(%q) = %q, want %q", tt.used, got, tt.want)
		}
	}
}
