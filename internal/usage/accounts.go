package usage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/viper"
)

// AccountKind selects which credential override a per-account row supports.
// AccountKindHome accounts read a whole CODEX_HOME-style directory (auth
// files plus local logs); token-kind accounts (a key or env-var reference)
// are reserved for the key-based providers and not implemented yet.
type AccountKind int

const (
	AccountKindNone AccountKind = iota
	AccountKindHome
)

// Account is one configured account with its home resolved to an absolute
// directory.
type Account struct {
	// Provider is the canonical registry name the account belongs to.
	Provider string
	// Name is the user-facing alias (lowercase, [a-z0-9_-]).
	Name string
	// Home is the account's CODEX_HOME-equivalent directory (~ expanded).
	Home string
}

// AccountEntry is one accounts-config entry as written: Home is the
// unresolved path string (may start with ~), Key/Env are reserved for
// token-kind accounts. Management commands round-trip these; the fetch path
// resolves through AccountsForProvider.
type AccountEntry struct {
	Provider string
	Name     string
	Home     string
	Key      string
	Env      string
}

// accountAliasPattern constrains the <provider>:<name> suffix. The colon
// prefix keeps account rows out of every registry-name/alias/tool-name
// namespace, so a badly chosen alias can never shadow another provider. '*'
// is allowed (never first) so email-derived aliases like "han***o36" — the
// masked form of an email prefix — are valid; aliases never become path
// segments, so the glob character is safe there.
var accountAliasPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9*\-_.]{0,31}$`)

// ValidateAccountAlias reports whether alias is usable as the <provider>:<name>
// suffix. Config surfaces call this before writing; the fetch path only skips.
func ValidateAccountAlias(alias string) error {
	alias = strings.ToLower(strings.TrimSpace(alias))
	if !accountAliasPattern.MatchString(alias) {
		return fmt.Errorf("invalid account name %q: use 1-32 lowercase letters, digits, '-' or '_' (must start alphanumeric)", alias)
	}
	return nil
}

// AccountProviderName is the usage-row provider label for one account.
func AccountProviderName(provider, alias string) string {
	return strings.ToLower(strings.TrimSpace(provider)) + ":" + strings.ToLower(strings.TrimSpace(alias))
}

// SplitAccountProviderLabel parses a "<provider>:<name>" row label back into
// its parts. The prefix must be a canonical registry name that supports
// accounts, so arbitrary "word:word" strings never match. ok is false for
// anything else (including plain provider names).
func SplitAccountProviderLabel(label string) (provider, alias string, ok bool) {
	l := strings.ToLower(strings.TrimSpace(label))
	prefix, rest, found := strings.Cut(l, ":")
	if !found || prefix == "" || rest == "" {
		return "", "", false
	}
	for _, entry := range providers {
		if entry.Name == prefix {
			return prefix, rest, true
		}
	}
	return "", "", false
}

// ProviderSupportsAccounts reports whether a canonical registry provider
// supports per-account usage rows.
func ProviderSupportsAccounts(provider string) bool {
	for _, entry := range providers {
		if entry.Name == strings.ToLower(strings.TrimSpace(provider)) {
			return entry.AccountKind != AccountKindNone && entry.FetchForAccount != nil
		}
	}
	return false
}

// ResolveAccountHomePath resolves an accounts-config home value the same way
// the fetch path does: ~ expands to the user home, relative paths anchor
// there too.
func ResolveAccountHomePath(home string) string {
	return resolveAccountHome(home)
}

// expandHomePath resolves a leading ~ against the user's home directory.
func expandHomePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}

// resolveAccountHome turns a config home value into the directory the
// account fetchers read: ~ expands to the user home, and a relative path is
// anchored there too so behavior never depends on oct's working directory.
func resolveAccountHome(home string) string {
	expanded := expandHomePath(home)
	if filepath.IsAbs(expanded) {
		return filepath.Clean(expanded)
	}
	return filepath.Join(credentialHomePath(), expanded)
}

// AccountConfigEntries lists the valid accounts entries across all providers
// (provider names sorted, entries in config order), homes unresolved. The
// config schema is:
//
//	accounts:
//	  codex:
//	    - name: work
//	      home: ~/.codex-work
//	  kimi:
//	    - name: personal
//	      home: ~/.kimi-code-personal
//
// A per-provider shorthand map form is also accepted (iterated in sorted-name
// order):
//
//	accounts:
//	  codex:
//	    work: ~/.codex-work
//
// The legacy top-level `codex_accounts` key (list or shorthand form) is
// merged in as the codex provider for backward compatibility; accounts-key
// entries win on duplicate names. Entries with an invalid name, unknown
// provider, or missing home are skipped. Config written through
// `oct config account` is pre-validated, so skipping only ever drops
// hand-edited mistakes.
func AccountConfigEntries() []AccountEntry {
	var entries []AccountEntry
	upsertEntry := func(e AccountEntry) {
		e.Provider = strings.ToLower(strings.TrimSpace(e.Provider))
		e.Name = strings.ToLower(strings.TrimSpace(e.Name))
		if !accountAliasPattern.MatchString(e.Name) || strings.TrimSpace(e.Home) == "" {
			return
		}
		if !ProviderSupportsAccounts(e.Provider) {
			return
		}
		for i, existing := range entries {
			if existing.Provider == e.Provider && existing.Name == e.Name {
				entries[i] = e
				return
			}
		}
		entries = append(entries, e)
	}
	// Field lookups differ between config sources: config files decode to
	// map[string]interface{}, while values written through viper.Set (the
	// `oct config account` commands) keep concrete Go types.
	appendFromMap := func(provider string, item interface{}) {
		switch m := item.(type) {
		case map[string]interface{}:
			name, _ := m["name"].(string)
			home, _ := m["home"].(string)
			key, _ := m["key"].(string)
			env, _ := m["env"].(string)
			upsertEntry(AccountEntry{Provider: provider, Name: name, Home: home, Key: key, Env: env})
		case map[string]string:
			upsertEntry(AccountEntry{Provider: provider, Name: m["name"], Home: m["home"], Key: m["key"], Env: m["env"]})
		}
	}
	appendList := func(provider string, list interface{}) {
		switch parsed := list.(type) {
		case []interface{}:
			for _, item := range parsed {
				appendFromMap(provider, item)
			}
		case []map[string]interface{}:
			for _, m := range parsed {
				appendFromMap(provider, m)
			}
		case []map[string]string:
			for _, m := range parsed {
				appendFromMap(provider, m)
			}
		case map[string]interface{}:
			// Go map iteration is randomized; sort aliases so the shorthand
			// form yields a deterministic row order too.
			names := make([]string, 0, len(parsed))
			for name := range parsed {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				home, _ := parsed[name].(string)
				upsertEntry(AccountEntry{Provider: provider, Name: name, Home: home})
			}
		}
	}

	// Legacy key first so accounts-key entries override on name conflicts.
	if legacy := viper.Get("codex_accounts"); legacy != nil {
		appendList("codex", legacy)
	}
	if raw := viper.Get("accounts"); raw != nil {
		// Normalize the provider map: config files decode to
		// map[string]interface{}, while values written through viper.Set (the
		// `oct config account` commands) keep concrete Go types.
		normalized := map[string]interface{}{}
		switch parsed := raw.(type) {
		case map[string]interface{}:
			normalized = parsed
		case map[string][]interface{}:
			for provider, list := range parsed {
				normalized[provider] = list
			}
		case map[string][]map[string]interface{}:
			for provider, list := range parsed {
				normalized[provider] = list
			}
		case map[string][]map[string]string:
			for provider, list := range parsed {
				normalized[provider] = list
			}
		}
		providerNames := make([]string, 0, len(normalized))
		for provider := range normalized {
			providerNames = append(providerNames, provider)
		}
		sort.Strings(providerNames)
		for _, provider := range providerNames {
			appendList(provider, normalized[provider])
		}
	}
	return entries
}

// AccountsForProvider resolves the configured accounts of one provider to
// fetchable directories, in config order. Unknown or unsupported providers
// yield nil.
func AccountsForProvider(provider string) []Account {
	provider = strings.ToLower(strings.TrimSpace(provider))
	var accounts []Account
	for _, entry := range AccountConfigEntries() {
		if entry.Provider != provider {
			continue
		}
		if entry.Key != "" || entry.Env != "" {
			// Token-kind entries are reserved but not implemented yet.
			continue
		}
		accounts = append(accounts, Account{Provider: entry.Provider, Name: entry.Name, Home: resolveAccountHome(entry.Home)})
	}
	return accounts
}

// AllAccounts lists every configured account, providers in sorted order then
// config order.
func AllAccounts() []Account {
	var accounts []Account
	for _, entry := range AccountConfigEntries() {
		if entry.Key != "" || entry.Env != "" {
			continue
		}
		accounts = append(accounts, Account{Provider: entry.Provider, Name: entry.Name, Home: resolveAccountHome(entry.Home)})
	}
	return accounts
}

// accountProviders builds the synthetic registry entries behind the
// <provider>:<name> usage rows: one entry per configured account, in
// registry provider order then config order. They deliberately stay out of
// the static providers slice: they exist only while the user configures
// them, and everything display-related (color, icon, compact letter) derives
// from the base provider via the shared substring match.
func accountProviders() []Provider {
	var out []Provider
	for _, entry := range providers {
		if entry.AccountKind == AccountKindNone || entry.FetchForAccount == nil {
			continue
		}
		for _, account := range AccountsForProvider(entry.Name) {
			account := account
			base := entry
			out = append(out, Provider{
				Name:            AccountProviderName(account.Provider, account.Name),
				MatchSubstrings: []string{entry.Name},
				Fetch: func(ctx context.Context) UsageResult {
					return base.FetchForAccount(ctx, account)
				},
				DescribeCredential: func() CredentialStatus {
					if base.DescribeAccountCredential == nil {
						return CredentialStatus{
							Status: CredentialStatusInfo,
							Note:   "no credential probe registered",
						}
					}
					return base.DescribeAccountCredential(account)
				},
			})
		}
	}
	return out
}
