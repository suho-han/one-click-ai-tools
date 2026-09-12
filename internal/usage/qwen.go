package usage

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Qwen Code usage. Qwen has no public quota API (the CLI itself learns about
// limits only from 429 responses), so oct reports a local estimate: it counts
// today's records in the CLI's token-usage JSONL logs
// (<QWEN_HOME|~/.qwen>/.../usage/token-usage-YYYY-MM.jsonl, one record per API
// call) against the qwen_daily_limit config (default 100, the post-2026-04
// OAuth free tier). The estimate covers this machine only.
const qwenUsageFilePattern = "token-usage-*.jsonl"

// qwenUsageDirDepth caps how deep under the Qwen home the walk descends, so a
// bloated session directory cannot turn into an unbounded scan.
const qwenUsageDirDepth = 4

// resolveQwenHome returns the Qwen Code data directory ($QWEN_HOME or
// ~/.qwen), matching the CLI's own resolution.
func resolveQwenHome() string {
	if home := os.Getenv("QWEN_HOME"); home != "" {
		return home
	}
	home, _ := userHomeDir()
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".qwen")
}

// qwenUsageRecord is the subset of a token-usage JSONL line oct reads. Field
// names other than apiDurationMs are read defensively: the CLI's design doc
// promises a local date, auth type, and token counters but only pins the
// duration key.
type qwenUsageRecord struct {
	Date      string          `json:"date"`
	LocalDate string          `json:"localDate"`
	AuthType  string          `json:"authType"`
	Model     string          `json:"model"`
	Timestamp json.RawMessage `json:"timestamp"`
}

// qwenTimestampString decodes the record's raw timestamp JSON (a number or a
// quoted string) to its plain value, so downstream parsers never see the JSON
// quotes.
func qwenTimestampString(rec qwenUsageRecord) string {
	raw := strings.TrimSpace(string(rec.Timestamp))
	if raw == "" {
		return ""
	}
	var s string
	if err := json.Unmarshal([]byte(raw), &s); err == nil {
		return strings.TrimSpace(s)
	}
	return raw
}

// qwenRecordDate extracts the record's local date (YYYY-MM-DD), preferring the
// explicit date fields and falling back to a unix timestamp.
func qwenRecordDate(rec qwenUsageRecord) string {
	if d := strings.TrimSpace(rec.Date); d != "" {
		return d
	}
	if d := strings.TrimSpace(rec.LocalDate); d != "" {
		return d
	}
	s := qwenTimestampString(rec)
	if s == "" {
		return ""
	}
	if sec, err := strconv.ParseInt(s, 10, 64); err == nil {
		if sec > 10_000_000_000 { // milliseconds
			return time.UnixMilli(sec).Format("2006-01-02")
		}
		return time.Unix(sec, 0).Format("2006-01-02")
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Local().Format("2006-01-02")
	}
	return ""
}

// qwenPlanLabel turns a CLI auth type into a human plan label.
func qwenPlanLabel(authType string) string {
	t := strings.ToLower(authType)
	switch {
	case t == "":
		return ""
	case strings.Contains(t, "oauth"):
		return "OAuth"
	case strings.Contains(t, "key"):
		return "API Key"
	default:
		return authType
	}
}

// collectQwenUsage walks the Qwen home for token-usage JSONL files and returns
// (today's request count, plan label from the newest record, whether any
// records exist at all).
func collectQwenUsage(qwenHome string, now time.Time) (int, string, bool) {
	today := now.Format("2006-01-02")
	count := 0
	plan := ""
	anyRecord := false

	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable subtree: skip, a usage estimate must not fail hard
		}
		if d.IsDir() {
			rel, relErr := filepath.Rel(qwenHome, path)
			if relErr == nil && rel != "." && strings.Count(rel, string(filepath.Separator)) >= qwenUsageDirDepth {
				return fs.SkipDir
			}
			return nil
		}
		matched, matchErr := filepath.Match(qwenUsageFilePattern, d.Name())
		if matchErr != nil || !matched {
			return nil
		}
		f, openErr := os.Open(path)
		if openErr != nil {
			return nil
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var rec qwenUsageRecord
			if json.Unmarshal([]byte(line), &rec) != nil {
				continue
			}
			anyRecord = true
			if plan == "" {
				plan = qwenPlanLabel(rec.AuthType)
			}
			if qwenRecordDate(rec) == today {
				count++
			}
		}
		return nil
	}

	_ = filepath.WalkDir(qwenHome, walkFn)
	return count, plan, anyRecord
}

// FetchQwenUsage reports a local daily request estimate for Qwen Code.
func FetchQwenUsage(ctx context.Context) UsageResult {
	qwenHome := resolveQwenHome()
	result := UsageResult{
		Provider: "qwen",
		Period:   "1d",
		Unit:     "req",
		Source:   "local",
		Status:   "warn",
		Message:  "No data: Qwen Code usage records not found",
	}
	if qwenHome == "" {
		result.Message = "No data: Qwen Code data directory not found"
		return result
	}

	limit := viper.GetInt("qwen_daily_limit")
	if limit <= 0 {
		limit = 100
	}
	result.Limit = strconv.Itoa(limit)

	count, plan, anyRecord := collectQwenUsage(qwenHome, time.Now())
	if !anyRecord {
		result.SourceDetail = fmt.Sprintf("qwen_home=%s", qwenHome)
		if osDebugEnabled() {
			result.SourceDetail += " (no token-usage files matched)"
		}
		return result
	}

	result.Plan = plan
	if plan != "" {
		result.PlanSource = "local_auth"
	}
	result.Used = strconv.Itoa(count)
	result.Status = "ok"
	result.Message = "Local estimate from Qwen Code token-usage records (this machine only)"
	if limit > 0 && count >= limit {
		result.Status = "warn"
		result.Message += "; daily limit likely reached"
	}
	if osDebugEnabled() {
		result.SourceDetail = fmt.Sprintf("qwen_home=%s limit=%d", qwenHome, limit)
	}

	return result
}
