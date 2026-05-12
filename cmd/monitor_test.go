package cmd

import (
	"testing"

	"github.com/suho-han/one-click-tools/internal/usage"
)

func TestSortMonitorResults(t *testing.T) {
	in := []usage.UsageResult{
		{Provider: "b", Unit: "percent", Used: "88", Buckets: map[string]string{"5h": "80"}},
		{Provider: "a", Unit: "percent", Used: "92", Buckets: map[string]string{"5h": "95"}},
	}
	out := sortMonitorResults(in, "used", true)
	if out[0].Provider != "a" {
		t.Fatalf("expected provider a first, got %s", out[0].Provider)
	}
}

func TestUsageSeverity(t *testing.T) {
	if got := usageSeverity(usage.UsageResult{Unit: "percent", Used: "70"}); got != "OK" {
		t.Fatalf("expected OK, got %s", got)
	}
	if got := usageSeverity(usage.UsageResult{Unit: "percent", Used: "88"}); got != "WARN" {
		t.Fatalf("expected WARN, got %s", got)
	}
	if got := usageSeverity(usage.UsageResult{Unit: "percent", Used: "99"}); got != "CRIT" {
		t.Fatalf("expected CRIT, got %s", got)
	}
}

func TestMonitorColorHelpers_DarkTerminalFriendly(t *testing.T) {
	if got := colorizeSeverityLabel("CRIT"); got != "\x1b[1;91mCRIT\x1b[0m" {
		t.Fatalf("unexpected CRIT color label: %q", got)
	}
	if got := colorizeMonitorStatus("warn"); got != "\x1b[1;93mwarn\x1b[0m" {
		t.Fatalf("unexpected warn status label: %q", got)
	}
	if got := colorizeMonitorProvider("cursor"); got != "\x1b[1;94mcursor\x1b[0m" {
		t.Fatalf("expected updated cursor color label, got %q", got)
	}
	if got := colorizeMonitorProvider("claude"); got == "claude" {
		t.Fatalf("expected colored provider label, got %q", got)
	}
}

func TestMonitorWidthHelpers(t *testing.T) {
	if got := monitorMessageWidth(90); got != 0 {
		t.Fatalf("expected compact mode msg width 0 for narrow terminal, got %d", got)
	}
	if got := monitorMessageWidth(110); got != 20 {
		t.Fatalf("expected medium msg width 20, got %d", got)
	}
	if got := truncateMonitorText("abcdefghijklmnopqrstuvwxyz", 10); got != "abcdefg..." {
		t.Fatalf("unexpected truncateMonitorText result: %q", got)
	}
}

func TestPadANSI_VisibleWidth(t *testing.T) {
	colored := colorizeMonitorProvider("codex")
	padded := padANSI(colored, 14)
	if got := visibleLenANSI(padded); got != 14 {
		t.Fatalf("expected visible width 14, got %d (%q)", got, padded)
	}
}

func TestMonitorProviderDisplayLabel_IconCapability(t *testing.T) {
	t.Setenv("OCT_NO_ICONS", "")
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("LC_ALL", "en_US.UTF-8")
	if got := monitorProviderDisplayLabel("Cursor"); got != "▣ Cursor" {
		t.Fatalf("expected cursor icon label, got %q", got)
	}

	t.Setenv("TERM", "dumb")
	if got := monitorProviderDisplayLabel("OpenCode"); got != "OpenCode" {
		t.Fatalf("expected plain provider for dumb term, got %q", got)
	}
}
