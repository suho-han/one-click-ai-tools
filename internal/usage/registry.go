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
	// Standalone marks providers with no installable CLI tool behind them
	// (plan/account services). They have no update.Tool entry, so GetUsage
	// fetches them only when the user lists them in agent_order or
	// enabled_tools — an unconfigured standalone service must never add a
	// permanent "not configured" row to the default usage table.
	Standalone bool
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
	{
		Name:            "kimi",
		MatchSubstrings: []string{"kimi", "moonshot"},
		ColorCode:       "92",
		CompactLabel:    "K",
		Fetch:           FetchKimiUsage,
	},
	{
		Name:            "zai",
		Aliases:         []string{"zhipu"},
		MatchSubstrings: []string{"zai", "glm", "zhipu", "bigmodel"},
		ColorCode:       "91",
		CompactLabel:    "Z",
		Standalone:      true,
		Fetch:           FetchZaiUsage,
	},
	{
		Name:            "qwen",
		MatchSubstrings: []string{"qwen"},
		ColorCode:       "94",
		CompactLabel:    "Q",
		Fetch:           FetchQwenUsage,
	},
	{
		Name:            "deepseek",
		MatchSubstrings: []string{"deepseek"},
		ColorCode:       "95",
		CompactLabel:    "S",
		Standalone:      true,
		Fetch:           FetchDeepseekUsage,
	},
	{
		Name:            "openrouter",
		MatchSubstrings: []string{"openrouter"},
		ColorCode:       "94",
		CompactLabel:    "N",
		Standalone:      true,
		Fetch:           FetchOpenRouterUsage,
	},
	{
		Name:            "minimax",
		Aliases:         []string{"mmx"},
		MatchSubstrings: []string{"minimax"},
		ColorCode:       "93",
		CompactLabel:    "M",
		Fetch:           FetchMinimaxUsage,
	},
	{
		Name:            "grok",
		MatchSubstrings: []string{"grok", "xai", "supergrok"},
		ColorCode:       "95",
		CompactLabel:    "V",
		Standalone:      true,
		Fetch:           FetchGrokUsage,
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

// LookupStandalone resolves a raw config entry (exact name or alias match) to
// a standalone provider. Installable tools resolve through update.Tools, so
// config validation and the interactive save path use this to accept and
// preserve the usage-only provider names.
func LookupStandalone(name string) (Provider, bool) {
	p := strings.ToLower(strings.TrimSpace(name))
	if p == "" {
		return Provider{}, false
	}
	for _, entry := range providers {
		if !entry.Standalone {
			continue
		}
		if entry.Name == p {
			return entry, true
		}
		for _, alias := range entry.Aliases {
			if alias == p {
				return entry, true
			}
		}
	}
	return Provider{}, false
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
