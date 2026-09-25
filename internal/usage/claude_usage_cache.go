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

// Claude's OAuth usage endpoint answers 429 when polled too aggressively
// (the menubar refreshes every minute), and the ~/.claude.json fallback only
// works when the Claude CLI itself happened to fetch usage recently. This
// file is oct's own last-good cache: successful OAuth fetches are recorded
// here so a 429 can still show real numbers, and the recorded 429 time
// backs the client off instead of hammering the endpoint every refresh.

const (
	// claudeUsageCacheMaxAge caps how long a cached fetch is served; older
	// entries fall through to the warn state rather than showing stale data.
	claudeUsageCacheMaxAge = 6 * time.Hour
	// claudeRateLimitBackoffDefault is applied when the 429 carries no
	// Retry-After header.
	claudeRateLimitBackoffDefault = 4 * time.Minute
	claudeRateLimitBackoffMax     = 30 * time.Minute
)

type claudeUsageCacheEntry struct {
	FetchedAtMs    int64             `json:"fetchedAtMs"`
	BackoffUntilMs int64             `json:"backoffUntilMs,omitempty"`
	Buckets        map[string]string `json:"buckets,omitempty"`
	BucketResets   map[string]string `json:"bucket_resets,omitempty"`
}

// claudeUsageCachePath is a variable so tests can redirect the cache file.
var claudeUsageCachePath = defaultClaudeUsageCachePath

func defaultClaudeUsageCachePath() string {
	base, err := os.UserCacheDir()
	if err != nil || strings.TrimSpace(base) == "" {
		if home, homeErr := os.UserHomeDir(); homeErr == nil && strings.TrimSpace(home) != "" {
			base = filepath.Join(home, ".cache")
		}
	}
	return filepath.Join(base, "one-click-tools", "claude-usage.json")
}

func readClaudeUsageCache() claudeUsageCacheEntry {
	var entry claudeUsageCacheEntry
	data, err := os.ReadFile(claudeUsageCachePath())
	if err != nil {
		return entry
	}
	// Best-effort: a corrupt or partially written cache behaves like a
	// missing one.
	_ = json.Unmarshal(data, &entry)
	return entry
}

func writeClaudeUsageCache(entry claudeUsageCacheEntry) {
	path := claudeUsageCachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}

// saveClaudeLastGoodUsage records a successful fetch and clears any pending
// rate-limit backoff. Empty fetches are not worth caching.
func saveClaudeLastGoodUsage(buckets, resets map[string]string, now time.Time) {
	if len(buckets) == 0 {
		return
	}
	writeClaudeUsageCache(claudeUsageCacheEntry{
		FetchedAtMs:  now.UnixMilli(),
		Buckets:      buckets,
		BucketResets: resets,
	})
}

// markClaudeRateLimited records the 429 and schedules a backoff window,
// keeping the last-good buckets so later runs can still serve them.
func markClaudeRateLimited(now time.Time, retryAfter time.Duration) {
	backoff := retryAfter
	if backoff <= 0 {
		backoff = claudeRateLimitBackoffDefault
	}
	if backoff > claudeRateLimitBackoffMax {
		backoff = claudeRateLimitBackoffMax
	}
	entry := readClaudeUsageCache()
	entry.BackoffUntilMs = now.Add(backoff).UnixMilli()
	if entry.FetchedAtMs == 0 {
		entry.FetchedAtMs = now.UnixMilli()
	}
	writeClaudeUsageCache(entry)
}

// claudeUsageBackoffRemaining reports how much longer the usage endpoint
// should be left alone after a recent 429.
func claudeUsageBackoffRemaining(now time.Time) (time.Duration, bool) {
	entry := readClaudeUsageCache()
	if entry.BackoffUntilMs == 0 {
		return 0, false
	}
	remaining := time.Duration(entry.BackoffUntilMs-now.UnixMilli()) * time.Millisecond
	if remaining <= 0 {
		return 0, false
	}
	return remaining, true
}

// lastGoodClaudeUsage turns the cached buckets into a served result, mirroring
// fetchClaudeCachedUsage's shape (ok / source=cache). Falls back to the base
// result when nothing usable is cached.
func lastGoodClaudeUsage(base UsageResult, now time.Time, reason string) (UsageResult, bool) {
	entry := readClaudeUsageCache()
	if len(entry.Buckets) == 0 {
		return base, false
	}
	fetchedAt := time.UnixMilli(entry.FetchedAtMs)
	if now.Sub(fetchedAt) > claudeUsageCacheMaxAge {
		return base, false
	}

	result := base
	result.Status = "ok"
	result.Source = "cache"
	result.Unit = "percent"
	result.Limit = "100"
	result.Buckets = entry.Buckets
	result.BucketResets = entry.BucketResets
	if entry.Buckets["5h"] != "" {
		result.Used = entry.Buckets["5h"]
	} else if entry.Buckets["7d"] != "" {
		result.Used = entry.Buckets["7d"]
	} else {
		return base, false
	}

	result.Message = fmt.Sprintf("Cached Claude usage from %s (%s)", fetchedAt.Format("15:04"), reason)
	return result, true
}

// parseRetryAfter reads a seconds-based Retry-After header value; anything
// unparsable falls through to the default backoff.
func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}
