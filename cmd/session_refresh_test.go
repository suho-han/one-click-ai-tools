package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/suho-han/one-click-tools/internal/sessionrefresh"
	"github.com/suho-han/one-click-tools/internal/usage"
)

func TestSessionRefreshJSONModeEmitsStructuredResultsAndUsage(t *testing.T) {
	origRun := sessionRefreshRun
	origGetUsage := sessionRefreshGetUsage
	t.Cleanup(func() {
		sessionRefreshRun = origRun
		sessionRefreshGetUsage = origGetUsage
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	})

	sessionRefreshRun = func(opts sessionrefresh.RefreshOptions) []sessionrefresh.RefreshResult {
		return []sessionrefresh.RefreshResult{{
			Provider:   "agy",
			Supported:  true,
			Mode:       "local-session",
			Status:     "ok",
			Confidence: "verified",
			Message:    "Local Antigravity session artifacts detected",
		}}
	}
	sessionRefreshGetUsage = func() ([]usage.UsageResult, error) {
		return []usage.UsageResult{{
			Provider: "antigravity",
			Status:   "ok",
			Used:     "3",
			Unit:     "sessions",
			Message:  "Estimated from 3 local Antigravity session artifacts",
		}}, nil
	}

	buf := bytes.NewBuffer(nil)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"session-refresh", "--provider", "gemini", "--json"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("session-refresh execute failed: %v", err)
	}

	var output struct {
		RefreshResults []map[string]any `json:"refresh_results"`
		Usage          []map[string]any `json:"usage"`
	}
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("invalid json output: %v\n%s", err, buf.String())
	}
	if len(output.RefreshResults) != 1 {
		t.Fatalf("expected 1 refresh result, got %d", len(output.RefreshResults))
	}
	if output.RefreshResults[0]["provider"] != "agy" {
		t.Fatalf("expected agy refresh provider, got %#v", output.RefreshResults[0])
	}
	if output.RefreshResults[0]["confidence"] != "verified" {
		t.Fatalf("expected verified confidence, got %#v", output.RefreshResults[0])
	}
	if len(output.Usage) != 1 {
		t.Fatalf("expected 1 usage result, got %d", len(output.Usage))
	}
	if output.Usage[0]["provider"] != "antigravity" {
		t.Fatalf("expected antigravity usage provider, got %#v", output.Usage[0])
	}
}

func TestSessionRefreshTextModePrintsRefreshedUsage(t *testing.T) {
	origRun := sessionRefreshRun
	origGetUsage := sessionRefreshGetUsage
	t.Cleanup(func() {
		sessionRefreshRun = origRun
		sessionRefreshGetUsage = origGetUsage
	})

	sessionRefreshRun = func(opts sessionrefresh.RefreshOptions) []sessionrefresh.RefreshResult {
		return []sessionrefresh.RefreshResult{{Provider: "codex", Status: "ok", Confidence: "verified", Mode: "auth-status", Message: "Logged in using ChatGPT"}}
	}
	sessionRefreshGetUsage = func() ([]usage.UsageResult, error) {
		return []usage.UsageResult{{Provider: "codex", Period: "current", Used: "1.0", Limit: "100", Unit: "percent", Source: "local", Status: "ok", Message: "Usage extracted from local Codex session logs"}}, nil
	}

	buf := bytes.NewBuffer(nil)
	printSessionRefreshResults(buf, sessionRefreshRun(sessionrefresh.RefreshOptions{}))
	buf.WriteString("\nrefreshed usage\n")
	usageResults, err := sessionRefreshGetUsage()
	usage.RenderTable(buf, mustUsage(t, usageResults, err))
	out := buf.String()
	if !strings.Contains(out, "refreshed usage") {
		t.Fatalf("expected refreshed usage section, got:\n%s", out)
	}
	if !strings.Contains(out, "codex") {
		t.Fatalf("expected codex in output, got:\n%s", out)
	}
	if !strings.Contains(out, "verified") {
		t.Fatalf("expected confidence column in output, got:\n%s", out)
	}
}

func TestSplitCommaProviders(t *testing.T) {
	got := splitCommaProviders([]string{"codex, cursor", "gemini"})
	want := []string{"codex", "cursor", "gemini"}
	if len(got) != len(want) {
		t.Fatalf("splitCommaProviders len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("splitCommaProviders[%d] = %q, want %q (%v)", i, got[i], want[i], got)
		}
	}
}

func mustUsage(t *testing.T, results []usage.UsageResult, err error) []usage.UsageResult {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected usage error: %v", err)
	}
	return results
}
