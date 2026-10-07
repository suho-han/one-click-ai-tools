package update

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// checkTimeout caps a whole version-check run (brew/npm outdated probes,
// per-package registry lookups). The menu bar app layers its own watchdog
// above this. Swappable in tests.
var checkTimeout = 45 * time.Second

// RunVersionCheck reports each tool's installed version next to the latest
// version its manager knows about, without updating anything. With JSON it
// emits the same NDJSON discipline as Run (check_start / tool_check /
// check_done); without JSON it prints one human-readable line per tool.
func RunVersionCheck(ctx context.Context, opts ...Options) error {
	if ctx == nil {
		ctx = context.Background()
	}
	config := Options{}
	if len(opts) > 0 {
		config = opts[0]
	}
	out := config.Output
	if out == nil {
		out = os.Stdout
	}
	emit := newEventEmitter(config.JSON, out)

	// The check mirrors Run's tool resolution — including --only bypassing
	// enabled_tools — but without an explicit selection it reports on the
	// whole registry: a disabled tool still deserves an update button.
	orderedTools := GetOrderedTools(viper.GetStringSlice("agent_order"))
	tools, err := resolveOnlyTools(config.Only, orderedTools)
	if err != nil {
		return err
	}
	if tools == nil {
		tools = orderedTools
	}
	if len(tools) == 0 {
		if config.JSON {
			emit.checkStart(0)
			emit.checkDone()
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	// Same memoized detection phase as Run: ExplainPlans shells out for
	// manager detection and installed versions, and the latest-version
	// probes below reuse the cached answers where they overlap.
	detectCtx := withProbeMemo(ctx)
	plans := ExplainPlans(detectCtx, tools)
	latest := collectLatestVersions(detectCtx, plans)

	emit.checkStart(len(plans))
	if !config.JSON && len(plans) > 0 {
		fmt.Fprintf(out, "Checking %d tools for updates...\n", len(plans))
	}
	for i, plan := range plans {
		latestVersion, latestKnown := latest[plan.Tool.BinaryName]
		outdated := versionIsNewer(plan.VersionBefore, latestVersion)
		emit.toolCheck(plan, latestVersion, latestKnown, outdated)
		if !config.JSON {
			fmt.Fprintf(out, "[%d/%d] %s\n", i+1, len(plans), formatCheckLine(plan, latestVersion, latestKnown, outdated))
		}
	}
	emit.checkDone()
	return nil
}

// collectLatestVersions returns, keyed by binary name, the newest version
// each tool's manager can report without running an installer. An empty
// value with a present key means the manager answered and the tool is up to
// date; an absent key means there is no query API (native updaters, install
// scripts) or the probe failed — only running the updater can tell.
func collectLatestVersions(ctx context.Context, plans []Plan) map[string]string {
	latest := make(map[string]string, len(plans))
	var brewMap map[string]string
	var npmMap map[string]string
	for _, plan := range plans {
		tool, manager := plan.Tool, plan.Manager
		switch manager {
		case Brew:
			if brewMap == nil {
				brewMap = brewOutdatedVersions(ctx)
			}
			if brewMap == nil {
				continue
			}
			// The probe answered: listed entries are outdated with their
			// available version, absence means current.
			latest[tool.BinaryName] = brewMap[strings.ToLower(tool.BrewTarget())]
		case Npm:
			if npmMap == nil {
				npmMap = npmOutdatedGlobalVersions(ctx)
			}
			if npmMap == nil {
				// `npm outdated -g` unavailable (old npm, launch failure):
				// ask the registry directly per package instead.
				if v := registryLatestVersion(ctx, manager, tool.Package); v != "" {
					latest[tool.BinaryName] = v
				}
				continue
			}
			latest[tool.BinaryName] = npmMap[tool.Package]
		case Pnpm, Yarn:
			// pnpm/yarn globals never appear in npm's outdated listing, but
			// the registry they read from is the same one npm queries.
			if v := registryLatestVersion(ctx, manager, tool.Package); v != "" {
				latest[tool.BinaryName] = v
			}
		}
	}
	return latest
}

type brewOutdatedEntry struct {
	Name           string `json:"name"`
	CurrentVersion string `json:"current_version"`
}

type brewOutdatedJSON struct {
	Formulae []brewOutdatedEntry `json:"formulae"`
	Casks    []brewOutdatedEntry `json:"casks"`
}

// brewOutdatedVersions runs `brew outdated --json=v2` once and maps each
// formula/cask name to its available version. A nil map means the probe
// could not run or parse — brew-managed tools then report latest unknown.
func brewOutdatedVersions(ctx context.Context) map[string]string {
	out, err := commandContextWithEnv(ctx, "brew", "outdated", "--json=v2").Output()
	if err != nil {
		return nil
	}
	var parsed brewOutdatedJSON
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil
	}
	versions := make(map[string]string, len(parsed.Formulae)+len(parsed.Casks))
	for _, entry := range parsed.Formulae {
		versions[strings.ToLower(entry.Name)] = entry.CurrentVersion
	}
	for _, entry := range parsed.Casks {
		versions[strings.ToLower(entry.Name)] = entry.CurrentVersion
	}
	return versions
}

type npmOutdatedEntry struct {
	Latest string `json:"latest"`
}

// npmOutdatedGlobalVersions runs `npm outdated --global --json` once and maps
// package name to its latest version. npm exits 1 when anything is outdated,
// so the exit code alone is not an error; a nil map means the probe could not
// run or parse and callers fall back to per-package lookups.
func npmOutdatedGlobalVersions(ctx context.Context) map[string]string {
	out, err := commandContextWithEnv(ctx, "npm", "outdated", "--global", "--json").Output()
	if out == nil && err != nil {
		return nil
	}
	var parsed map[string]npmOutdatedEntry
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil
	}
	versions := make(map[string]string, len(parsed))
	for pkg, entry := range parsed {
		versions[pkg] = entry.Latest
	}
	return versions
}

// registryLatestVersion asks the package registry (shared by npm, pnpm and
// yarn) for a single package's latest version. Used when the one-shot
// `npm outdated` probe is unavailable — pnpm/yarn-managed tools, or a failed
// global outdated — since the registry answer is the same for all three.
func registryLatestVersion(ctx context.Context, manager Manager, pkg string) string {
	if strings.TrimSpace(pkg) == "" {
		return ""
	}
	probe, args := "npm", []string{"view", pkg, "version"}
	switch manager {
	case Pnpm:
		probe, args = "pnpm", []string{"view", pkg, "version"}
	case Yarn:
		probe, args = "yarn", []string{"info", pkg, "version"}
	}
	if manager.missingBinary() != "" {
		return ""
	}
	out, err := commandContextWithEnv(ctx, probe, args...).Output()
	if err != nil {
		return ""
	}
	return firstNonEmptyLine(string(out))
}

// versionIsNewer reports whether latest is a strictly newer release than
// installed. Both sides may carry cosmetic prefixes (v) or suffixes (build
// metadata); the comparison is numeric on the dot-separated components and
// gives up (reporting "not newer") when a component is not numeric — a
// conservative answer that only ever hides an update button, never fakes one.
func versionIsNewer(installed, latest string) bool {
	installed = strings.TrimPrefix(strings.TrimSpace(installed), "v")
	latest = strings.TrimPrefix(strings.TrimSpace(latest), "v")
	if installed == "" || latest == "" || installed == latest {
		return false
	}
	installedParts := strings.SplitN(installed, ".", 3)
	latestParts := strings.SplitN(latest, ".", 3)
	for i := 0; i < len(installedParts) && i < len(latestParts); i++ {
		a, aOK := numericPrefix(installedParts[i])
		b, bOK := numericPrefix(latestParts[i])
		if !aOK || !bOK {
			return false
		}
		if a != b {
			return a < b
		}
	}
	return len(installedParts) < len(latestParts)
}

// numericPrefix parses the leading digits of s ("3-beta.1" -> 3, true).
func numericPrefix(s string) (int, bool) {
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0, false
	}
	n, err := strconv.Atoi(s[:end])
	if err != nil {
		return 0, false
	}
	return n, true
}

func formatCheckLine(plan Plan, latest string, latestKnown, outdated bool) string {
	name := plan.Tool.Colorize(plan.Tool.Name)
	switch {
	case plan.VersionBefore == "":
		return fmt.Sprintf("%s: not installed", name)
	case !latestKnown:
		return fmt.Sprintf("%s: %s (latest unknown — run the update to check)", name, plan.VersionBefore)
	case outdated:
		return fmt.Sprintf("%s: %s -> %s (update available)", name, plan.VersionBefore, latest)
	default:
		return fmt.Sprintf("%s: %s (latest)", name, plan.VersionBefore)
	}
}
