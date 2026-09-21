package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/suho-han/one-click-ai-tools/internal/netclient"
)

// Z.ai GLM Coding Plan quota. Z.ai is an account-level service with no CLI of
// its own, so it runs as a standalone (opt-in) provider.
//
// Endpoint: GET {base}/api/monitor/usage/quota/limit
//
//	base:    https://api.z.ai (global) | https://open.bigmodel.cn (China)
//
// Auth:     Authorization: <token>  -- raw token, deliberately NO Bearer prefix
// Response: {"data":{"limits":[{"type":"TOKENS_LIMIT","percentage":42}, ...]}}
//
// Token resolution honors the ways users actually wire GLM into their coding
// tools: a dedicated Z.ai key, a Zhipu (China) key, an Anthropic-protocol
// token pointed at Z.ai via ANTHROPIC_BASE_URL, or an OpenCode auth.json
// entry created by `opencode auth login`.
const (
	zaiGlobalBaseURL = "https://api.z.ai"
	zaiChinaBaseURL  = "https://open.bigmodel.cn"
	zaiUsagePath     = "/api/monitor/usage/quota/limit"
)

// zaiUsageEndpoint allows overriding the API URL for testing (full URL).
var zaiUsageEndpoint = zaiGlobalBaseURL + zaiUsagePath

type zaiUsageResponse struct {
	Data struct {
		Limits []struct {
			Type       string          `json:"type"`
			Percentage json.RawMessage `json:"percentage"`
		} `json:"limits"`
	} `json:"data"`
}

// resolveZaiCredential finds an authorization token and the platform base URL
// it belongs to. Priority: ZAI_API_KEY (global) -> ZHIPU_API_KEY (China) ->
// ANTHROPIC_AUTH_TOKEN when ANTHROPIC_BASE_URL points at Z.ai/Zhipu (a bare
// Anthropic token must never be sent to Z.ai) -> OpenCode auth.json.
func resolveZaiCredential() (token, base, source string) {
	if key := os.Getenv("ZAI_API_KEY"); key != "" {
		return key, zaiGlobalBaseURL, "env:ZAI_API_KEY"
	}
	if key := os.Getenv("ZHIPU_API_KEY"); key != "" {
		return key, zaiChinaBaseURL, "env:ZHIPU_API_KEY"
	}
	if key := os.Getenv("ANTHROPIC_AUTH_TOKEN"); key != "" {
		if base := zaiBaseFromHost(os.Getenv("ANTHROPIC_BASE_URL")); base != "" {
			return key, base, "env:ANTHROPIC_AUTH_TOKEN"
		}
	}
	return resolveZaiCredentialFromOpenCode()
}

// zaiBaseFromHost maps an Anthropic-protocol base URL to the Z.ai platform it
// belongs to; "" when the URL is not a Z.ai/Zhipu endpoint.
func zaiBaseFromHost(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return ""
	}
	host := strings.ToLower(parsed.Host)
	switch {
	case strings.Contains(host, "api.z.ai"):
		return zaiGlobalBaseURL
	case strings.Contains(host, "bigmodel.cn"):
		return zaiChinaBaseURL
	default:
		return ""
	}
}

// resolveZaiCredentialFromOpenCode reads ~/.local/share/opencode/auth.json and
// returns the first Z.ai/Zhipu entry found (key for api-type entries, access
// token for oauth-type ones).
func resolveZaiCredentialFromOpenCode() (token, base, source string) {
	home, _ := userHomeDir()
	if home == "" {
		return "", "", ""
	}
	data, err := os.ReadFile(filepath.Join(home, ".local", "share", "opencode", "auth.json"))
	if err != nil {
		return "", "", ""
	}

	var auths map[string]struct {
		Key    string `json:"key"`
		Access string `json:"access"`
		Type   string `json:"type"`
	}
	if err := json.Unmarshal(data, &auths); err != nil {
		return "", "", ""
	}

	lookup := func(names ...string) string {
		for _, name := range names {
			auth, ok := auths[name]
			if !ok {
				continue
			}
			if auth.Key != "" {
				return auth.Key
			}
			if auth.Access != "" {
				return auth.Access
			}
		}
		return ""
	}

	if key := lookup("zai-coding-plan", "zai"); key != "" {
		return key, zaiGlobalBaseURL, "opencode:auth.json"
	}
	if key := lookup("zhipu"); key != "" {
		return key, zaiChinaBaseURL, "opencode:auth.json"
	}
	return "", "", ""
}

// describeZaiCredential probes the Z.ai token chain in fetch priority order:
// ZAI_API_KEY -> ZHIPU_API_KEY -> ANTHROPIC_AUTH_TOKEN (only when
// ANTHROPIC_BASE_URL points at Z.ai/Zhipu) -> OpenCode auth.json entries.
func describeZaiCredential() CredentialStatus {
	anthropicSource := CredentialSource{
		Kind:     CredentialKindEnv,
		Location: "ANTHROPIC_AUTH_TOKEN",
		Found:    false,
		Note:     "only used when ANTHROPIC_BASE_URL points at a Z.ai/Zhipu host",
	}
	if credentialEnvFound("ANTHROPIC_AUTH_TOKEN") && zaiBaseFromHost(os.Getenv("ANTHROPIC_BASE_URL")) != "" {
		anthropicSource.Found = true
	}

	authPath := credentialHomePath(".local", "share", "opencode", "auth.json")
	sources := []CredentialSource{
		{
			Kind:     CredentialKindEnv,
			Location: "ZAI_API_KEY",
			Found:    credentialEnvFound("ZAI_API_KEY"),
		},
		{
			Kind:     CredentialKindEnv,
			Location: "ZHIPU_API_KEY",
			Found:    credentialEnvFound("ZHIPU_API_KEY"),
			Note:     "China platform (open.bigmodel.cn)",
		},
		anthropicSource,
		{
			Kind:     CredentialKindFile,
			Location: authPath,
			Found:    authPath != "" && credentialJSONEntryKeyPresent(authPath, "zai-coding-plan", "zai", "zhipu"),
			Note:     "entries written by `opencode auth login`",
		},
	}
	status := credentialStatus(sources)
	if status.Status == CredentialStatusMissing {
		status.Note = "set ZAI_API_KEY/ZHIPU_API_KEY or run 'opencode auth login'"
	}
	return status
}

// zaiBucketFor maps a limits[] entry type to an oct bucket label; ok=false for
// unrecognized types (surfaced via SourceDetail debug instead of guessing).
// WEEK is tested before TOKEN because weekly limits are token-typed too.
func zaiBucketFor(limitType string) (string, bool) {
	t := strings.ToUpper(limitType)
	switch {
	case strings.Contains(t, "WEEK"):
		return "7d", true
	case strings.Contains(t, "TOKEN"):
		return "5h", true
	case strings.Contains(t, "TIME"), strings.Contains(t, "MONTH"):
		return "1m", true
	default:
		return "", false
	}
}

// FetchZaiUsage fetches GLM Coding Plan quota percentages.
func FetchZaiUsage(ctx context.Context) UsageResult {
	result := UsageResult{
		Provider: "zai",
		Period:   "5h/7d/1m",
		Unit:     "percent",
		Source:   "remote",
		Status:   "warn",
		Message:  "No data: Z.ai token not found (set ZAI_API_KEY/ZHIPU_API_KEY or run 'opencode auth login')",
	}

	token, base, source := resolveZaiCredential()
	if token == "" {
		return result
	}
	result.SourceDetail = fmt.Sprintf("auth_source=%s", source)

	endpoint := os.Getenv("OCT_ZAI_USAGE_ENDPOINT")
	if endpoint == "" {
		endpoint = base + zaiUsagePath
	}

	resp, err := fetchZaiUsage(ctx, endpoint, token)
	if err != nil {
		result.Status = "error"
		result.Used = "n/a"
		result.Message = fmt.Sprintf("API error: %v", err)
		if osDebugEnabled() {
			result.SourceDetail = fmt.Sprintf("auth_source=%s endpoint=%s", source, endpoint)
		}
		return result
	}

	result.Status = "ok"
	result.Buckets = map[string]string{}

	var unmatched []string
	for _, limit := range resp.Data.Limits {
		bucket, ok := zaiBucketFor(limit.Type)
		if !ok {
			unmatched = append(unmatched, limit.Type)
			continue
		}
		if value, ok := zaiPercentString(limit.Percentage); ok {
			result.Buckets[bucket] = value
		}
	}

	result.Used = firstNonEmpty(result.Buckets["5h"], result.Buckets["7d"], result.Buckets["1m"])
	result.Message = "Fetched from Z.ai quota API"
	if result.Used == "" {
		result.Status = "warn"
		result.Message = "No usable quota limits in Z.ai response"
	}
	if osDebugEnabled() {
		result.SourceDetail = fmt.Sprintf("auth_source=%s endpoint=%s", source, endpoint)
		if len(unmatched) > 0 {
			result.SourceDetail += " unmatched_types=" + strings.Join(unmatched, "|")
		}
	}

	return result
}

// zaiPercentString renders a percentage JSON value (number or numeric string)
// as the raw stored string form used across oct.
func zaiPercentString(raw json.RawMessage) (string, bool) {
	s := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	if s == "" || s == "null" {
		return "", false
	}
	var f float64
	if err := json.Unmarshal([]byte(s), &f); err != nil {
		return "", false
	}
	return fmt.Sprintf("%.0f", f), true
}

// fetchZaiUsage calls the Z.ai quota endpoint. The platform token goes in the
// Authorization header verbatim -- the API rejects "Bearer "-prefixed values.
func fetchZaiUsage(ctx context.Context, endpoint, token string) (*zaiUsageResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en")
	req.Header.Set("User-Agent", "one-click-tools/1.0")

	resp, err := netclient.DefaultClient.DoWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := readAllCapped(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed zaiUsageResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &parsed, nil
}
