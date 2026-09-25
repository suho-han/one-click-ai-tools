package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/suho-han/one-click-ai-tools/internal/usage"
)

func TestShouldAutoJSONFallback(t *testing.T) {
	tests := []struct {
		name        string
		jsonMode    bool
		compactMode bool
		format      string
		isTTY       bool
		want        bool
	}{
		{name: "json already requested", jsonMode: true, isTTY: false, want: false},
		{name: "compact already requested", compactMode: true, isTTY: false, want: false},
		{name: "explicit format requested", format: "waybar", isTTY: false, want: false},
		{name: "tty and no output flag", isTTY: true, want: false},
		{name: "non tty and no output flag", isTTY: false, want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldAutoJSONFallback(tc.jsonMode, tc.compactMode, tc.format, tc.isTTY)
			if got != tc.want {
				t.Fatalf("shouldAutoJSONFallback(%v, %v, %q, %v) = %v, want %v", tc.jsonMode, tc.compactMode, tc.format, tc.isTTY, got, tc.want)
			}
		})
	}
}

func TestResolveUsageOutputMode(t *testing.T) {
	tests := []struct {
		name        string
		jsonMode    bool
		compactMode bool
		format      string
		want        string
		wantErr     bool
	}{
		{name: "defaults to none", want: ""},
		{name: "json flag", jsonMode: true, want: "json"},
		{name: "compact flag", compactMode: true, want: "compact"},
		{name: "format waybar", format: "waybar", want: "waybar"},
		{name: "format is case-insensitive", format: "SwiftBar", want: "swiftbar"},
		{name: "format overrides json", jsonMode: true, format: "polybar", want: "polybar"},
		{name: "invalid format errors", format: "fancy", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveUsageOutputMode(tc.jsonMode, tc.compactMode, tc.format)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("resolveUsageOutputMode(%v,%v,%q) = %q, want error", tc.jsonMode, tc.compactMode, tc.format, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveUsageOutputMode(%v,%v,%q) errored: %v", tc.jsonMode, tc.compactMode, tc.format, err)
			}
			if got != tc.want {
				t.Fatalf("resolveUsageOutputMode(%v,%v,%q) = %q, want %q", tc.jsonMode, tc.compactMode, tc.format, got, tc.want)
			}
		})
	}
}

func captureCommandStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe failed: %v", err)
	}
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("read pipe failed: %v", err)
	}
	_ = r.Close()
	return strings.TrimSpace(buf.String())
}

func TestUsageCommandPrintsCompactRemainingOutput(t *testing.T) {
	orig := usageFetcher
	usageFetcher = func(ctx context.Context) ([]usage.UsageResult, error) {
		return []usage.UsageResult{
			{Provider: "codex", Unit: "percent", Used: "80.0", Buckets: map[string]string{"7d": "55.0"}},
			{Provider: "claude-code", Unit: "percent", Used: "12.0", Buckets: map[string]string{"5h": "12.0"}},
		}, nil
	}
	defer func() { usageFetcher = orig }()

	cmd := *usageCmd
	cmd.SetArgs([]string{"--compact"})
	if err := cmd.Flags().Set("compact", "true"); err != nil {
		t.Fatalf("set compact flag failed: %v", err)
	}
	// The flag set is shared across in-process runs; clear --format so an
	// earlier format test cannot override --compact.
	if err := cmd.Flags().Set("format", ""); err != nil {
		t.Fatalf("reset format flag failed: %v", err)
	}
	out := captureCommandStdout(t, func() {
		if err := cmd.RunE(&cmd, nil); err != nil {
			t.Fatalf("usage --compact returned error: %v", err)
		}
	})

	if got, want := out, "X-45% C-88%"; got != want {
		t.Fatalf("usage --compact output = %q, want %q", got, want)
	}
}

func TestUsageOrderedTools_RespectsEnabledTools(t *testing.T) {
	oldOrder := viper.GetStringSlice("agent_order")
	oldEnabled := viper.GetStringSlice("enabled_tools")
	t.Cleanup(func() {
		viper.Set("agent_order", oldOrder)
		viper.Set("enabled_tools", oldEnabled)
	})

	viper.Set("agent_order", []string{"agy", "claude", "cursor", "copilot", "opencode", "codex"})
	viper.Set("enabled_tools", []string{"codex", "opencode"})

	tools := usageOrderedTools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	// Filtering honors enabled_tools, but the row sequence follows agent_order
	// (the execution priority) so the TTY table agrees with `usage --json` and
	// the menubar popover.
	if tools[0].BinaryName != "opencode" || tools[1].BinaryName != "codex" {
		t.Fatalf("unexpected tool order: %s, %s", tools[0].BinaryName, tools[1].BinaryName)
	}
}

func TestUsageHelpUsesAntigravityCanonicalWording(t *testing.T) {
	got := usageCmd.Long
	if !contains(got, "Antigravity") {
		t.Fatalf("expected usage help to mention Antigravity, got: %s", got)
	}
	if !contains(got, "Legacy aliases 'gemini' and 'gemini-cli' still map to 'agy' for compatibility.") {
		t.Fatalf("expected canonical legacy-alias wording, got: %s", got)
	}
}

func TestUsageCommandReturnsErrorForJSONFetchFailure(t *testing.T) {
	orig := usageFetcher
	usageFetcher = func(ctx context.Context) ([]usage.UsageResult, error) {
		return nil, fmt.Errorf("boom")
	}
	defer func() { usageFetcher = orig }()

	cmd := *usageCmd
	cmd.SetArgs([]string{"--json"})
	err := cmd.RunE(&cmd, nil)
	if err == nil || !contains(err.Error(), "fetch usage") {
		t.Fatalf("expected fetch usage error, got %v", err)
	}
}
