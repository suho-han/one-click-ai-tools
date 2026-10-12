package usage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Antigravity usage comes from `agy --print /usage`, which boots a full CLI
// session (language server, loadCodeAssist, keychain token refresh) on every
// call, and the menubar refreshes every minute. Two guards keep that poll
// from destabilizing the agy credentials it depends on:
//
//   - a minimum re-fetch interval: a real `agy` run happens at most once per
//     antigravityMinFetchInterval, with the last good numbers served from
//     this cache in between (each run races the VS Code extension's
//     `agy --hub` process for OAuth token refreshes, and losing that race
//     is how the stored credentials end up inconsistent).
//   - an auth-failure backoff: when the run fails with auth-indicative
//     output, polling is paused for antigravityAuthBackoff instead of
//     re-triggering the failing login path every refresh. The user is told
//     to re-authenticate once in a terminal.
//
// The file follows the Claude usage cache pattern (claude_usage_cache.go).

const (
	antigravityMinFetchInterval = 5 * time.Minute
	antigravityAuthBackoff      = 30 * time.Minute
	// antigravityCacheMaxAge caps how long a cached fetch is served; older
	// entries fall through to the warn state rather than showing stale data.
	antigravityCacheMaxAge = 6 * time.Hour
)

type antigravityUsageCacheEntry struct {
	FetchedAtMs    int64             `json:"fetchedAtMs"`
	BackoffUntilMs int64             `json:"backoffUntilMs,omitempty"`
	BackoffReason  string            `json:"backoffReason,omitempty"`
	Buckets        map[string]string `json:"buckets,omitempty"`
	BucketResets   map[string]string `json:"bucket_resets,omitempty"`
}

// antigravityCachePath is a variable so tests can redirect the cache file.
var antigravityCachePath = defaultAntigravityCachePath

func defaultAntigravityCachePath() string {
	base, err := os.UserCacheDir()
	if err != nil || strings.TrimSpace(base) == "" {
		if home, homeErr := os.UserHomeDir(); homeErr == nil && strings.TrimSpace(home) != "" {
			base = filepath.Join(home, ".cache")
		}
	}
	return filepath.Join(base, "one-click-tools", "antigravity-usage.json")
}

func readAntigravityUsageCache() antigravityUsageCacheEntry {
	var entry antigravityUsageCacheEntry
	data, err := os.ReadFile(antigravityCachePath())
	if err != nil {
		return entry
	}
	// Best-effort: a corrupt or partially written cache behaves like a
	// missing one.
	_ = json.Unmarshal(data, &entry)
	return entry
}

func writeAntigravityUsageCache(entry antigravityUsageCacheEntry) {
	path := antigravityCachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}

func saveAntigravityLastGoodUsage(buckets, resets map[string]string, now time.Time) {
	if len(buckets) == 0 {
		return
	}
	// A success also clears any pending auth backoff: the token evidently
	// works again (e.g. the user re-authenticated in a terminal).
	writeAntigravityUsageCache(antigravityUsageCacheEntry{
		FetchedAtMs:  now.UnixMilli(),
		Buckets:      buckets,
		BucketResets: resets,
	})
}

// antigravityFetchHold reports whether a real `agy` run should be skipped
// right now, and why. Auth backoff wins over the min-interval because it
// says why the previous run failed, not just when the last one happened.
func antigravityFetchHold(now time.Time) (string, bool) {
	entry := readAntigravityUsageCache()
	if entry.BackoffUntilMs != 0 {
		if remaining := time.Until(time.UnixMilli(entry.BackoffUntilMs)); remaining > 0 {
			return fmt.Sprintf("Antigravity auth check paused after a failed login; retrying in %d min (run `agy` once in a terminal to re-authenticate)",
				int(remaining.Minutes())+1), true
		}
	}
	if entry.FetchedAtMs != 0 {
		elapsed := now.Sub(time.UnixMilli(entry.FetchedAtMs))
		if elapsed < antigravityMinFetchInterval {
			return fmt.Sprintf("last refresh %s ago (min interval %s)",
				elapsed.Round(time.Second), antigravityMinFetchInterval), true
		}
	}
	return "", false
}

// serveAntigravityCachedUsage turns the last good buckets into a result while
// the live fetch is on hold. Mirrors lastGoodClaudeUsage.
func serveAntigravityCachedUsage(base UsageResult, now time.Time, holdReason string) (UsageResult, bool) {
	entry := readAntigravityUsageCache()
	if len(entry.Buckets) == 0 {
		return base, false
	}
	fetchedAt := time.UnixMilli(entry.FetchedAtMs)
	if now.Sub(fetchedAt) > antigravityCacheMaxAge {
		return base, false
	}

	result := base
	result.Status = "ok"
	result.Source = "cache"
	result.Unit = "percent"
	result.Limit = "100"
	result.Buckets = entry.Buckets
	result.BucketResets = entry.BucketResets
	result.Used = maxAntigravityBucketUsed(entry.Buckets)
	result.Message = fmt.Sprintf("Cached Antigravity usage from %s (%s)", fetchedAt.Format("15:04"), holdReason)
	return result, true
}

func maxAntigravityBucketUsed(buckets map[string]string) string {
	maxUsed := 0.0
	for _, value := range buckets {
		used, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err == nil && used > maxUsed {
			maxUsed = used
		}
	}
	return fmt.Sprintf("%.1f", maxUsed)
}

// markAntigravityAuthFailure records the failed login and schedules the
// backoff window, keeping the last-good buckets for later runs to serve.
func markAntigravityAuthFailure(now time.Time, detail string) {
	entry := readAntigravityUsageCache()
	entry.BackoffUntilMs = now.Add(antigravityAuthBackoff).UnixMilli()
	entry.BackoffReason = detail
	if entry.FetchedAtMs == 0 {
		entry.FetchedAtMs = now.UnixMilli()
	}
	writeAntigravityUsageCache(entry)
}

// antigravityAuthFailureMarkers are lowercase fragments that identify an
// auth-problem run, based on agy's own diagnostics (printmode silent auth,
// keyring auth, the browser login fallback).
var antigravityAuthFailureMarkers = []string{
	"not logged in",
	"silent auth failed",
	"cannot complete interactive login",
	"no controlling terminal",
	"authentication failed",
	"login required",
	"failed to open browser",
	"could not open browser automatically",
	"oauth-callback",
}

// isAntigravityAuthFailure classifies an `agy --print /usage` run as an
// auth failure when its combined output carries an auth-problem marker.
func isAntigravityAuthFailure(output string, err error) bool {
	combined := strings.ToLower(output)
	if err != nil {
		combined += "\n" + strings.ToLower(err.Error())
	}
	for _, marker := range antigravityAuthFailureMarkers {
		if strings.Contains(combined, marker) {
			return true
		}
	}
	return false
}

// antigravityAuthFailureDetail condenses the failing run's output into a
// short reason for the backoff record and user-facing message.
func antigravityAuthFailureDetail(output string, err error) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lowered := strings.ToLower(line)
		for _, marker := range antigravityAuthFailureMarkers {
			if strings.Contains(lowered, marker) {
				if len(line) > 120 {
					return line[:120]
				}
				return line
			}
		}
	}
	if err != nil {
		message := err.Error()
		if len(message) > 120 {
			return message[:120]
		}
		return message
	}
	return "authentication failed"
}
