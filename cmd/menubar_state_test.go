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

func TestMenubarOverviewAndProviderSectionLabels(t *testing.T) {
	if got := menubarOverviewTitle(); got != "Usage Overview" {
		t.Fatalf("menubarOverviewTitle = %q", got)
	}
	if got := menubarProviderSectionTitle(0); got != "Providers" {
		t.Fatalf("menubarProviderSectionTitle(0) = %q", got)
	}
	if got := menubarProviderSectionTitle(6); got != "Providers (6)" {
		t.Fatalf("menubarProviderSectionTitle(6) = %q", got)
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

func TestMenubarProviderLineIncludesBadgeBucketsAndStatus(t *testing.T) {
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

	for _, want := range []string{"[warn]", "Codex", "5h 88%", "7d 64%"} {
		if !strings.Contains(line, want) {
			t.Fatalf("line = %q, want substring %q", line, want)
		}
	}
}

func TestMenubarProviderLineOmitsMessageForOKStatus(t *testing.T) {
	line := menubarProviderLine(usage.UsageResult{
		Provider: "Copilot",
		Status:   "ok",
		Message:  "should stay hidden",
	})
	if strings.Contains(line, "should stay hidden") {
		t.Fatalf("line = %q, want ok provider message omitted", line)
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
	script := buildTerminalAppleScript(`'/tmp/oct' 'usage' '--flag=\\\"quoted\\\"'`)
	if !strings.Contains(script, `do script "'/tmp/oct' 'usage' '--flag=\\\\\\\"quoted\\\\\\\"'"`) {
		t.Fatalf("script = %q, want escaped do script payload", script)
	}
}

func TestMenubarProviderDetailsIncludesDeepStatus(t *testing.T) {
	details := menubarProviderDetails(usage.UsageResult{
		Provider:     "Codex",
		Plan:         "plus",
		PlanSource:   "codex auth.jwt id_token",
		Status:       "warn",
		Used:         "88",
		Limit:        "100",
		Unit:         "percent",
		Source:       "local",
		SourceDetail: "session logs",
		Message:      "approaching threshold",
		Buckets: map[string]string{
			"5h": "88",
			"7d": "64",
		},
	})

	for _, want := range []string{"Provider: Codex", "Status: warn", "5h: 88%", "7d: 64%", "Plan: plus", "Plan source: codex auth.jwt id_token", "Used: 88", "Limit: 100", "Source: local", "Detail: session logs", "Message: approaching threshold"} {
		found := false
		for _, got := range details {
			if strings.Contains(got, want) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("details = %#v, want substring %q", details, want)
		}
	}
}

func TestMenubarRefreshIntervalFallsBackAndFormatsLabel(t *testing.T) {
	if got := menubarRefreshInterval(""); got != time.Minute {
		t.Fatalf("menubarRefreshInterval(empty) = %s, want 1m", got)
	}
	if got := menubarRefreshInterval("90s"); got != 90*time.Second {
		t.Fatalf("menubarRefreshInterval(90s) = %s, want 90s", got)
	}
	if got := menubarAutoRefreshLabel(90 * time.Second); got != "Auto refresh: every 1m30s" {
		t.Fatalf("menubarAutoRefreshLabel = %q", got)
	}
}

func TestMenubarNextRefreshLabel(t *testing.T) {
	now := time.Date(2026, 6, 12, 15, 26, 4, 0, time.UTC)
	if got := menubarNextRefreshLabel(time.Time{}, time.Minute); got != "Next refresh: pending" {
		t.Fatalf("menubarNextRefreshLabel(zero) = %q", got)
	}
	if got := menubarNextRefreshLabel(now, time.Minute); got != "Next refresh: 15:27:04" {
		t.Fatalf("menubarNextRefreshLabel = %q", got)
	}
}

type staticErr string

func (e staticErr) Error() string { return string(e) }

func assertErr(msg string) error { return staticErr(msg) }
