package usage

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/suho-han/one-click-ai-tools/internal/netclient"
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

	result, ok := fetchCopilotQuotaUsage(t.Context(), UsageResult{Provider: "copilot"}, "test-token")
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

func TestFetchCopilotUsageQuotaPathMakesSingleRequest(t *testing.T) {
	var mu sync.Mutex
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"login":"octocat",
			"copilot_plan":"individual",
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
	t.Setenv("GITHUB_API_TOKEN", "test-token")

	result := FetchCopilotUsage(t.Context())

	mu.Lock()
	got := requests
	mu.Unlock()
	if got != 1 {
		t.Fatalf("expected exactly 1 HTTP request on the quota path, got %d", got)
	}
	if result.Plan != "individual" {
		t.Fatalf("expected individual plan from quota payload, got %q", result.Plan)
	}
	if result.Source != "quota" {
		t.Fatalf("expected quota source, got %q", result.Source)
	}
}

func TestFetchCopilotUsageBillingFallbackSkipsDuplicatePlanProbe(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		switch r.URL.Path {
		case "/copilot_internal/user":
			http.Error(w, `{"message":"boom"}`, http.StatusInternalServerError)
		case "/users/octocat/settings/billing/premium_request/usage":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"usage_items":[{"product":"copilot","net_quantity":12.5,"unit_type":"requests"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("OCT_COPILOT_USER_ENDPOINT", server.URL+"/copilot_internal/user")
	t.Setenv("GITHUB_API_TOKEN", "test-token")
	t.Setenv("GITHUB_USER", "octocat")

	origClient := netclient.DefaultClient
	defer func() { netclient.DefaultClient = origClient }()
	netclient.DefaultClient = &netclient.Client{HTTPClient: server.Client(), MaxRetries: 0}
	origTransport := server.Client().Transport
	server.Client().Transport = rewriteHostTransport{base: origTransport, target: server.URL}

	result := FetchCopilotUsage(t.Context())

	mu.Lock()
	got := append([]string(nil), paths...)
	mu.Unlock()
	want := []string{"/copilot_internal/user", "/users/octocat/settings/billing/premium_request/usage"}
	if len(got) != len(want) {
		t.Fatalf("expected requests %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected requests %v, got %v", want, got)
		}
	}
	if result.Used != "12.50" {
		t.Fatalf("expected billing usage 12.50, got %q", result.Used)
	}
	if result.PlanSource != "github billing api exposes usage only, not plan" {
		t.Fatalf("unexpected plan source: %q", result.PlanSource)
	}
}
