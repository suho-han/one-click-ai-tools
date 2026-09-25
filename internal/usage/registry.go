package usage

import (
	"context"
	"sort"
	"strings"

	"github.com/spf13/viper"
	"github.com/suho-han/one-click-ai-tools/internal/update"
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
	// AccountKind selects the per-account override this provider supports
	// (<provider>:<name> usage rows under the accounts config key). AccountKindNone
	// (the default) means the provider has no multi-account support.
	AccountKind AccountKind
	// FetchForAccount collects usage for one configured account. Required
	// when AccountKind is set.
	FetchForAccount func(ctx context.Context, account Account) UsageResult
	// DescribeAccountCredential probes one account's credential chain;
	// optional even when AccountKind is set.
	DescribeAccountCredential func(account Account) CredentialStatus
	// Fetch collects the provider's usage. It reports problems through the
	// UsageResult itself, never via error.
	Fetch func(ctx context.Context) UsageResult
	// DescribeCredential probes the provider's credential chain, reporting
	// which source would be used. Probes are local-only: existence checks
	// without network calls, and secret values are never read into memory
	// for reporting. See credentials.go for the shared types.
	DescribeCredential func() CredentialStatus
}

// providers is the single source of truth for provider identity. Order also
// defines the default provider order (SelectedTools fallback).
var providers = []Provider{
	{
		Name:               "agy",
		Aliases:            []string{"antigravity", "gemini"},
		MatchSubstrings:    []string{"antigravity", "gemini"},
		ColorCode:          "94",
		Label:              "✨ ",
		CompactLabel:       "G",
		Fetch:              FetchAntigravityUsage,
		DescribeCredential: describeAntigravityCredential,
	},
	{
		Name:               "claude",
		MatchSubstrings:    []string{"claude"},
		ColorCode:          "93",
		CompactLabel:       "C",
		Fetch:              FetchClaudeUsage,
		DescribeCredential: describeClaudeCredential,
	},
	{
		Name:               "commandcode",
		MatchSubstrings:    []string{"commandcode", "command code"},
		ColorCode:          "94",
		Label:              "⌘ ",
		CompactLabel:       "D",
		Fetch:              FetchCommandCodeUsage,
		DescribeCredential: describeCommandCodeCredential,
	},
	{
		Name:               "cursor-agent",
		MatchSubstrings:    []string{"cursor"},
		ColorCode:          "94",
		Label:              "▣ ",
		CompactLabel:       "R",
		Fetch:              FetchCursorUsage,
		DescribeCredential: describeCursorCredential,
	},
	{
		Name:               "copilot",
		MatchSubstrings:    []string{"copilot", "github"},
		ColorCode:          "95",
		CompactLabel:       "P",
		Fetch:              FetchCopilotUsage,
		DescribeCredential: describeCopilotCredential,
	},
	{
		Name:               "opencode",
		MatchSubstrings:    []string{"opencode"},
		ColorCode:          "97",
		Label:              "🧩 ",
		CompactLabel:       "O",
		Fetch:              FetchOpenCodeUsage,
		DescribeCredential: describeOpenCodeCredential,
	},
	{
		Name:               "codex",
		MatchSubstrings:    []string{"codex", "openai"},
		ColorCode:          "96",
		CompactLabel:       "X",
		Fetch:              FetchCodexUsage,
		DescribeCredential: describeCodexCredential,
		AccountKind:        AccountKindHome,
		FetchForAccount: func(ctx context.Context, account Account) UsageResult {
			return fetchCodexUsageForHome(ctx, account.Home, AccountProviderName(account.Provider, account.Name))
		},
		DescribeAccountCredential: func(account Account) CredentialStatus {
			return describeCodexCredentialForHome(account.Home)
		},
	},
	{
		Name:               "kimi",
		MatchSubstrings:    []string{"kimi", "moonshot"},
		ColorCode:          "92",
		CompactLabel:       "K",
		Fetch:              FetchKimiUsage,
		DescribeCredential: describeKimiCredential,
		AccountKind:        AccountKindHome,
		FetchForAccount: func(ctx context.Context, account Account) UsageResult {
			return fetchKimiUsageForHome(ctx, account.Home, AccountProviderName(account.Provider, account.Name))
		},
		DescribeAccountCredential: func(account Account) CredentialStatus {
			return describeKimiCredentialForHome(account.Home)
		},
	},
	{
		Name:               "zai",
		Aliases:            []string{"zhipu"},
		MatchSubstrings:    []string{"zai", "glm", "zhipu", "bigmodel"},
		ColorCode:          "91",
		CompactLabel:       "Z",
		Standalone:         true,
		Fetch:              FetchZaiUsage,
		DescribeCredential: describeZaiCredential,
	},
	{
		Name:               "qwen",
		MatchSubstrings:    []string{"qwen"},
		ColorCode:          "94",
		CompactLabel:       "Q",
		Fetch:              FetchQwenUsage,
		DescribeCredential: describeQwenCredential,
		AccountKind:        AccountKindHome,
		FetchForAccount: func(ctx context.Context, account Account) UsageResult {
			return fetchQwenUsageForHome(ctx, account.Home, AccountProviderName(account.Provider, account.Name))
		},
		DescribeAccountCredential: func(account Account) CredentialStatus {
			return describeQwenCredentialForHome(account.Home)
		},
	},
	{
		Name:               "deepseek",
		MatchSubstrings:    []string{"deepseek"},
		ColorCode:          "95",
		CompactLabel:       "S",
		Standalone:         true,
		Fetch:              FetchDeepseekUsage,
		DescribeCredential: describeDeepseekCredential,
	},
	{
		Name:               "openrouter",
		MatchSubstrings:    []string{"openrouter"},
		ColorCode:          "94",
		CompactLabel:       "N",
		Standalone:         true,
		Fetch:              FetchOpenRouterUsage,
		DescribeCredential: describeOpenRouterCredential,
	},
	{
		Name:               "minimax",
		Aliases:            []string{"mmx"},
		MatchSubstrings:    []string{"minimax"},
		ColorCode:          "93",
		CompactLabel:       "M",
		Fetch:              FetchMinimaxUsage,
		DescribeCredential: describeMinimaxCredential,
	},
	{
		Name:               "grok",
		MatchSubstrings:    []string{"grok", "xai", "supergrok"},
		ColorCode:          "95",
		CompactLabel:       "V",
		Standalone:         true,
		Fetch:              FetchGrokUsage,
		DescribeCredential: describeGrokCredential,
		AccountKind:        AccountKindHome,
		FetchForAccount: func(ctx context.Context, account Account) UsageResult {
			return fetchGrokUsageForHome(ctx, account.Home, AccountProviderName(account.Provider, account.Name))
		},
		DescribeAccountCredential: func(account Account) CredentialStatus {
			return describeGrokCredentialForHome(account.Home)
		},
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

// CanonicalProviderName resolves a raw provider reference (exact registry
// name, alias, or legacy tool name) to the canonical name fetchers report in
// UsageResult.Provider. Config surfaces (alert thresholds, snoozes) must
// store and look up that name so keys match regardless of which alias the
// user typed.
func CanonicalProviderName(name string) (string, bool) {
	p := strings.ToLower(strings.TrimSpace(name))
	if p == "" {
		return "", false
	}
	if resolved, ok := lookupProviderName(p); ok {
		return resolved, true
	}
	// <provider>:<name> account rows are their own canonical key, but only
	// while the account is actually configured, so typo'd names fail
	// validation instead of silently storing thresholds nothing will ever
	// report.
	if provider, alias, ok := SplitAccountProviderLabel(p); ok {
		for _, account := range AccountsForProvider(provider) {
			if account.Name == alias {
				return p, true
			}
		}
	}
	// Legacy config keys speak tool names ("claude-code", "cursor") rather
	// than registry names ("claude", "cursor-agent").
	if normalized := update.NormalizeToolName(p); normalized != p {
		if resolved, ok := lookupProviderName(normalized); ok {
			return resolved, true
		}
	}
	return "", false
}

func lookupProviderName(p string) (string, bool) {
	for _, entry := range providers {
		if entry.Name == p {
			return entry.Name, true
		}
		for _, alias := range entry.Aliases {
			if alias == p {
				return entry.Name, true
			}
		}
	}
	return "", false
}

// AlertProviderNames lists provider names usable as alert-threshold keys:
// every installable provider (fetched by default) plus standalone providers
// the user explicitly opted into. Names are canonical registry keys, matching
// UsageResult.Provider values.
func AlertProviderNames() []string {
	names := make([]string, 0, len(providers))
	seen := make(map[string]bool, len(providers))
	for _, p := range providers {
		if !p.Standalone {
			seen[p.Name] = true
			names = append(names, p.Name)
		}
	}
	source := viper.GetStringSlice("enabled_tools")
	if len(source) == 0 {
		source = viper.GetStringSlice("agent_order")
	}
	for _, entry := range source {
		for _, part := range strings.Split(strings.ToLower(entry), ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if p, ok := LookupStandalone(part); ok && !seen[p.Name] {
				seen[p.Name] = true
				names = append(names, p.Name)
			}
		}
	}
	// <provider>:<name> account rows are alertable once configured; their
	// UsageResult.Provider values are the keys thresholds are stored under.
	for _, p := range accountProviders() {
		if !seen[p.Name] {
			seen[p.Name] = true
			names = append(names, p.Name)
		}
	}
	sort.Strings(names)
	return names
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
