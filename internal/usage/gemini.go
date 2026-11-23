package usage

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/suho-han/one-click-ai-tools/internal/execenv"
)

func baseAntigravityUsageResult() UsageResult {
	return UsageResult{
		Provider:   "antigravity",
		Plan:       "unknown",
		PlanSource: "antigravity cli does not expose tier; see app settings",
		Period:     "current",
		Used:       "n/a",
		Limit:      "100",
		Unit:       "percent",
		Source:     "agy-cli",
		Status:     "warn",
		Message:    "No data: Antigravity usage unavailable (run agy and check /usage)",
	}
}

var antigravityUsageCommandOutput = commandOutputDetached

// describeAntigravityCredential reports the agy CLI delegation: oct never
// holds Antigravity credentials, the `agy` binary manages them itself, so the
// meaningful check is only whether that binary is reachable.
func describeAntigravityCredential() CredentialStatus {
	found := credentialBinaryPresent("agy")
	status := credentialStatus([]CredentialSource{{
		Kind:     CredentialKindCLI,
		Location: "agy (credentials managed by the agy CLI)",
		Found:    found,
	}})
	if !found {
		status.Note = "install the agy CLI; oct delegates `agy --print /usage`"
	}
	return status
}

func FetchAntigravityUsage(ctx context.Context) UsageResult {
	result := withPlanDetection(ctx, baseAntigravityUsageResult(), detectAntigravityPlan)
	if cliResult, ok := fetchAntigravityCLIUsage(ctx, result); ok {
		return cliResult
	}
	return result
}

func FetchGeminiUsage(ctx context.Context) UsageResult {
	return FetchAntigravityUsage(ctx)
}

// ReconnectAntigravity runs agy's own sign-in UI without racing a quota poll
// for the same credentials. The next poll verifies whether sign-in succeeded.
func ReconnectAntigravity(ctx context.Context) error {
	command := execenv.CommandContext(ctx, "agy")
	if command.Err != nil {
		return fmt.Errorf("find Antigravity CLI (agy): %w", command.Err)
	}
	wait, cancel := context.WithTimeout(ctx, 20*time.Second)
	unlock, err := lockAntigravityUsage(wait)
	cancel()
	if err != nil {
		return fmt.Errorf("wait for Antigravity usage lock: %w", err)
	}
	defer unlock()

	// An initial prompt can run after sign-in. Only queue /logout when agy
	// hasn't already reported that it needs a login. The detached probe can
	// wait for an OAuth code even after printing the auth-required diagnostic.
	out, probeErr := antigravityUsageCommandOutput(ctx, 3*time.Second, command.Path, "--print", "/usage")
	if !isAntigravityAuthFailure(out, probeErr) {
		command.Args = append(command.Args, "--prompt-interactive", "/logout")
	}

	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("run Antigravity sign-in: %w", err)
	}
	// Forget both backoff and cached quotas: the account may have changed.
	if err := os.Remove(antigravityCachePath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear Antigravity usage cache: %w", err)
	}
	return nil
}

type antigravityCLIUsageRow struct {
	Label     string
	Window    string
	Remaining float64
	ResetTime string
}

func fetchAntigravityCLIUsage(ctx context.Context, base UsageResult) (UsageResult, bool) {
	unlock, err := lockAntigravityUsage(ctx)
	if err != nil {
		if cached, ok := serveAntigravityCachedUsage(base, readAntigravityUsageCache(), time.Now(), "refresh lock unavailable"); ok {
			return cached, true
		}
		base.Message = "No data: Antigravity usage unavailable (refresh lock unavailable)"
		return base, true
	}
	defer unlock()
	// Recheck only after acquiring the shared lock: another process may have
	// refreshed or entered auth backoff while this one waited.
	now := time.Now()
	entry := readAntigravityUsageCache()
	// Rate-limit the live agy run (min interval / auth backoff) and fall
	// back to the last good numbers; see antigravity_cache.go for why the
	// per-minute menubar poll must not reach agy every time.
	if holdReason, authRequired := antigravityFetchHold(entry, now); holdReason != "" {
		base.AuthRequired = authRequired
		if result, ok := serveAntigravityCachedUsage(base, entry, now, holdReason); ok {
			return result, true
		}
		base.Message = "No data: " + holdReason
		return base, true
	}

	// Leave time to classify and persist agy's output before the outer
	// usage deadline. An unauthenticated agy can wait for an OAuth code.
	timeout := 20 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		timeout = min(timeout, time.Until(deadline)-time.Second)
	}
	out, err := antigravityUsageCommandOutput(ctx, timeout, "agy", "--print", "/usage")
	if isAntigravityAuthFailure(out, err) {
		detail := antigravityAuthFailureDetail(out, err)
		markAntigravityAuthFailure(now, detail)
		base.AuthRequired = true
		base.Message = fmt.Sprintf("Antigravity auth failed (%s); run `oct usage reconnect antigravity` to re-authenticate.", detail)
		return base, true
	}
	if err != nil || strings.TrimSpace(out) == "" {
		return base, false
	}

	rows := parseAntigravityCLIUsage(out)
	if len(rows) == 0 {
		return base, false
	}

	result := base
	result.Status = "ok"
	result.Source = "agy-cli"
	result.Unit = "percent"
	result.Limit = "100"
	result.Buckets = map[string]string{}
	result.BucketResets = map[string]string{}
	result.Message = "Usage parsed from agy --print /usage"

	maxUsed := 0.0
	debugParts := make([]string, 0, len(rows))
	for _, row := range rows {
		used := UsedFromRemainingPercent(row.Remaining)
		if used > maxUsed {
			maxUsed = used
		}
		key := antigravityCLIUsageBucketKey(row)
		result.Buckets[key] = fmt.Sprintf("%.1f", used)
		if strings.TrimSpace(row.ResetTime) != "" {
			result.BucketResets[key] = strings.TrimSpace(row.ResetTime)
		}
		debugParts = append(debugParts, fmt.Sprintf("%s remaining=%.1f reset=%s", strings.TrimPrefix(key, "model:"), row.Remaining, row.ResetTime))
	}
	result.Used = fmt.Sprintf("%.1f", maxUsed)
	if len(result.BucketResets) == 0 {
		result.BucketResets = nil
	}
	if osDebugEnabled() {
		result.SourceDetail = strings.Join(debugParts, ";")
	}
	saveAntigravityLastGoodUsage(result.Buckets, result.BucketResets, now)
	return result, true
}

func parseAntigravityCLIUsage(output string) []antigravityCLIUsageRow {
	rows := []antigravityCLIUsageRow{}
	for _, line := range strings.Split(output, "\n") {
		row, ok := parseAntigravityCLIUsageLine(line)
		if ok {
			rows = append(rows, row)
		}
	}
	return rows
}

var (
	antigravityANSIRe        = regexp.MustCompile("\x1b\\[[0-9;]*[A-Za-z]")
	antigravityColumnSplitRe = regexp.MustCompile("(?:\t| {2,})")
	antigravityShortWindowRe = regexp.MustCompile("(?i)^[0-9]+[hdwm]$")
)

var antigravityWindowUnits = map[string]string{
	"hour": "h", "hours": "h", "hr": "h", "hrs": "h",
	"day": "d", "days": "d",
	"week": "w", "weeks": "w",
	"month": "m", "months": "m",
}

var antigravityNumberWords = map[string]int{
	"one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6,
	"seven": 7, "eight": 8, "nine": 9, "ten": 10, "eleven": 11, "twelve": 12,
}

func parseAntigravityCLIUsageLine(line string) (antigravityCLIUsageRow, bool) {
	return parseAntigravityCLIUsageLineInLocation(line, time.Local)
}

// parseAntigravityCLIUsageLineInLocation parses one quota row. Rows are
// column-aligned (tabs or 2+ spaces) as "<model>  <description>  <N%>
// <reset>", e.g. "Gemini Models  Five Hour Limit Remaining  44%  2026-11-23
// 17:44 KST" or "Gemini 4 Argon  Gemini 4 Argon Weekly Limit  100%  ...".
// A single-space token fallback keeps older, tightly spaced output working.
func parseAntigravityCLIUsageLineInLocation(line string, loc *time.Location) (antigravityCLIUsageRow, bool) {
	trimmed := strings.TrimSpace(antigravityANSIRe.ReplaceAllString(line, ""))
	if trimmed == "" {
		return antigravityCLIUsageRow{}, false
	}

	var cols []string
	for _, part := range antigravityColumnSplitRe.Split(trimmed, -1) {
		if part = strings.TrimSpace(part); part != "" {
			cols = append(cols, part)
		}
	}
	if len(cols) >= 3 && strings.Contains(strings.ToLower(cols[1]), "limit") {
		if remaining, ok := parseAntigravityRemainingPercent(cols[2]); ok {
			return antigravityCLIUsageRow{
				Label:     cols[0],
				Window:    extractAntigravityColumnWindow(cols[0], cols[1]),
				Remaining: remaining,
				ResetTime: normalizeAntigravityResetTime(strings.Join(cols[3:], " "), loc),
			}, true
		}
	}

	fields := strings.Fields(trimmed)
	if len(fields) < 3 {
		return antigravityCLIUsageRow{}, false
	}
	for i := 2; i < len(fields); i++ {
		remaining, ok := parseAntigravityRemainingPercent(fields[i])
		if !ok {
			continue
		}
		limitIdx := -1
		switch {
		case strings.EqualFold(fields[i-1], "limit"):
			limitIdx = i - 1
		case i >= 3 && strings.EqualFold(fields[i-2], "limit") && strings.EqualFold(fields[i-1], "remaining"):
			limitIdx = i - 2
		}
		if limitIdx <= 0 {
			continue
		}
		window, labelEnd := splitTrailingAntigravityWindow(fields[:limitIdx])
		if labelEnd <= 0 {
			return antigravityCLIUsageRow{}, false
		}
		return antigravityCLIUsageRow{
			Label:     deduplicateRepeatedAntigravityLabel(strings.Join(fields[:labelEnd], " ")),
			Window:    window,
			Remaining: remaining,
			ResetTime: normalizeAntigravityResetTime(strings.Join(fields[i+1:], " "), loc),
		}, true
	}
	return antigravityCLIUsageRow{}, false
}

func parseAntigravityRemainingPercent(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if !strings.HasSuffix(s, "%") {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
	if err != nil || v < 0 || v > 100 {
		return 0, false
	}
	return v, true
}

// deduplicateRepeatedAntigravityLabel collapses "X Y X Y" into "X Y": the
// single-space fallback sees the model name twice when the description
// column repeats it ("Gemini 4 Argon Gemini 4 Argon Weekly Limit").
func deduplicateRepeatedAntigravityLabel(label string) string {
	tokens := strings.Fields(label)
	if n := len(tokens); n >= 2 && n%2 == 0 {
		half := n / 2
		same := true
		for i := 0; i < half; i++ {
			if !strings.EqualFold(tokens[i], tokens[half+i]) {
				same = false
				break
			}
		}
		if same {
			return strings.Join(tokens[:half], " ")
		}
	}
	return label
}

// extractAntigravityColumnWindow derives the window from the description
// column ("Five Hour Limit Remaining" -> "Five Hour"), dropping a repeated
// model-name prefix ("Gemini 4 Argon Weekly Limit" -> "Weekly").
func extractAntigravityColumnWindow(label, desc string) string {
	tokens := strings.Fields(desc)
	limitIdx := -1
	for i, tok := range tokens {
		if strings.EqualFold(tok, "limit") {
			limitIdx = i
		}
	}
	if limitIdx < 0 {
		return ""
	}
	before := tokens[:limitIdx]
	labelTokens := strings.Fields(label)
	if len(before) > len(labelTokens) {
		prefix := true
		for i, tok := range labelTokens {
			if !strings.EqualFold(before[i], tok) {
				prefix = false
				break
			}
		}
		if prefix {
			before = before[len(labelTokens):]
		}
	}
	if window, _ := splitTrailingAntigravityWindow(before); window != "" {
		return window
	}
	return strings.Join(before, " ")
}

// splitTrailingAntigravityWindow peels a trailing window descriptor
// ("Weekly", "5h", "Five Hour") off tokens and returns it with the index
// where the remaining label tokens end.
func splitTrailingAntigravityWindow(tokens []string) (string, int) {
	n := len(tokens)
	if n >= 2 {
		if _, isUnit := antigravityWindowUnits[strings.ToLower(tokens[n-1])]; isUnit {
			if v, ok := antigravityNumberValue(tokens[n-2]); ok && v > 0 {
				return tokens[n-2] + " " + tokens[n-1], n - 2
			}
		}
	}
	if n >= 1 && looksLikeAntigravityUsageWindow(tokens[n-1]) {
		return tokens[n-1], n - 1
	}
	return "", n
}

func antigravityNumberValue(token string) (int, bool) {
	token = strings.ToLower(strings.TrimSpace(token))
	if v, ok := antigravityNumberWords[token]; ok {
		return v, true
	}
	v, err := strconv.Atoi(token)
	if err != nil || v <= 0 {
		return 0, false
	}
	return v, true
}

func looksLikeAntigravityUsageWindow(value string) bool {
	s := strings.ToLower(strings.TrimSpace(value))
	switch s {
	case "hourly", "daily", "weekly", "monthly":
		return true
	}
	if antigravityShortWindowRe.MatchString(s) {
		return true
	}
	if num, unit, ok := strings.Cut(s, "-"); ok {
		if _, isUnit := antigravityWindowUnits[unit]; isUnit {
			_, isNum := antigravityNumberValue(num)
			return isNum
		}
	}
	return false
}

// antigravityUsageWindowTag maps a window descriptor to the short bucket
// suffix shared with other providers (5h, 7d, 1m); unknown descriptors are
// kept verbatim so distinct rows never collide on one bucket key.
func antigravityUsageWindowTag(window string) string {
	s := strings.Join(strings.Fields(strings.ReplaceAll(strings.ToLower(strings.TrimSpace(window)), "-", " ")), " ")
	if s == "" {
		return ""
	}
	switch s {
	case "weekly", "1w", "7d", "1 week", "one week", "7 day", "7 days", "seven day", "seven days":
		return "7d"
	case "five hour", "five hours", "5 hour", "5 hours", "5h":
		return "5h"
	case "daily", "1d", "1 day", "one day", "24h", "24 hour", "24 hours":
		return "1d"
	case "monthly", "1m", "1 month", "one month":
		return "1m"
	case "hourly", "1h", "1 hour", "one hour":
		return "1h"
	}
	if antigravityShortWindowRe.MatchString(s) {
		return s
	}
	if parts := strings.Fields(s); len(parts) == 2 {
		n, okNum := antigravityNumberValue(parts[0])
		unit, okUnit := antigravityWindowUnits[parts[1]]
		if okNum && okUnit {
			if unit == "w" && n == 1 {
				return "7d"
			}
			return fmt.Sprintf("%d%s", n, unit)
		}
	}
	return strings.TrimSpace(window)
}

func antigravityCLIUsageBucketLabel(label string) string {
	norm := strings.Join(strings.Fields(strings.ToLower(label)), " ")
	switch {
	case strings.Contains(norm, "claude") && strings.Contains(norm, "gpt"):
		return "Claude/GPT"
	case norm == "gemini", norm == "gemini models", norm == "google", norm == "google models":
		return "Gemini"
	default:
		// Specific models ("Gemini 4 Argon") keep their own name.
		return strings.Join(strings.Fields(label), " ")
	}
}

// antigravityCLIUsageBucketKey builds the "model:<label> <window>" bucket
// key so each (model, window) row keeps its own bucket and reset time.
func antigravityCLIUsageBucketKey(row antigravityCLIUsageRow) string {
	label := antigravityCLIUsageBucketLabel(row.Label)
	if tag := antigravityUsageWindowTag(row.Window); tag != "" {
		return "model:" + label + " " + tag
	}
	return "model:" + label
}

// antigravityZoneOffsets lists only unambiguous abbreviations; CST/IST/MST
// are resolved solely through the local zone.
var antigravityZoneOffsets = map[string]int{
	"UTC": 0, "GMT": 0, "KST": 9 * 3600, "JST": 9 * 3600, "HKT": 8 * 3600, "SGT": 8 * 3600,
	"AWST": 8 * 3600, "AEST": 10 * 3600, "AEDT": 11 * 3600, "NZST": 12 * 3600, "NZDT": 13 * 3600,
	"CET": 3600, "CEST": 2 * 3600, "EET": 2 * 3600, "EEST": 3 * 3600, "WET": 0, "WEST": 3600,
	"BST": 3600, "EST": -5 * 3600, "EDT": -4 * 3600, "CDT": -5 * 3600, "MDT": -6 * 3600,
	"PST": -8 * 3600, "PDT": -7 * 3600, "AKST": -9 * 3600, "AKDT": -8 * 3600, "HST": -10 * 3600,
}

// parseFixedOffsetZone reads "GMT+9", "UTC-05:30", "+09", "-0500" style
// zones. A bare "GMT"/"UTC" is left to the abbreviation table.
func parseFixedOffsetZone(token string) (*time.Location, bool) {
	t := strings.TrimSpace(token)
	upper := strings.ToUpper(t)
	for _, prefix := range []string{"GMT", "UTC"} {
		if strings.HasPrefix(upper, prefix) {
			t = t[len(prefix):]
			break
		}
	}
	if len(t) < 2 || (t[0] != '+' && t[0] != '-') {
		return nil, false
	}
	sign := 1
	if t[0] == '-' {
		sign = -1
	}
	digits := t[1:]
	var hh, mm string
	switch {
	case strings.Contains(digits, ":"):
		var ok bool
		hh, mm, ok = strings.Cut(digits, ":")
		if !ok {
			return nil, false
		}
	case len(digits) <= 2:
		hh, mm = digits, "0"
	case len(digits) == 4:
		hh, mm = digits[:2], digits[2:]
	default:
		return nil, false
	}
	h, errH := strconv.Atoi(hh)
	m, errM := strconv.Atoi(mm)
	if errH != nil || errM != nil || h < 0 || h > 14 || m < 0 || m > 59 {
		return nil, false
	}
	return time.FixedZone(token, sign*(h*3600+m*60)), true
}

// normalizeAntigravityResetTime converts agy's "2026-11-30 12:44 KST" reset
// column to RFC3339 UTC so the Go and Swift countdowns can parse it. Zones it
// cannot resolve reliably return raw unchanged rather than a wrong offset.
func normalizeAntigravityResetTime(raw string, loc *time.Location) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if loc == nil {
		loc = time.Local
	}
	if _, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return raw
	}
	for _, layout := range []string{
		"2006-01-02 15:04:05 -0700", "2006-01-02 15:04 -0700",
		"2006-01-02 15:04:05 -07:00", "2006-01-02 15:04 -07:00",
		"2006-01-02 15:04:05 Z07:00", "2006-01-02 15:04 Z07:00",
	} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
	}

	parts := strings.Fields(raw)
	layouts := []string{"2006-01-02 15:04:05", "2006-01-02 15:04"}
	switch len(parts) {
	case 2:
		for _, layout := range layouts {
			if t, err := time.ParseInLocation(layout, raw, loc); err == nil {
				return t.UTC().Format(time.RFC3339)
			}
		}
	case 3:
		dateTime, tzToken := parts[0]+" "+parts[1], parts[2]
		for _, layout := range layouts {
			local, err := time.ParseInLocation(layout, dateTime, loc)
			if err != nil {
				continue
			}
			if zoneName, _ := local.Zone(); strings.EqualFold(zoneName, tzToken) {
				return local.UTC().Format(time.RFC3339)
			}
			if fixed, ok := parseFixedOffsetZone(tzToken); ok {
				if t, err := time.ParseInLocation(layout, dateTime, fixed); err == nil {
					return t.UTC().Format(time.RFC3339)
				}
			}
			if !strings.EqualFold(tzToken, "Local") {
				if named, err := time.LoadLocation(tzToken); err == nil {
					if t, err := time.ParseInLocation(layout, dateTime, named); err == nil {
						return t.UTC().Format(time.RFC3339)
					}
				}
			}
			if offset, ok := antigravityZoneOffsets[strings.ToUpper(tzToken)]; ok {
				zone := time.FixedZone(strings.ToUpper(tzToken), offset)
				if t, err := time.ParseInLocation(layout, dateTime, zone); err == nil {
					return t.UTC().Format(time.RFC3339)
				}
			}
			return raw
		}
	}
	return raw
}

func osDebugEnabled() bool {
	return strings.TrimSpace(os.Getenv("OCT_USAGE_DEBUG")) == "1"
}
