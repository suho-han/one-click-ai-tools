package usage

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/suho-han/one-click-ai-tools/internal/fsutil"
)

// The OAuth snapshot is shared by oct processes. A short freshness window
// coalesces polls; the last confirmed credits retain their own timestamp.

const (
	// claudeUsageCacheMaxAge caps how long a cached fetch is served; older
	// entries fall through to the warn state rather than showing stale data.
	claudeUsageCacheMaxAge = 6 * time.Hour
	claudeUsageFreshFor    = 2 * time.Minute
	// claudeRateLimitBackoffDefault is applied when the 429 carries no
	// Retry-After header.
	claudeRateLimitBackoffDefault = 4 * time.Minute
)

type claudeUsageCacheEntry struct {
	FetchedAtMs    int64               `json:"fetchedAtMs"`
	Credential     [sha256.Size]byte   `json:"credential"`
	Used           string              `json:"used,omitempty"`
	BackoffUntilMs int64               `json:"backoffUntilMs,omitempty"`
	Buckets        map[string]string   `json:"buckets,omitempty"`
	BucketResets   map[string]string   `json:"bucket_resets,omitempty"`
	ResetCredits   *int                `json:"reset_credits,omitempty"`
	ResetDetails   []ResetCreditDetail `json:"reset_credit_details,omitempty"`
	ResetCheckedMs int64               `json:"reset_checked_ms,omitempty"`
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
	// Older oct builds write only quota and backoff to claude-usage.json.
	return filepath.Join(base, "one-click-tools", "claude-oauth-usage.json")
}

func claudeCredentialCachePath(credential [sha256.Size]byte) string {
	return fmt.Sprintf("%s.%x", claudeUsageCachePath(), credential)
}

func readClaudeUsageCache(credential [sha256.Size]byte) claudeUsageCacheEntry {
	entry := claudeUsageCacheEntry{Credential: credential}
	data, err := os.ReadFile(claudeCredentialCachePath(credential))
	if os.IsNotExist(err) {
		// Carry over a previous snapshot only when it belongs to this token.
		data, err = os.ReadFile(claudeUsageCachePath())
	}
	if err == nil && json.Unmarshal(data, &entry) == nil && entry.Credential == credential {
		return entry
	}
	return claudeUsageCacheEntry{Credential: credential}
}

func writeClaudeUsageCache(entry claudeUsageCacheEntry) {
	path := claudeCredentialCachePath(entry.Credential)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	// Atomic write: a crash mid-write must not lose the recorded
	// state (for claude, the 429 backoff deadline) or leave a truncated
	// cache that reads back as "no cache".
	_ = fsutil.WriteFileAtomic(path, data, 0o600)
}

// save and rateLimited are called while the fetch holds the sidecar lock.
func (entry *claudeUsageCacheEntry) save(result UsageResult, now time.Time) {
	entry.FetchedAtMs = now.UnixMilli()
	entry.BackoffUntilMs = 0
	entry.Used = result.Used
	entry.Buckets = result.Buckets
	entry.BucketResets = result.BucketResets
	if result.ResetCredits != nil {
		entry.ResetCredits = result.ResetCredits
		entry.ResetDetails = result.ResetCreditDetails
		entry.ResetCheckedMs = now.UnixMilli()
	}
	writeClaudeUsageCache(*entry)
}

func (entry *claudeUsageCacheEntry) rateLimited(now time.Time, retryAfter string) {
	header := strings.TrimSpace(retryAfter)
	entry.BackoffUntilMs = now.Add(claudeRateLimitBackoffDefault).UnixMilli()
	if seconds, err := strconv.ParseInt(header, 10, 64); err == nil && seconds > 0 {
		nowMs := now.UnixMilli()
		entry.BackoffUntilMs = nowMs + min(seconds, (math.MaxInt64-nowMs)/1000)*1000
	} else if until, err := http.ParseTime(header); err == nil && until.After(now) {
		entry.BackoffUntilMs = until.UnixMilli()
	}
	writeClaudeUsageCache(*entry)
}

func (entry claudeUsageCacheEntry) withCachedResets(result UsageResult, now time.Time) UsageResult {
	checkedAt := time.UnixMilli(entry.ResetCheckedMs)
	if entry.ResetCredits == nil || entry.ResetCheckedMs == 0 {
		return result
	}
	// Only unexpired grants count. The decoded slice is owned by this read.
	active := entry.ResetDetails[:0]
	count := 0
	for _, detail := range entry.ResetDetails {
		if expiry, ok := parseBucketResetTime(detail.ExpiresAt); ok && !expiry.After(now) {
			continue
		}
		active = append(active, detail)
		count += detail.Count
	}
	result.ResetCredits = &count
	result.ResetCreditDetails = active
	result.ResetCreditsCachedAt = checkedAt.UTC().Format(time.RFC3339)
	return result
}

func (entry claudeUsageCacheEntry) usage(base UsageResult, now time.Time, reason string) (UsageResult, bool) {
	fetchedAt := time.UnixMilli(entry.FetchedAtMs)
	if entry.Used == "" || now.Sub(fetchedAt) > claudeUsageCacheMaxAge {
		return base, false
	}
	base.Status = "ok"
	base.Source = "cache"
	base.Used = entry.Used
	base.Buckets = entry.Buckets
	base.BucketResets = entry.BucketResets
	base.Message = fmt.Sprintf("Cached Claude usage from %s (%s)", fetchedAt.Format("15:04"), reason)
	return entry.withCachedResets(base, now), true
}
