package usage

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/suho-han/one-click-ai-tools/internal/netclient"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const commandCodeDefaultAPIBaseURL = "https://api.commandcode.ai"

var commandCodeAPIBaseURL = commandCodeDefaultAPIBaseURL

type commandCodeAuth struct {
	APIKey          string `json:"apiKey"`
	UserID          string `json:"userId"`
	UserName        string `json:"userName"`
	KeyName         string `json:"keyName"`
	AuthenticatedAt string `json:"authenticatedAt"`
}

type commandCodeWhoamiResponse struct {
	User *struct {
		ID       string `json:"id"`
		UserName string `json:"userName"`
		Name     string `json:"name"`
	} `json:"user"`
	Org *struct {
		ID    string `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
	} `json:"org"`
}

type commandCodeCreditsResponse struct {
	Credits *struct {
		PlanID           string                   `json:"planId"`
		MonthlyCredits   float64                  `json:"monthlyCredits"`
		PurchasedCredits float64                  `json:"purchasedCredits"`
		FreeCredits      float64                  `json:"freeCredits"`
		WindowLimits     *commandCodeWindowLimits `json:"windowLimits"`
	} `json:"credits"`
	WindowLimits *commandCodeWindowLimits `json:"windowLimits"`
}

type commandCodeWindowLimits struct {
	Limited  bool                    `json:"limited"`
	FiveHour *commandCodeWindowLimit `json:"fiveHour"`
	Weekly   *commandCodeWindowLimit `json:"weekly"`
}

type commandCodeWindowLimit struct {
	Used    float64 `json:"used"`
	Cap     float64 `json:"cap"`
	ResetAt int64   `json:"resetAt"`
}

type commandCodeSubscriptionResponse struct {
	Data *struct {
		PlanID             string `json:"planId"`
		Status             string `json:"status"`
		CurrentPeriodStart string `json:"currentPeriodStart"`
		CurrentPeriodEnd   string `json:"currentPeriodEnd"`
	} `json:"data"`
}

type commandCodeSummaryResponse struct {
	TotalCost  float64 `json:"totalCost"`
	TotalCount int     `json:"totalCount"`
}

func FetchCommandCodeUsage(ctx context.Context) UsageResult {
	result := UsageResult{
		Provider:   "commandcode",
		Plan:       "unknown",
		PlanSource: "commandcode billing unavailable",
		Period:     "current",
		Used:       "n/a",
		Limit:      "100",
		Unit:       "percent",
		Source:     "remote",
		Status:     "warn",
		Message:    "No data: Command Code API key not found (run commandcode login or set COMMAND_CODE_API_KEY)",
	}

	apiKey, authSource := resolveCommandCodeAPIKey()
	if apiKey == "" {
		return result
	}

	usageData, err := fetchCommandCodeUsageData(ctx, apiKey)
	if err != nil {
		result.Status = "error"
		result.Message = fmt.Sprintf("API error: %v", err)
		if osDebugEnabled() {
			result.SourceDetail = "auth_source=" + authSource
		}
		return result
	}

	result.Status = "ok"
	result.Buckets = map[string]string{}
	result.BucketResets = map[string]string{}
	result.SourceDetail = "auth_source=" + authSource

	planID := firstNonEmpty(usageData.SubscriptionPlanID(), usageData.CreditsPlanID())
	if planID != "" {
		result.Plan = commandCodePlanLabel(planID)
		result.PlanSource = "commandcode billing planId"
	}

	if usageData.Credits != nil && usageData.Credits.Credits != nil {
		credits := usageData.Credits.Credits
		if windowLimits := commandCodeUsageWindowLimits(usageData.Credits); windowLimits != nil {
			addCommandCodeWindow(result.Buckets, result.BucketResets, "5h", windowLimits.FiveHour)
			addCommandCodeWindow(result.Buckets, result.BucketResets, "7d", windowLimits.Weekly)
		}

		monthlyPercent, ok := commandCodeMonthlyUsagePercent(planID, credits.MonthlyCredits, credits.PurchasedCredits, credits.FreeCredits, usageData.Summary)
		if ok {
			result.Buckets["1m"] = fmt.Sprintf("%.1f", monthlyPercent)
		}
	}

	switch {
	case result.Buckets["5h"] != "":
		result.Used = result.Buckets["5h"]
		result.Message = "Usage fetched from Command Code billing API (5h bucket)"
	case result.Buckets["7d"] != "":
		result.Used = result.Buckets["7d"]
		result.Message = "Usage fetched from Command Code billing API (7d bucket)"
	case result.Buckets["1m"] != "":
		result.Used = result.Buckets["1m"]
		result.Message = "Usage fetched from Command Code billing API (monthly credits)"
	default:
		result.Status = "warn"
		result.Used = "0"
		result.Message = "No data: Command Code billing API returned no usage buckets"
	}

	if usageData.Summary != nil && usageData.Summary.TotalCount > 0 {
		result.Message += fmt.Sprintf("; %d requests this cycle", usageData.Summary.TotalCount)
	}
	if !osDebugEnabled() {
		result.SourceDetail = ""
	}

	return result
}

type commandCodeUsageData struct {
	Whoami       *commandCodeWhoamiResponse
	Credits      *commandCodeCreditsResponse
	Subscription *commandCodeSubscriptionResponse
	Summary      *commandCodeSummaryResponse
}

func (d commandCodeUsageData) orgID() string {
	if d.Whoami != nil && d.Whoami.Org != nil {
		return strings.TrimSpace(d.Whoami.Org.ID)
	}
	return ""
}

func (d commandCodeUsageData) CreditsPlanID() string {
	if d.Credits != nil && d.Credits.Credits != nil {
		return strings.TrimSpace(d.Credits.Credits.PlanID)
	}
	return ""
}

func (d commandCodeUsageData) SubscriptionPlanID() string {
	if d.Subscription != nil && d.Subscription.Data != nil {
		return strings.TrimSpace(d.Subscription.Data.PlanID)
	}
	return ""
}

func fetchCommandCodeUsageData(ctx context.Context, apiKey string) (commandCodeUsageData, error) {
	var data commandCodeUsageData
	if err := fetchCommandCodeJSON(ctx, apiKey, "/alpha/whoami", nil, &data.Whoami); err != nil {
		return data, err
	}

	params := map[string]string{}
	if orgID := data.orgID(); orgID != "" {
		params["orgId"] = orgID
	}
	if err := fetchCommandCodeJSON(ctx, apiKey, "/alpha/billing/credits", params, &data.Credits); err != nil {
		return data, err
	}

	var subscription commandCodeSubscriptionResponse
	if err := fetchCommandCodeJSON(ctx, apiKey, "/alpha/billing/subscriptions", params, &subscription); err == nil {
		data.Subscription = &subscription
	}

	summaryParams := cloneStringMap(params)
	if subscription.Data != nil && strings.TrimSpace(subscription.Data.CurrentPeriodStart) != "" {
		summaryParams["since"] = strings.TrimSpace(subscription.Data.CurrentPeriodStart)
	}
	var summary commandCodeSummaryResponse
	if err := fetchCommandCodeJSON(ctx, apiKey, "/alpha/usage/summary", summaryParams, &summary); err == nil {
		data.Summary = &summary
	}

	return data, nil
}

func fetchCommandCodeJSON(ctx context.Context, apiKey, endpoint string, params map[string]string, target any) error {
	base := strings.TrimRight(resolveCommandCodeAPIBaseURL(), "/")
	endpoint = strings.TrimLeft(endpoint, "/")
	reqURL := base + "/" + endpoint
	if len(params) > 0 {
		values := url.Values{}
		for key, value := range params {
			if strings.TrimSpace(value) != "" {
				values.Set(key, value)
			}
		}
		if encoded := values.Encode(); encoded != "" {
			reqURL += "?" + encoded
		}
	}

	// 15s ceiling per call, derived from the caller's ctx; retries follow
	// the shared netclient policy.
	callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
		"Accept":        "application/json",
		"User-Agent":    "one-click-tools/1.0",
	}
	return netclient.DefaultClient.GetJSON(callCtx, reqURL, headers, target)
}

// describeCommandCodeCredential probes the Command Code key chain in fetch
// priority order: COMMAND_CODE_API_KEY -> ~/.commandcode/<auth file>.
func describeCommandCodeCredential() CredentialStatus {
	authPath := ""
	if home := credentialHomePath(".commandcode"); home != "" {
		authPath = filepath.Join(home, commandCodeAuthFileName())
	}
	status := credentialStatus([]CredentialSource{
		{
			Kind:     CredentialKindEnv,
			Location: "COMMAND_CODE_API_KEY",
			Found:    credentialEnvFound("COMMAND_CODE_API_KEY"),
		},
		{
			Kind:     CredentialKindFile,
			Location: authPath,
			Found:    authPath != "" && credentialJSONFileFound(authPath, "apiKey"),
			Note:     "file name follows COMMANDCODE_API_ENV (auth/auth.local/auth.staging)",
		},
	})
	if status.Status == CredentialStatusMissing {
		status.Note = "run 'commandcode login' or set COMMAND_CODE_API_KEY"
	}
	return status
}

func resolveCommandCodeAPIKey() (string, string) {
	if key := strings.TrimSpace(os.Getenv("COMMAND_CODE_API_KEY")); key != "" {
		return key, "env:COMMAND_CODE_API_KEY"
	}

	home, _ := userHomeDir()
	if strings.TrimSpace(home) == "" {
		return "", ""
	}

	authPath := filepath.Join(home, ".commandcode", commandCodeAuthFileName())
	data, err := os.ReadFile(authPath)
	if err != nil {
		return "", ""
	}
	var auth commandCodeAuth
	if err := json.Unmarshal(data, &auth); err != nil {
		return "", ""
	}
	if strings.TrimSpace(auth.APIKey) == "" {
		return "", ""
	}
	return strings.TrimSpace(auth.APIKey), commandCodeAuthFileName()
}

func commandCodeAuthFileName() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("COMMANDCODE_API_ENV"))) {
	case "local":
		return "auth.local.json"
	case "staging":
		return "auth.staging.json"
	default:
		return "auth.json"
	}
}

func resolveCommandCodeAPIBaseURL() string {
	if override := strings.TrimSpace(os.Getenv("OCT_COMMANDCODE_API_BASE_URL")); override != "" {
		return override
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("COMMANDCODE_SANDBOX")), "true") {
		if custom := strings.TrimSpace(os.Getenv("COMMANDCODE_API_URL")); custom != "" {
			return custom
		}
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("COMMANDCODE_API_ENV"))) {
	case "local":
		return "http://localhost:9090"
	case "staging":
		return "https://staging-api.commandcode.ai"
	default:
		return commandCodeAPIBaseURL
	}
}

func addCommandCodeWindow(buckets map[string]string, resets map[string]string, bucket string, window *commandCodeWindowLimit) {
	if window == nil || window.Cap <= 0 {
		return
	}
	usedPercent := (window.Used / window.Cap) * 100
	if usedPercent < 0 {
		usedPercent = 0
	}
	buckets[bucket] = fmt.Sprintf("%.1f", usedPercent)
	if window.ResetAt > 0 {
		resets[bucket] = fmt.Sprintf("%d", window.ResetAt)
	}
}

func commandCodeUsageWindowLimits(credits *commandCodeCreditsResponse) *commandCodeWindowLimits {
	if credits == nil {
		return nil
	}
	if credits.WindowLimits != nil {
		return credits.WindowLimits
	}
	if credits.Credits != nil {
		return credits.Credits.WindowLimits
	}
	return nil
}

func commandCodeMonthlyUsagePercent(planID string, monthlyRemaining, purchasedRemaining, freeRemaining float64, summary *commandCodeSummaryResponse) (float64, bool) {
	planTotal := commandCodePlanCredits(planID)
	totalRemaining := maxFloat(0, monthlyRemaining) + maxFloat(0, purchasedRemaining) + maxFloat(0, freeRemaining)
	if planTotal > 0 {
		totalPool := maxFloat(planTotal, monthlyRemaining) + maxFloat(0, purchasedRemaining) + maxFloat(0, freeRemaining)
		if totalPool <= 0 {
			return 0, false
		}
		return ClampPercent(((totalPool - totalRemaining) / totalPool) * 100), true
	}
	if summary != nil && summary.TotalCost > 0 {
		totalPool := summary.TotalCost + totalRemaining
		if totalPool <= 0 {
			return 0, false
		}
		return ClampPercent((summary.TotalCost / totalPool) * 100), true
	}
	if totalRemaining > 0 {
		return 0, true
	}
	return 0, false
}

func commandCodePlanCredits(planID string) float64 {
	switch normalizedCommandCodePlanID(planID) {
	case "individual-go":
		return 10
	case "individual-goat":
		return 70
	case "individual-pro":
		return 30
	case "individual-pro-v1":
		return 80
	case "individual-provider":
		return 15
	case "individual-max":
		return 150
	case "individual-ultra":
		return 300
	case "teams-pro":
		return 40
	default:
		return 0
	}
}

func commandCodePlanLabel(planID string) string {
	switch normalizedCommandCodePlanID(planID) {
	case "individual-go":
		return "Go"
	case "individual-goat":
		return "GOAT"
	case "individual-pro", "individual-pro-v1":
		return "Pro"
	case "individual-provider":
		return "Provider"
	case "individual-max":
		return "Max"
	case "individual-ultra":
		return "Ultra"
	case "teams-pro":
		return "Teams Pro"
	default:
		return strings.TrimSpace(planID)
	}
}

func normalizedCommandCodePlanID(planID string) string {
	planID = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(planID), "_", "-"))
	plans := []string{"individual-pro-v1", "individual-provider", "individual-ultra", "individual-goat", "individual-max", "individual-pro", "individual-go", "teams-pro"}
	for _, plan := range plans {
		if strings.HasPrefix(planID, plan) {
			return plan
		}
	}
	return planID
}

func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
