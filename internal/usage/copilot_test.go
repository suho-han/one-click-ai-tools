package usage

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchCopilotQuotaUsageMapsAICBudget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("Authorization header did not contain test bearer token")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"login":"octocat",
			"copilot_plan":"individual",
			"token_based_billing":true,
			"quota_snapshots":{
				"premium_interactions":{
					"entitlement":200,
					"percent_remaining":41.7
				}
			}
		}`)
	}))
	defer server.Close()
	t.Setenv("OCT_COPILOT_USER_ENDPOINT", server.URL)

	result, ok := fetchCopilotQuotaUsage(UsageResult{Provider: "copilot"}, "test-token")
	if !ok {
		t.Fatal("expected quota usage result")
	}
	if result.Source != "quota" {
		t.Fatalf("expected quota source, got %q", result.Source)
	}
	if result.Plan != "individual" {
		t.Fatalf("expected individual plan, got %q", result.Plan)
	}
	if result.Used != "117" {
		t.Fatalf("expected rounded 117 AIC used, got %q", result.Used)
	}
	if result.Limit != "200" {
		t.Fatalf("expected 200 AIC limit, got %q", result.Limit)
	}
	if result.Unit != "AIC" {
		t.Fatalf("expected AIC unit, got %q", result.Unit)
	}
	if result.Buckets["quota"] != "58.3" {
		t.Fatalf("expected quota bucket 58.3, got %q", result.Buckets["quota"])
	}
}
