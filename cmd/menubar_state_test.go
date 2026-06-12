package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/suho-han/one-click-tools/internal/usage"
)

func TestMenubarLoadingSnapshot(t *testing.T) {
	snap := buildMenubarLoadingSnapshot([]string{"Claude Code", "Codex"})
	if snap.Title != "oct …" {
		t.Fatalf("Title = %q, want %q", snap.Title, "oct …")
	}
	if !strings.Contains(snap.SummaryLine, "Loading usage") {
		t.Fatalf("SummaryLine = %q, want loading summary", snap.SummaryLine)
	}
	if len(snap.ProviderLines) != 2 {
		t.Fatalf("ProviderLines len = %d, want 2", len(snap.ProviderLines))
	}
}

func TestMenubarUsageSnapshotSummarizesCounts(t *testing.T) {
	now := time.Date(2026, 6, 12, 14, 5, 6, 0, time.FixedZone("KST", 9*3600))
	results := []usage.UsageResult{
		{Provider: "Claude", Status: "ok", Used: "42", Unit: "percent", Buckets: map[string]string{"5h": "42"}},
		{Provider: "Codex", Status: "warn", Used: "88", Unit: "percent", Buckets: map[string]string{"5h": "88", "7d": "64"}},
		{Provider: "Copilot", Status: "error", Message: "auth missing"},
	}

	snap := buildMenubarUsageSnapshot(results, now)
	if snap.Title != "oct !!" {
		t.Fatalf("Title = %q, want %q", snap.Title, "oct !!")
	}
	if got := snap.SummaryLine; got != "3 providers · 1 ok · 1 warn · 1 error" {
		t.Fatalf("SummaryLine = %q", got)
	}
	if got := snap.UpdatedLine; got != "Last refresh: 14:05:06" {
		t.Fatalf("UpdatedLine = %q", got)
	}
	if len(snap.ProviderLines) != 3 {
		t.Fatalf("ProviderLines len = %d, want 3", len(snap.ProviderLines))
	}
}

func TestMenubarProviderLineIncludesBucketsAndStatus(t *testing.T) {
	line := menubarProviderLine(usage.UsageResult{
		Provider: "Codex",
		Status:   "warn",
		Used:     "88",
		Unit:     "percent",
		Buckets: map[string]string{
			"5h": "88",
			"7d": "64",
		},
	})

	for _, want := range []string{"Codex", "5h 88%", "7d 64%", "warn"} {
		if !strings.Contains(line, want) {
			t.Fatalf("line = %q, want substring %q", line, want)
		}
	}
}

func TestMenubarErrorSnapshotPreservesTimestamp(t *testing.T) {
	now := time.Date(2026, 6, 12, 14, 5, 6, 0, time.UTC)
	snap := buildMenubarErrorSnapshot([]string{"Claude Code"}, now, assertErr("boom"))
	if snap.Title != "oct !!" {
		t.Fatalf("Title = %q, want %q", snap.Title, "oct !!")
	}
	if !strings.Contains(snap.SummaryLine, "Refresh failed") {
		t.Fatalf("SummaryLine = %q, want refresh failure", snap.SummaryLine)
	}
	if got := snap.UpdatedLine; got != "Last refresh: 14:05:06" {
		t.Fatalf("UpdatedLine = %q", got)
	}
}

func TestMenubarShellQuote(t *testing.T) {
	got := shellQuote("/tmp/o'ct path")
	want := "'/tmp/o'\\''ct path'"
	if got != want {
		t.Fatalf("shellQuote = %q, want %q", got, want)
	}
}

func TestMenubarCommandUsesCurrentExecutable(t *testing.T) {
	got := buildMenubarExecCommand("/tmp/oct binary", "usage", "--json")
	want := "'/tmp/oct binary' 'usage' '--json'"
	if got != want {
		t.Fatalf("buildMenubarExecCommand = %q, want %q", got, want)
	}
}

func TestMenubarAppleScriptEscapesCommand(t *testing.T) {
	script := buildTerminalAppleScript(`'/tmp/oct' 'usage' '--flag=\"quoted\"'`)
	if !strings.Contains(script, `do script "'/tmp/oct' 'usage' '--flag=\\\"quoted\\\"'"`) {
		t.Fatalf("script = %q, want escaped do script payload", script)
	}
}

type staticErr string

func (e staticErr) Error() string { return string(e) }

func assertErr(msg string) error { return staticErr(msg) }
