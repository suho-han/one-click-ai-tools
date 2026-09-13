package usage

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const zaiQuotaPayload = `{
  "data": {
    "limits": [
      {"type": "TOKENS_LIMIT_5H", "percentage": 42},
      {"type": "TOKENS_LIMIT_WEEKLY", "percentage": 13},
      {"type": "TIME_LIMIT_MONTHLY", "percentage": 7}
    ]
  }
}`

func TestFetchZaiUsageMapsLimits(t *testing.T) {
	t.Setenv("ZAI_API_KEY", "tok-123")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "")
	t.Setenv("ZHIPU_API_KEY", "")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "tok-123" {
			t.Errorf("Authorization = %q, want raw token without Bearer prefix", got)
		}
		w.Write([]byte(zaiQuotaPayload))
	}))
	defer server.Close()

	t.Setenv("OCT_ZAI_USAGE_ENDPOINT", server.URL)
	result := FetchZaiUsage(t.Context())

	if result.Status != "ok" {
		t.Fatalf("status = %q, message = %q", result.Status, result.Message)
	}
	if result.Buckets["5h"] != "42" {
		t.Errorf("5h bucket = %q, want 42", result.Buckets["5h"])
	}
	if result.Buckets["7d"] != "13" {
		t.Errorf("7d bucket = %q, want 13", result.Buckets["7d"])
	}
	if result.Buckets["1m"] != "7" {
		t.Errorf("1m bucket = %q, want 7", result.Buckets["1m"])
	}
	if result.Used != "42" {
		t.Errorf("used = %q, want 42", result.Used)
	}
	if result.Unit != "percent" {
		t.Errorf("unit = %q, want percent", result.Unit)
	}
}

func TestResolveZaiCredentialPriority(t *testing.T) {
	t.Setenv("ANTHROPIC_BASE_URL", "")

	t.Setenv("ZAI_API_KEY", "zai-key")
	t.Setenv("ZHIPU_API_KEY", "zhipu-key")
	token, base, _ := resolveZaiCredential()
	if token != "zai-key" || base != zaiGlobalBaseURL {
		t.Fatalf("ZAI priority: token/base = %q/%q", token, base)
	}

	os.Unsetenv("ZAI_API_KEY")
	token, base, _ = resolveZaiCredential()
	if token != "zhipu-key" || base != zaiChinaBaseURL {
		t.Fatalf("ZHIPU fallback: token/base = %q/%q", token, base)
	}
}

func TestResolveZaiCredentialAnthropicTokenOnlyForZaiHosts(t *testing.T) {
	t.Setenv("ZAI_API_KEY", "")
	t.Setenv("ZHIPU_API_KEY", "")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "anthropic-token")

	t.Setenv("ANTHROPIC_BASE_URL", "https://api.anthropic.com")
	if token, _, _ := resolveZaiCredential(); token != "" {
		t.Fatalf("anthropic host must not yield a Z.ai token, got %q", token)
	}

	t.Setenv("ANTHROPIC_BASE_URL", "https://api.z.ai/api/anthropic")
	token, base, _ := resolveZaiCredential()
	if token != "anthropic-token" || base != zaiGlobalBaseURL {
		t.Fatalf("z.ai host: token/base = %q/%q", token, base)
	}

	t.Setenv("ANTHROPIC_BASE_URL", "https://open.bigmodel.cn/api/anthropic")
	token, base, _ = resolveZaiCredential()
	if token != "anthropic-token" || base != zaiChinaBaseURL {
		t.Fatalf("bigmodel host: token/base = %q/%q", token, base)
	}
}

func TestResolveZaiCredentialFromOpenCodeAuth(t *testing.T) {
	t.Setenv("ZAI_API_KEY", "")
	t.Setenv("ZHIPU_API_KEY", "")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "")
	t.Setenv("ANTHROPIC_BASE_URL", "")

	home := t.TempDir()
	origHome := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = origHome })

	authDir := filepath.Join(home, ".local", "share", "opencode")
	if err := os.MkdirAll(authDir, 0o755); err != nil {
		t.Fatal(err)
	}
	authPath := filepath.Join(authDir, "auth.json")
	if err := os.WriteFile(authPath, []byte(`{"zai-coding-plan":{"type":"api","key":"oc-zai-token"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	token, base, source := resolveZaiCredential()
	if token != "oc-zai-token" || base != zaiGlobalBaseURL || source != "opencode:auth.json" {
		t.Fatalf("token/base/source = %q/%q/%q", token, base, source)
	}

	if err := os.WriteFile(authPath, []byte(`{"zhipu":{"type":"api","key":"oc-zhipu-token"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	token, base, _ = resolveZaiCredential()
	if token != "oc-zhipu-token" || base != zaiChinaBaseURL {
		t.Fatalf("zhipu entry: token/base = %q/%q", token, base)
	}
}

func TestFetchZaiUsageWithoutToken(t *testing.T) {
	t.Setenv("ZAI_API_KEY", "")
	t.Setenv("ZHIPU_API_KEY", "")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "")
	origHome := userHomeDir
	userHomeDir = func() (string, error) { return t.TempDir(), nil }
	t.Cleanup(func() { userHomeDir = origHome })

	result := FetchZaiUsage(t.Context())
	if result.Status != "warn" {
		t.Fatalf("status = %q, want warn", result.Status)
	}
	if !strings.Contains(result.Message, "ZAI_API_KEY") {
		t.Errorf("message = %q, want key guidance", result.Message)
	}
}
