package cmd

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestSessionRefreshJSONModeEmitsStructuredResults(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"session-refresh", "--provider", "gemini", "--dry-run", "--json"})
	defer func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	}()

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("session-refresh execute failed: %v", err)
	}

	var results []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
		t.Fatalf("invalid json output: %v\n%s", err, buf.String())
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0]["provider"] != "agy" {
		t.Fatalf("expected agy provider, got %#v", results[0])
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
