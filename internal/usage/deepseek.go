package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/suho-han/one-click-ai-tools/internal/netclient"
)

// DeepSeek platform balance. DeepSeek is a pay-as-you-go API (no coding-plan
// windows), so the provider reports the account's remaining balance; it runs
// as a standalone (opt-in) provider.
//
// Endpoint: GET https://api.deepseek.com/user/balance
// Auth:     Authorization: Bearer <DEEPSEEK_API_KEY>
// Response:
//
//	{"is_available": true,
//	 "balance_infos": [{"currency":"USD","total_balance":"110.00",
//	                    "granted_balance":"10.00","topped_up_balance":"100.00"}]}
const deepseekBalanceURL = "https://api.deepseek.com/user/balance"

// deepseekBalanceEndpoint allows overriding the API URL for testing.
var deepseekBalanceEndpoint = deepseekBalanceURL

type deepseekBalanceResponse struct {
	IsAvailable  bool `json:"is_available"`
	BalanceInfos []struct {
		Currency string `json:"currency"`
		Total    string `json:"total_balance"`
		Granted  string `json:"granted_balance"`
		ToppedUp string `json:"topped_up_balance"`
	} `json:"balance_infos"`
}

// describeDeepseekCredential probes the single DeepSeek source: the
// DEEPSEEK_API_KEY env var.
func describeDeepseekCredential() CredentialStatus {
	status := credentialStatus([]CredentialSource{{
		Kind:     CredentialKindEnv,
		Location: "DEEPSEEK_API_KEY",
		Found:    credentialEnvFound("DEEPSEEK_API_KEY"),
	}})
	if status.Status == CredentialStatusMissing {
		status.Note = "set DEEPSEEK_API_KEY"
	}
	return status
}

// FetchDeepseekUsage reports the DeepSeek account balance.
func FetchDeepseekUsage(ctx context.Context) UsageResult {
	result := UsageResult{
		Provider: "deepseek",
		Period:   "balance",
		Source:   "remote",
		Status:   "warn",
		Message:  "No data: DEEPSEEK_API_KEY not set",
	}

	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		return result
	}

	endpoint := os.Getenv("OCT_DEEPSEEK_BALANCE_ENDPOINT")
	if endpoint == "" {
		endpoint = deepseekBalanceEndpoint
	}
	result.SourceDetail = "auth_source=env:DEEPSEEK_API_KEY"

	resp, err := fetchDeepseekBalance(ctx, endpoint, apiKey)
	if err != nil {
		result.Status = "error"
		result.Used = "n/a"
		result.Message = fmt.Sprintf("API error: %v", err)
		if osDebugEnabled() {
			result.SourceDetail = fmt.Sprintf("auth_source=env:DEEPSEEK_API_KEY endpoint=%s", endpoint)
		}
		return result
	}

	if len(resp.BalanceInfos) == 0 {
		result.Status = "warn"
		result.Message = "No balance info in DeepSeek response"
		return result
	}

	// The primary row uses the first balance entry; extra currencies travel
	// in the message. Currencies are never summed (they are different units).
	primary := resp.BalanceInfos[0]
	result.Unit = primary.Currency
	result.Used = strings.TrimSpace(primary.Total)
	result.Status = "ok"
	result.Period = "balance"
	result.Message = "Fetched from DeepSeek balance API"
	if !resp.IsAvailable {
		result.Status = "warn"
		result.Message = "Balance fetched but DeepSeek reports the account unavailable"
	}
	if extra := balanceSummary(resp.BalanceInfos[1:]); extra != "" {
		result.Message += " (" + extra + ")"
	}
	if osDebugEnabled() {
		result.SourceDetail = fmt.Sprintf("auth_source=env:DEEPSEEK_API_KEY endpoint=%s", endpoint)
	}

	return result
}

func balanceSummary(infos []struct {
	Currency string `json:"currency"`
	Total    string `json:"total_balance"`
	Granted  string `json:"granted_balance"`
	ToppedUp string `json:"topped_up_balance"`
}) string {
	parts := make([]string, 0, len(infos))
	for _, info := range infos {
		if total := strings.TrimSpace(info.Total); total != "" {
			parts = append(parts, total+" "+info.Currency)
		}
	}
	return strings.Join(parts, ", ")
}

// fetchDeepseekBalance calls the DeepSeek balance API and parses the response.
func fetchDeepseekBalance(ctx context.Context, endpoint, apiKey string) (*deepseekBalanceResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
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

	var parsed deepseekBalanceResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &parsed, nil
}
