package usage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/suho-han/one-click-ai-tools/internal/netclient"
)

// MiniMax Coding/Token Plan quota. The plan API reports 5-hour and weekly
// request windows as used/total counts.
//
// Endpoint: POST {base}/v1/coding_plan/remains   (base: api.minimax.io |
//
//	api.minimaxi.com for mainland China; MINIMAX_REGION=cn switches)
//
// Auth:     Authorization: Bearer <MINIMAX_CODING_API_KEY | MINIMAX_API_KEY>
// Response (numeric fields may arrive as JSON numbers or strings):
//
//	{"current_interval_total_count": 200, "current_interval_usage_count": 139,
//	 "end_time": 1760000000000, "current_weekly_total_count": 2048,
//	 "current_weekly_usage_count": 214, "weekly_end_time": ...}
const minimaxUsageURL = "https://api.minimax.io/v1/coding_plan/remains"

// minimaxUsageEndpoint allows overriding the API URL for testing.
var minimaxUsageEndpoint = minimaxUsageURL

// flexNum accepts a JSON number or a numeric string; the MiniMax API has
// mixed the two across versions.
type flexNum float64

func (f *flexNum) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("not a number: %q", s)
	}
	*f = flexNum(v)
	return nil
}

type minimaxRemains struct {
	IntervalTotal  flexNum `json:"current_interval_total_count"`
	IntervalUsed   flexNum `json:"current_interval_usage_count"`
	EndTime        flexNum `json:"end_time"`
	WeeklyTotal    flexNum `json:"current_weekly_total_count"`
	WeeklyUsed     flexNum `json:"current_weekly_usage_count"`
	WeeklyEndTime  flexNum `json:"weekly_end_time"`
	IntervalStatus string  `json:"current_interval_status"`
	WeeklyStatus   string  `json:"current_weekly_status"`
}

func (m minimaxRemains) nonEmpty() bool {
	return m.IntervalTotal > 0 || m.WeeklyTotal > 0 || m.IntervalUsed > 0 || m.WeeklyUsed > 0
}

// resolveMinimaxToken finds the plan API token.
// Priority: MINIMAX_CODING_API_KEY env -> MINIMAX_API_KEY env.
func resolveMinimaxToken() (string, string) {
	if key := os.Getenv("MINIMAX_CODING_API_KEY"); key != "" {
		return key, "env:MINIMAX_CODING_API_KEY"
	}
	if key := os.Getenv("MINIMAX_API_KEY"); key != "" {
		return key, "env:MINIMAX_API_KEY"
	}
	return "", ""
}

// minimaxBaseURL picks the plan API host: mainland China when MINIMAX_REGION
// is "cn"/"china", global otherwise.
func minimaxBaseURL() string {
	switch strings.ToLower(os.Getenv("MINIMAX_REGION")) {
	case "cn", "china":
		return "https://api.minimaxi.com"
	default:
		return "https://api.minimax.io"
	}
}

func minimaxReset(unixMs flexNum) (string, bool) {
	if unixMs <= 0 {
		return "", false
	}
	ms := float64(unixMs)
	if ms < 10_000_000_000 { // seconds, not milliseconds
		return time.Unix(int64(ms), 0).UTC().Format(time.RFC3339), true
	}
	return time.UnixMilli(int64(ms)).UTC().Format(time.RFC3339), true
}

// FetchMinimaxUsage fetches the MiniMax plan quota (5-hour + weekly windows).
func FetchMinimaxUsage(ctx context.Context) UsageResult {
	result := UsageResult{
		Provider: "minimax",
		Period:   "5h/7d",
		Unit:     "req",
		Source:   "remote",
		Status:   "warn",
		Message:  "No data: MiniMax API token not found (set MINIMAX_CODING_API_KEY or MINIMAX_API_KEY)",
	}

	token, source := resolveMinimaxToken()
	if token == "" {
		return result
	}
	result.SourceDetail = fmt.Sprintf("auth_source=%s", source)

	endpoint := os.Getenv("OCT_MINIMAX_USAGE_ENDPOINT")
	if endpoint == "" {
		endpoint = minimaxUsageEndpoint
	}

	remains, err := fetchMinimaxRemains(ctx, endpoint, token)
	if err != nil {
		result.Status = "error"
		result.Used = "n/a"
		result.Message = fmt.Sprintf("API error: %v", err)
		if osDebugEnabled() {
			result.SourceDetail = fmt.Sprintf("auth_source=%s endpoint=%s", source, endpoint)
		}
		return result
	}

	if !remains.nonEmpty() {
		result.Status = "warn"
		result.Message = "No quota counts in MiniMax response"
		if osDebugEnabled() {
			result.SourceDetail = fmt.Sprintf("auth_source=%s endpoint=%s", source, endpoint)
		}
		return result
	}

	result.Status = "ok"
	result.Buckets = map[string]string{}
	result.BucketResets = map[string]string{}

	if remains.IntervalTotal > 0 {
		result.Buckets["5h"] = fmt.Sprintf("%.0f", percentOf(remains.IntervalUsed, remains.IntervalTotal))
	}
	if reset, ok := minimaxReset(remains.EndTime); ok {
		result.BucketResets["5h"] = reset
	}
	if remains.WeeklyTotal > 0 {
		result.Buckets["7d"] = fmt.Sprintf("%.0f", percentOf(remains.WeeklyUsed, remains.WeeklyTotal))
	}
	if reset, ok := minimaxReset(remains.WeeklyEndTime); ok {
		result.BucketResets["7d"] = reset
	}

	result.Used = firstNonEmpty(result.Buckets["7d"], result.Buckets["5h"])
	if weeklyLimit := remains.WeeklyTotal; weeklyLimit > 0 {
		result.Limit = fmt.Sprintf("%.0f", float64(weeklyLimit))
	}
	result.Message = "Fetched from MiniMax plan API"

	var degraded []string
	if v, ok := ParsePercent(result.Buckets["5h"]); ok && v >= 100 {
		degraded = append(degraded, "5h exhausted")
	}
	if len(degraded) > 0 {
		result.Status = "warn"
		result.Message += " (" + strings.Join(degraded, ", ") + ")"
	}
	if osDebugEnabled() {
		result.SourceDetail = fmt.Sprintf("auth_source=%s endpoint=%s", source, endpoint)
	}

	return result
}

func percentOf(used, total flexNum) float64 {
	if total <= 0 {
		return 0
	}
	return float64(used) / float64(total) * 100
}

// fetchMinimaxRemains calls the plan remains endpoint and parses the response,
// tolerating both a bare remains object and one wrapped in "data".
func fetchMinimaxRemains(ctx context.Context, endpoint, token string) (*minimaxRemains, error) {
	body := bytes.NewReader([]byte(`{}`))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "one-click-tools/1.0")

	resp, err := netclient.DefaultClient.DoWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := readAllCapped(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var parsed minimaxRemains
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	if !parsed.nonEmpty() {
		// Some surfaces wrap the remains object in a "data" envelope.
		var wrapped struct {
			Data minimaxRemains `json:"data"`
		}
		if json.Unmarshal(raw, &wrapped) == nil && wrapped.Data.nonEmpty() {
			return &wrapped.Data, nil
		}
	}

	return &parsed, nil
}
