package usage

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/suho-han/one-click-ai-tools/internal/netclient"
)

const claudeUsageEndpoint = "https://api.anthropic.com/api/oauth/usage"

var claudeVersionPrefix = regexp.MustCompile(`^\d+\.\d+\.\d+`)

func fetchClaudeOAuthUsage(ctx context.Context, token string) (*http.Response, error) {
	// The optional inventory is tied to the Claude Code surface. Identify the
	// installed version rather than pinning a version the user may not have.
	versionOutput, _ := commandOutput(ctx, time.Second, "claude", "--version")
	if version := claudeVersionPrefix.FindString(strings.TrimSpace(versionOutput)); version != "" {
		query := url.Values{"cedar_ember": {"1"}, "skip_spend": {"1"}}
		resp, err := requestClaudeOAuthUsage(ctx, token, claudeUsageEndpoint+"?"+query.Encode(), "claude-cli/"+version+" (external, cli)")
		if err != nil || (resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusForbidden) {
			return resp, err
		}
		// An unsupported optional inventory must not hide regular usage.
		resp.Body.Close()
	}
	return requestClaudeOAuthUsage(ctx, token, claudeUsageEndpoint, "")
}

func requestClaudeOAuthUsage(ctx context.Context, token, endpoint, userAgent string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	// Let the shared usage cache handle backoff instead of retrying inline.
	return netclient.DefaultClient.HTTPClient.Do(req)
}

func parseClaudeResetCredits(raw, limits json.RawMessage, now time.Time) (*int, []ResetCreditDetail) {
	var inventory struct {
		Eligible bool              `json:"eligible"`
		Grants   []json.RawMessage `json:"grants"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &inventory) != nil || !inventory.Eligible || inventory.Grants == nil {
		return nil, nil
	}

	scopedModel := claudeScopedWeeklyModel(limits)
	var details []ResetCreditDetail
	count := 0
	for _, rawGrant := range inventory.Grants {
		var grant struct {
			Label     string          `json:"label"`
			Left      *int            `json:"resets_left"`
			Paused    bool            `json:"paused"`
			StartsAt  json.RawMessage `json:"starts_at"`
			ExpiresAt json.RawMessage `json:"ends_at"`
			Clears    json.RawMessage `json:"clears"`
		}
		if json.Unmarshal(rawGrant, &grant) != nil || grant.Left == nil || *grant.Left < 0 {
			return nil, nil // A partial inventory cannot confirm a lower count.
		}
		if *grant.Left == 0 || grant.Paused {
			continue
		}
		starts, startOK := parseClaudeGrantTime(grant.StartsAt)
		ends, endOK := parseClaudeGrantTime(grant.ExpiresAt)
		if !startOK || !endOK {
			return nil, nil
		}
		if now.Before(starts) || (!ends.IsZero() && !now.Before(ends)) {
			continue
		}
		// A private response must not turn a malformed count into an unbounded tooltip.
		if *grant.Left > 50-count {
			return nil, nil
		}
		detail := ResetCreditDetail{Title: strings.TrimSpace(grant.Label), Count: *grant.Left}
		detail.Scope = claudeGrantScope(grant.Clears, detail.Title, scopedModel)
		if !ends.IsZero() {
			detail.ExpiresAt = ends.UTC().Format(time.RFC3339Nano)
		}
		details = append(details, detail)
		count += detail.Count
	}
	return &count, details
}

func claudeScopedWeeklyModel(raw json.RawMessage) string {
	var limits []struct {
		Kind  string `json:"kind"`
		Scope struct {
			Model struct {
				DisplayName string `json:"display_name"`
			} `json:"model"`
		} `json:"scope"`
	}
	if json.Unmarshal(raw, &limits) != nil {
		return ""
	}
	model := ""
	for _, limit := range limits {
		if limit.Kind != "weekly_scoped" {
			continue
		}
		if name := strings.TrimSpace(limit.Scope.Model.DisplayName); name != "" {
			if model != "" && model != name {
				return "" // The generic scoped window cannot identify one of several models.
			}
			model = name
		}
	}
	return model
}

func claudeGrantScope(raw json.RawMessage, title, scopedModel string) string {
	var clears []string
	if json.Unmarshal(raw, &clears) != nil {
		return ""
	}

	var session, weekly bool
	var scoped []string
	for _, window := range clears {
		switch window {
		case "five_hour":
			session = true
		case "seven_day":
			weekly = true
		case "seven_day_sonnet":
			scoped = append(scoped, "Sonnet · Weekly")
		case "seven_day_opus":
			scoped = append(scoped, "Opus · Weekly")
		case "seven_day_overage_included":
			model := scopedModel
			// Fable launch grants use a generic scoped-window key. The
			// grant label can identify Fable even without a matching limit row.
			if strings.Contains(strings.ToLower(title), "fable") {
				model = "Fable"
			}
			if model == "" {
				model = "Model-specific"
			}
			scoped = append(scoped, model+" · Weekly")
		}
	}
	switch {
	case weekly && session:
		return "All models · 5h + Weekly"
	case weekly:
		return "All models · Weekly"
	case session && len(scoped) == 0:
		return "All models · 5h"
	case session:
		return "All models · 5h + " + strings.Join(scoped, " + ")
	default:
		return strings.Join(scoped, " + ")
	}
}

func parseClaudeGrantTime(raw json.RawMessage) (time.Time, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return time.Time{}, true
	}
	text := string(raw)
	if text[0] == '"' {
		var err error
		text, err = strconv.Unquote(text)
		if err != nil {
			return time.Time{}, false
		}
	}
	return parseBucketResetTime(text)
}
