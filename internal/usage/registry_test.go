package usage

import (
	"context"
	"testing"
)

func TestRegistryCoversEveryFetcherKey(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range providers {
		if seen[p.Name] {
			t.Fatalf("duplicate provider name %q", p.Name)
		}
		seen[p.Name] = true
		if p.Fetch == nil {
			t.Fatalf("provider %q has no Fetch", p.Name)
		}
		for _, alias := range p.Aliases {
			if seen[alias] {
				t.Fatalf("alias %q collides with an earlier key", alias)
			}
			seen[alias] = true
		}
	}

	for key := range providerFetchers {
		if !seen[key] {
			t.Fatalf("fetcher key %q not backed by the registry", key)
		}
	}
	for _, key := range []string{"agy", "antigravity", "gemini", "claude", "commandcode", "cursor-agent", "copilot", "opencode", "codex", "kimi", "zai", "zhipu", "qwen", "deepseek", "openrouter", "minimax", "grok"} {
		if _, ok := providerFetchers[key]; !ok {
			t.Fatalf("fetcher key %q missing", key)
		}
	}
}

func TestDefaultProviderOrderMatchesRegistry(t *testing.T) {
	want := []string{"agy", "claude", "commandcode", "cursor-agent", "copilot", "opencode", "codex", "kimi", "zai", "qwen", "deepseek", "openrouter", "minimax", "grok"}
	got := defaultProviderOrder()
	if len(got) != len(want) {
		t.Fatalf("defaultProviderOrder() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("defaultProviderOrder() = %v, want %v", got, want)
		}
	}
}

func TestMatchProvider(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantName string
		wantOK   bool
	}{
		{name: "exact binary name", input: "claude", wantName: "claude", wantOK: true},
		{name: "alias", input: "gemini", wantName: "agy", wantOK: true},
		{name: "case and whitespace", input: "  Claude-Code ", wantName: "claude", wantOK: true},
		{name: "substring from display name", input: "github-copilot", wantName: "copilot", wantOK: true},
		{name: "openai maps to codex", input: "openai-codex", wantName: "codex", wantOK: true},
		{name: "command code with space", input: "command code", wantName: "commandcode", wantOK: true},
		{name: "unknown falls through", input: "warp", wantOK: false},
		{name: "empty falls through", input: "", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := matchProvider(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("matchProvider(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if ok && got.Name != tt.wantName {
				t.Fatalf("matchProvider(%q) = %q, want %q", tt.input, got.Name, tt.wantName)
			}
		})
	}
}

// TestDisplayHelpersDerivedFromRegistry pins that the legacy display
// behaviors (colors, icons, compact letters) survive the registry derivation,
// including unknown-name fallbacks.
func TestDisplayHelpersDerivedFromRegistry(t *testing.T) {
	t.Setenv("OCT_NO_ICONS", "")
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("LC_ALL", "en_US.UTF-8")

	if got := compactProviderLabel("claude-code"); got != "C" {
		t.Errorf("compactProviderLabel(claude-code) = %q, want C", got)
	}
	if got := compactProviderLabel("github-copilot"); got != "P" {
		t.Errorf("compactProviderLabel(github-copilot) = %q, want P", got)
	}
	if got := compactProviderLabel("agy"); got != "G" {
		t.Errorf("compactProviderLabel(agy) = %q, want G", got)
	}
	if got := compactProviderLabel(""); got != "?" {
		t.Errorf("compactProviderLabel(empty) = %q, want ?", got)
	}
	if got := compactProviderLabel("warp"); got != "W" {
		t.Errorf("compactProviderLabel(unknown) = %q, want W (first rune upper)", got)
	}

	if got := providerDisplayLabel("antigravity"); got != "✨ antigravity" {
		t.Errorf("providerDisplayLabel(antigravity) = %q, want sparkle prefix", got)
	}
	if got := providerDisplayLabel("warp"); got != "warp" {
		t.Errorf("providerDisplayLabel(unknown) = %q, want unchanged", got)
	}

	if got := colorizeProvider("claude-code", "claude-code"); got != "\x1b[1;93mclaude-code\x1b[0m" {
		t.Errorf("colorizeProvider(claude) = %q", got)
	}
	if got := colorizeProvider("warp", "warp"); got != "warp" {
		t.Errorf("colorizeProvider(unknown) = %q, want unstyled", got)
	}
}

// TestSwappingFetchers still works: tests replace providerFetchers wholesale.
func TestSwappingFetchers(t *testing.T) {
	orig := providerFetchers
	t.Cleanup(func() { providerFetchers = orig })
	providerFetchers = map[string]func(context.Context) UsageResult{
		"codex": func(context.Context) UsageResult { return UsageResult{Provider: "codex", Status: "ok"} },
	}
	if _, ok := providerFetchers["codex"]; !ok {
		t.Fatal("wholesale fetcher swap broken")
	}
}

func TestLookupStandalone(t *testing.T) {
	if p, ok := LookupStandalone("zai"); !ok || p.Name != "zai" {
		t.Fatalf("LookupStandalone(zai) = %q, %v; want zai, true", p.Name, ok)
	}
	if p, ok := LookupStandalone("Zhipu"); !ok || p.Name != "zai" {
		t.Fatalf("LookupStandalone(Zhipu) = %q, %v; want alias resolution to zai", p.Name, ok)
	}
	if p, ok := LookupStandalone("deepseek"); !ok || p.Name != "deepseek" {
		t.Fatalf("LookupStandalone(deepseek) = %q, %v; want deepseek, true", p.Name, ok)
	}
	if _, ok := LookupStandalone("claude"); ok {
		t.Fatal("LookupStandalone(claude) = true; installable tools must not resolve as standalone")
	}
	if _, ok := LookupStandalone(""); ok {
		t.Fatal("LookupStandalone(empty) = true; want false")
	}
}

func TestMinimaxFetcherKeyedByToolBinaryName(t *testing.T) {
	// GetUsage resolves installable providers by update.Tool BinaryName, so
	// the MiniMax tool (mmx) needs a fetcher under that key — otherwise the
	// advertised provider never fetches in the default configuration.
	if _, ok := providerFetchers["mmx"]; !ok {
		t.Fatal(`providerFetchers missing "mmx"; the MiniMax tool would never fetch`)
	}
	if _, ok := providerFetchers["minimax"]; !ok {
		t.Fatal(`providerFetchers missing "minimax"`)
	}
}

func TestCanonicalProviderName(t *testing.T) {
	cases := map[string]string{
		"minimax":     "minimax",
		"mmx":         "minimax", // config UI stores the tool binary name
		"zhipu":       "zai",     // alias
		"claude":      "claude",
		"claude-code": "claude",       // legacy tool name
		"cursor":      "cursor-agent", // legacy display name
		"antigravity": "agy",
	}
	for raw, want := range cases {
		got, ok := CanonicalProviderName(raw)
		if !ok || got != want {
			t.Errorf("CanonicalProviderName(%q) = %q, %v; want %q", raw, got, ok, want)
		}
	}
	if _, ok := CanonicalProviderName("not-a-provider"); ok {
		t.Error("unknown provider resolved")
	}
	if _, ok := CanonicalProviderName(""); ok {
		t.Error("empty name resolved")
	}
}
