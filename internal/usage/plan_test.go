package usage

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/suho-han/one-click-tools/internal/netclient"
)

func TestDetectPlanFromJWTToken_OpenAIPlanClaim(t *testing.T) {
	payload := map[string]any{
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_plan_type": "plus",
		},
	}
	token := makeJWT(t, payload)
	if got := detectPlanFromJWTToken(token); got != "plus" {
		t.Fatalf("detectPlanFromJWTToken() = %q, want plus", got)
	}
}

func TestParseCursorAboutPlan(t *testing.T) {
	input := strings.Join([]string{
		"Cursor Agent",
		"Version             1.2.3",
		"Subscription Tier   Pro",
	}, "\n")
	if got := parseCursorAboutPlan(input); got != "pro" {
		t.Fatalf("parseCursorAboutPlan() = %q, want pro", got)
	}
}

func TestDetectClaudeLocalConfigPlan_UsesHeuristicFlag(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	path := filepath.Join(home, ".claude.json")
	if err := os.WriteFile(path, []byte(`{"opusProMigrationComplete":true}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	plan, source := detectClaudeLocalConfigPlan()
	if plan != "pro" {
		t.Fatalf("plan = %q, want pro", plan)
	}
	if !strings.Contains(source, "opusProMigrationComplete") {
		t.Fatalf("source = %q", source)
	}
}

func TestDetectCopilotBillingPlanSource_Reports404AsNoPublicPlanField(t *testing.T) {
	origOutput := usageCommandOutput
	origClient := netclient.DefaultClient
	defer func() {
		usageCommandOutput = origOutput
		netclient.DefaultClient = origClient
	}()
	usageCommandOutput = func(timeout time.Duration, name string, args ...string) (string, error) {
		return "test-token", nil
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"login":"suho-han"}`))
		case "/users/suho-han/settings/billing/premium_request/usage":
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	netclient.DefaultClient = &netclient.Client{HTTPClient: server.Client(), MaxRetries: 0}
	origTransport := server.Client().Transport
	server.Client().Transport = rewriteHostTransport{base: origTransport, target: server.URL}
	if netclient.DefaultClient.HTTPClient == nil {
		t.Fatal("missing http client")
	}
	netclient.DefaultClient.HTTPClient.Transport = rewriteHostTransport{base: origTransport, target: server.URL}
	source := detectCopilotBillingPlanSource()
	if !strings.Contains(source, "404") || !strings.Contains(source, "no public plan field") {
		t.Fatalf("source = %q", source)
	}
}

func makeJWT(t *testing.T, payload map[string]any) string {
	t.Helper()
	headerBytes, err := json.Marshal(map[string]any{"alg": "none", "typ": "JWT"})
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	header := base64.RawURLEncoding.EncodeToString(headerBytes)
	body := base64.RawURLEncoding.EncodeToString(payloadBytes)
	return header + "." + body + ".sig"
}

type rewriteHostTransport struct {
	base   http.RoundTripper
	target string
}

func (t rewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = "http"
	clone.URL.Host = strings.TrimPrefix(t.target, "http://")
	clone.Host = clone.URL.Host
	return t.base.RoundTrip(clone)
}
