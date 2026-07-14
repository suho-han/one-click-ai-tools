package cmd

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/suho-han/one-click-ai-tools/internal/usage"
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

func TestSortMonitorResults_EmptySortKeyPreservesInputOrder(t *testing.T) {
	in := []usage.UsageResult{
		{Provider: "codex"},
		{Provider: "claude-code"},
		{Provider: "antigravity"},
	}
	out := sortMonitorResults(in, "", false)
	for i := range in {
		if out[i].Provider != in[i].Provider {
			t.Fatalf("expected preserved order at %d: got %s want %s", i, out[i].Provider, in[i].Provider)
		}
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
	if got := usageSeverity(usage.UsageResult{Unit: "percent", Used: "100", Status: "warn", Message: "No data: No local OpenCode session logs found"}); got != "WARN" {
		t.Fatalf("expected WARN for explicit warn status, got %s", got)
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

func TestMonitorProviderDisplayLabel_NoIconsEnv(t *testing.T) {
	t.Setenv("OCT_NO_ICONS", "1")
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("LC_ALL", "en_US.UTF-8")
	if got := monitorProviderDisplayLabel("Cursor"); got != "Cursor" {
		t.Fatalf("expected plain provider when OCT_NO_ICONS=1, got %q", got)
	}
}

func TestMonitorProviderDisplayLabel_NonUTF8Locale(t *testing.T) {
	t.Setenv("OCT_NO_ICONS", "")
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("LC_ALL", "C")
	if got := monitorProviderDisplayLabel("OpenCode"); got != "OpenCode" {
		t.Fatalf("expected plain provider for non-utf8 locale, got %q", got)
	}
}

func TestSortMonitorResults_By5hAnd7d(t *testing.T) {
	in := []usage.UsageResult{
		{Provider: "gamma", Unit: "percent", Used: "10", Buckets: map[string]string{"5h": "20", "7d": "30"}},
		{Provider: "alpha", Unit: "percent", Used: "90", Buckets: map[string]string{"5h": "80", "7d": "40"}},
		{Provider: "beta", Unit: "percent", Used: "50", Buckets: map[string]string{"5h": "40", "7d": "95"}},
	}

	by5h := sortMonitorResults(in, "5h", true)
	if got := by5h[0].Provider; got != "alpha" {
		t.Fatalf("expected alpha first for --sort-by 5h --desc, got %s", got)
	}

	by7d := sortMonitorResults(in, "7d", true)
	if got := by7d[0].Provider; got != "beta" {
		t.Fatalf("expected beta first for --sort-by 7d --desc, got %s", got)
	}
}

func TestPrintMonitorScreen_AutoCompactOnNarrowWidth(t *testing.T) {
	t.Setenv("COLUMNS", "90")
	out := captureStdout(t, func() {
		printMonitorScreen([]usage.UsageResult{
			{Provider: "codex", Unit: "percent", Used: "88", Limit: "100", Status: "warn", Message: "high usage", Buckets: map[string]string{"5h": "88", "7d": "55"}},
		}, time.Now(), false)
	})

	if !strings.Contains(out, "provider") || !strings.Contains(out, "sev") {
		t.Fatalf("expected compact header columns in output, got: %q", out)
	}
	if strings.Contains(out, "message") {
		t.Fatalf("expected compact mode to omit message header, got: %q", out)
	}
}

func TestPrintMonitorScreen_TruncatesMessageByWidth(t *testing.T) {
	t.Setenv("COLUMNS", "110")
	longMsg := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	out := captureStdout(t, func() {
		printMonitorScreen([]usage.UsageResult{
			{Provider: "codex", Unit: "percent", Used: "88", Limit: "100", Status: "warn", Message: longMsg, Buckets: map[string]string{"5h": "88", "7d": "55"}},
		}, time.Now(), false)
	})

	if !strings.Contains(out, "message") {
		t.Fatalf("expected full header with message column, got: %q", out)
	}
	if !strings.Contains(out, "abcdefghijklmnopq...") {
		t.Fatalf("expected message truncation for width 110, got: %q", out)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe create failed: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()
	_ = w.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read captured stdout failed: %v", err)
	}
	return string(b)
}
