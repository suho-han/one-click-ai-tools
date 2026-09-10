package usage

import (
	"context"
	"strings"
)

// Provider describes one AI provider's identity and fetch entrypoint.
// Adding a provider means adding one entry here (plus its fetch file); the
// fetcher map, default order, ANSI colors, table icons, and compact-title
// letters all derive from this registry, so no other site needs editing.
type Provider struct {
	// Name is the canonical registry key (a tool BinaryName).
	Name string
	// Aliases are additional fetcher lookup keys ("gemini"/"antigravity" for agy).
	Aliases []string
	// MatchSubstrings are lowercase substrings that identify this provider in
	// arbitrary display names (fetch results may carry any provider string,
	// e.g. "github-copilot" or "claude-code").
	MatchSubstrings []string
	// ColorCode is the ANSI SGR parameter for the usage table ("" = unstyled).
	ColorCode string
	// Label is the icon prefix shown in the usage table ("" = no prefix).
	Label string
	// CompactLabel is the letter used in compact titles ("" = derive from name).
	CompactLabel string
	// Fetch collects the provider's usage. It reports problems through the
	// UsageResult itself, never via error.
	Fetch func(ctx context.Context) UsageResult
}

// providers is the single source of truth for provider identity. Order also
// defines the default provider order (SelectedTools fallback).
var providers = []Provider{
	{
		Name:            "agy",
		Aliases:         []string{"antigravity", "gemini"},
		MatchSubstrings: []string{"antigravity", "gemini"},
		ColorCode:       "94",
		Label:           "✨ ",
		CompactLabel:    "G",
		Fetch:           FetchAntigravityUsage,
	},
	{
		Name:            "claude",
		MatchSubstrings: []string{"claude"},
		ColorCode:       "93",
		CompactLabel:    "C",
		Fetch:           FetchClaudeUsage,
	},
	{
		Name:            "commandcode",
		MatchSubstrings: []string{"commandcode", "command code"},
		ColorCode:       "94",
		Label:           "⌘ ",
		CompactLabel:    "D",
		Fetch:           FetchCommandCodeUsage,
	},
	{
		Name:            "cursor-agent",
		MatchSubstrings: []string{"cursor"},
		ColorCode:       "94",
		Label:           "▣ ",
		CompactLabel:    "R",
		Fetch:           FetchCursorUsage,
	},
	{
		Name:            "copilot",
		MatchSubstrings: []string{"copilot", "github"},
		ColorCode:       "95",
		CompactLabel:    "P",
		Fetch:           FetchCopilotUsage,
	},
	{
		Name:            "opencode",
		MatchSubstrings: []string{"opencode"},
		ColorCode:       "97",
		Label:           "🧩 ",
		CompactLabel:    "O",
		Fetch:           FetchOpenCodeUsage,
	},
	{
		Name:            "codex",
		MatchSubstrings: []string{"codex", "openai"},
		ColorCode:       "96",
		CompactLabel:    "X",
		Fetch:           FetchCodexUsage,
	},
}

// providerFetchers maps binary names and aliases to fetchers. Package-level
// var so tests can substitute fetchers wholesale.
var providerFetchers = buildProviderFetchers()

func buildProviderFetchers() map[string]func(context.Context) UsageResult {
	fetchers := make(map[string]func(context.Context) UsageResult, len(providers)*2)
	for _, p := range providers {
		fetchers[p.Name] = p.Fetch
		for _, alias := range p.Aliases {
			fetchers[alias] = p.Fetch
		}
	}
	return fetchers
}

// defaultProviderOrder returns the fallback agent_order: every registry
// provider by declaration order, aliases excluded.
func defaultProviderOrder() []string {
	order := make([]string, 0, len(providers))
	for _, p := range providers {
		order = append(order, p.Name)
	}
	return order
}

// matchProvider resolves an arbitrary provider display string to a registry
// entry, by exact key first and lowercase substring second.
func matchProvider(name string) (Provider, bool) {
	p := strings.ToLower(strings.TrimSpace(name))
	if p == "" {
		return Provider{}, false
	}
	for _, entry := range providers {
		if entry.Name == p {
			return entry, true
		}
		for _, alias := range entry.Aliases {
			if alias == p {
				return entry, true
			}
		}
	}
	for _, entry := range providers {
		for _, sub := range entry.MatchSubstrings {
			if strings.Contains(p, sub) {
				return entry, true
			}
		}
	}
	return Provider{}, false
}
