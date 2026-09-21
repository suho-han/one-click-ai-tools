package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

// stubCredentialCommands replaces the command/binary probes with canned
// answers so tests never touch the real keychain, gh, or PATH.
func stubCredentialCommands(t *testing.T, commands map[string]bool, binaries map[string]bool) {
	t.Helper()
	origPresent := credentialCommandPresent
	origBinary := credentialBinaryPresent
	credentialCommandPresent = func(name string, args ...string) bool {
		return commands[name]
	}
	credentialBinaryPresent = func(name string) bool {
		return binaries[name]
	}
	t.Cleanup(func() {
		credentialCommandPresent = origPresent
		credentialBinaryPresent = origBinary
	})
}

// isolateCredentialHome points HOME/USERPROFILE at a fresh temp dir and
// blanks every credential env var the probes consult, so a test only sees
// what it writes itself.
func isolateCredentialHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	for _, key := range []string{
		"KIMI_CODE_API_KEY", "KIMI_CODE_HOME",
		"CLAUDE_API_TOKEN", "CODEX_HOME",
		"ZAI_API_KEY", "ZHIPU_API_KEY", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_BASE_URL",
		"GROK_OAUTH_TOKEN", "GROK_HOME", "QWEN_HOME",
		"MINIMAX_CODING_API_KEY", "MINIMAX_API_KEY", "MINIMAX_REGION",
		"DEEPSEEK_API_KEY", "OPENROUTER_API_KEY", "OPENCODE_API_KEY",
		"COMMAND_CODE_API_KEY", "COMMANDCODE_API_ENV",
		"CURSOR_API_KEY", "OCT_CURSOR_USAGE_URL", "GITHUB_API_TOKEN", "GITHUB_TOKEN",
	} {
		t.Setenv(key, "")
	}
	return home
}

func TestDescribeProviderCredentialsUnknownName(t *testing.T) {
	if _, ok := DescribeProviderCredentials("nope"); ok {
		t.Fatalf("unknown provider reported as known")
	}
	if _, ok := DescribeProviderCredentials("  "); ok {
		t.Fatalf("blank name reported as known")
	}
}

func TestDescribeKimiCredential(t *testing.T) {
	isolateCredentialHome(t)
	stubCredentialCommands(t, nil, nil)

	t.Run("env key wins", func(t *testing.T) {
		t.Setenv("KIMI_CODE_API_KEY", "kimi-key")
		status, ok := DescribeProviderCredentials("kimi")
		if !ok || status.Status != CredentialStatusOK {
			t.Fatalf("status = %q ok=%v, want ok", status.Status, ok)
		}
		if status.Resolved != "env KIMI_CODE_API_KEY" {
			t.Fatalf("resolved = %q, want env key", status.Resolved)
		}
	})

	t.Run("static credential file", func(t *testing.T) {
		home, _ := os.UserHomeDir()
		credDir := filepath.Join(home, ".kimi-code", "credentials")
		if err := os.MkdirAll(credDir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(credDir, "kimi-code.json"), `{"access_token":"tok"}`)
		status, _ := DescribeProviderCredentials("kimi")
		if status.Status != CredentialStatusOK || status.Resolved != "file "+filepath.Join(credDir, "kimi-code.json") {
			t.Fatalf("status=%q resolved=%q", status.Status, status.Resolved)
		}
	})

	t.Run("rotated env file with expiry note", func(t *testing.T) {
		home, _ := os.UserHomeDir()
		credDir := filepath.Join(home, ".kimi-code", "credentials")
		if err := os.MkdirAll(credDir, 0o755); err != nil {
			t.Fatal(err)
		}
		expired := `{"access_token":"stale","expires_at":` + itoa64(time.Now().Unix()-100) + `}`
		writeFile(t, filepath.Join(credDir, "kimi-code-env-1.json"), expired)
		status, _ := DescribeProviderCredentials("kimi")
		if status.Status != CredentialStatusOK {
			t.Fatalf("status = %q, want ok", status.Status)
		}
		rotated := status.Sources[len(status.Sources)-1]
		if !rotated.Found || !strings.Contains(rotated.Note, "expired") {
			t.Fatalf("rotated source = %+v, want found with expiry note", rotated)
		}
	})

	t.Run("missing", func(t *testing.T) {
		// isolateCredentialHome already blanked everything; use a clean home.
		isolateCredentialHome(t)
		status, _ := DescribeProviderCredentials("kimi")
		if status.Status != CredentialStatusMissing {
			t.Fatalf("status = %q, want missing", status.Status)
		}
		if !strings.Contains(status.Note, "kimi login") {
			t.Fatalf("note = %q, want login hint", status.Note)
		}
	})
}

func TestDescribeSingleEnvProviders(t *testing.T) {
	isolateCredentialHome(t)

	for _, tc := range []struct{ provider, env string }{
		{"deepseek", "DEEPSEEK_API_KEY"},
		{"openrouter", "OPENROUTER_API_KEY"},
	} {
		t.Run(tc.provider+" missing", func(t *testing.T) {
			status, _ := DescribeProviderCredentials(tc.provider)
			if status.Status != CredentialStatusMissing {
				t.Fatalf("status = %q, want missing", status.Status)
			}
		})
		t.Run(tc.provider+" env", func(t *testing.T) {
			t.Setenv(tc.env, "key")
			status, _ := DescribeProviderCredentials(tc.provider)
			if status.Status != CredentialStatusOK || status.Resolved != "env "+tc.env {
				t.Fatalf("status=%q resolved=%q", status.Status, status.Resolved)
			}
		})
	}
}

func TestDescribeMinimaxCredential(t *testing.T) {
	isolateCredentialHome(t)
	status, _ := DescribeProviderCredentials("minimax")
	if status.Status != CredentialStatusMissing {
		t.Fatalf("status = %q, want missing", status.Status)
	}
	if len(status.Sources) != 2 {
		t.Fatalf("sources = %d, want 2 before REGION is set", len(status.Sources))
	}

	t.Setenv("MINIMAX_API_KEY", "key")
	t.Setenv("MINIMAX_REGION", "cn")
	status, _ = DescribeProviderCredentials("minimax")
	if status.Status != CredentialStatusOK {
		t.Fatalf("status = %q, want ok", status.Status)
	}
	if len(status.Sources) != 3 {
		t.Fatalf("sources = %d, want 3 with REGION set", len(status.Sources))
	}
}

func TestDescribeCodexCredential(t *testing.T) {
	home := isolateCredentialHome(t)
	stubCredentialCommands(t, nil, nil)

	codexHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(codexHome, "auth.json"), `{"tokens":{"access_token":"tok","account_id":"acc"}}`)

	status, _ := DescribeProviderCredentials("codex")
	if status.Status != CredentialStatusOK {
		t.Fatalf("status = %q, want ok", status.Status)
	}
	if !strings.HasSuffix(status.Resolved, filepath.Join(".codex", "auth.json")) {
		t.Fatalf("resolved = %q, want auth.json path", status.Resolved)
	}

	// CODEX_HOME relocation is honored.
	alt := filepath.Join(t.TempDir(), "alt-codex")
	if err := os.MkdirAll(alt, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", alt)
	status, _ = DescribeProviderCredentials("codex")
	if status.Status != CredentialStatusMissing {
		t.Fatalf("status after CODEX_HOME move = %q, want missing", status.Status)
	}
}

func TestDescribeZaiCredential(t *testing.T) {
	isolateCredentialHome(t)
	stubCredentialCommands(t, nil, nil)

	t.Run("anthropic token gated on base url", func(t *testing.T) {
		t.Setenv("ANTHROPIC_AUTH_TOKEN", "tok")
		status, _ := DescribeProviderCredentials("zai")
		if status.Status != CredentialStatusMissing {
			t.Fatalf("status without base url = %q, want missing", status.Status)
		}
		t.Setenv("ANTHROPIC_BASE_URL", "https://api.z.ai/api/anthropic")
		status, _ = DescribeProviderCredentials("zai")
		if status.Status != CredentialStatusOK || status.Resolved != "env ANTHROPIC_AUTH_TOKEN" {
			t.Fatalf("status=%q resolved=%q, want anthropic env", status.Status, status.Resolved)
		}
	})

	t.Run("opencode auth entry", func(t *testing.T) {
		home, _ := os.UserHomeDir()
		authPath := filepath.Join(home, ".local", "share", "opencode")
		if err := os.MkdirAll(authPath, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(authPath, "auth.json"), `{"zai-coding-plan":{"type":"api","key":"zk"}}`)
		status, _ := DescribeProviderCredentials("zai")
		if status.Status != CredentialStatusOK || !strings.Contains(status.Resolved, "auth.json") {
			t.Fatalf("status=%q resolved=%q", status.Status, status.Resolved)
		}
	})
}

func TestDescribeGrokCredential(t *testing.T) {
	isolateCredentialHome(t)
	grokHome := t.TempDir()
	t.Setenv("GROK_HOME", grokHome)
	writeFile(t, filepath.Join(grokHome, "auth.json"), `{"auth.x.ai:oauth":{"key":"gk"}}`)

	status, _ := DescribeProviderCredentials("grok")
	if status.Status != CredentialStatusOK || !strings.Contains(status.Resolved, "auth.json") {
		t.Fatalf("status=%q resolved=%q", status.Status, status.Resolved)
	}
}

func TestDescribeClaudeCredential(t *testing.T) {
	home := isolateCredentialHome(t)
	stubCredentialCommands(t, map[string]bool{"security": false}, map[string]bool{"claude": true})

	credDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(credDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(credDir, ".credentials.json"), `{"access_token":"tok"}`)

	status, _ := DescribeProviderCredentials("claude")
	if status.Status != CredentialStatusOK || !strings.Contains(status.Resolved, ".credentials.json") {
		t.Fatalf("status=%q resolved=%q", status.Status, status.Resolved)
	}
	if len(status.Sources) < 3 {
		t.Fatalf("sources = %d, want keychain+file+env at minimum", len(status.Sources))
	}
}

func TestDescribeCopilotCredential(t *testing.T) {
	isolateCredentialHome(t)
	stubCredentialCommands(t, nil, nil)
	viper.Set("github_api_token", "cfg-token")
	t.Cleanup(func() { viper.Set("github_api_token", "") })

	status, _ := DescribeProviderCredentials("copilot")
	if status.Status != CredentialStatusOK || status.Resolved != "config github_api_token" {
		t.Fatalf("status=%q resolved=%q, want config win", status.Status, status.Resolved)
	}

	viper.Set("github_api_token", "")
	t.Setenv("GITHUB_TOKEN", "env-token")
	status, _ = DescribeProviderCredentials("copilot")
	if status.Resolved != "env GITHUB_TOKEN" {
		t.Fatalf("resolved = %q, want env fallback", status.Resolved)
	}
}

func TestDescribeCursorCredential(t *testing.T) {
	home := isolateCredentialHome(t)
	stubCredentialCommands(t, nil, nil)

	authDir := filepath.Join(home, ".config", "cursor")
	if err := os.MkdirAll(authDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(authDir, "auth.json"), `{"accessToken":"ctok"}`)

	status, _ := DescribeProviderCredentials("cursor-agent")
	if status.Status != CredentialStatusOK || !strings.Contains(status.Resolved, "auth.json") {
		t.Fatalf("status=%q resolved=%q", status.Status, status.Resolved)
	}

	t.Setenv("OCT_CURSOR_USAGE_URL", "https://example.com/usage")
	status, _ = DescribeProviderCredentials("cursor-agent")
	if status.Resolved != "env OCT_CURSOR_USAGE_URL" {
		t.Fatalf("resolved = %q, want custom endpoint to win", status.Resolved)
	}
}

func TestDescribeCommandCodeCredential(t *testing.T) {
	home := isolateCredentialHome(t)
	credDir := filepath.Join(home, ".commandcode")
	if err := os.MkdirAll(credDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(credDir, "auth.json"), `{"apiKey":"ck"}`)

	status, _ := DescribeProviderCredentials("commandcode")
	if status.Status != CredentialStatusOK || !strings.HasSuffix(status.Resolved, "auth.json") {
		t.Fatalf("status=%q resolved=%q", status.Status, status.Resolved)
	}

	t.Setenv("COMMANDCODE_API_ENV", "local")
	status, _ = DescribeProviderCredentials("commandcode")
	if status.Status != CredentialStatusMissing {
		t.Fatalf("status with auth.local.json absent = %q, want missing", status.Status)
	}
}

func TestDescribeOpenCodeCredential(t *testing.T) {
	home := isolateCredentialHome(t)
	authDir := filepath.Join(home, ".local", "share", "opencode")
	if err := os.MkdirAll(authDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(authDir, "auth.json"), `{"opencode-go":{"type":"api","key":"ok"}}`)

	status, _ := DescribeProviderCredentials("opencode")
	if status.Status != CredentialStatusOK || !strings.Contains(status.Resolved, "auth.json") {
		t.Fatalf("status=%q resolved=%q", status.Status, status.Resolved)
	}
}

func TestDescribeAntigravityCredential(t *testing.T) {
	isolateCredentialHome(t)
	stubCredentialCommands(t, nil, map[string]bool{"agy": true})

	status, _ := DescribeProviderCredentials("agy")
	if status.Status != CredentialStatusOK || !strings.Contains(status.Resolved, "agy") {
		t.Fatalf("status=%q resolved=%q", status.Status, status.Resolved)
	}

	stubCredentialCommands(t, nil, nil)
	status, _ = DescribeProviderCredentials("agy")
	if status.Status != CredentialStatusMissing || status.Note == "" {
		t.Fatalf("status=%q note=%q, want missing with hint", status.Status, status.Note)
	}
}

func TestDescribeQwenCredentialIsInfo(t *testing.T) {
	isolateCredentialHome(t)
	status, _ := DescribeProviderCredentials("qwen")
	if status.Status != CredentialStatusInfo {
		t.Fatalf("status = %q, want info (local-only provider)", status.Status)
	}
}

func TestCredentialReportNeverContainsSecretValues(t *testing.T) {
	home := isolateCredentialHome(t)
	stubCredentialCommands(t, nil, nil)

	credDir := filepath.Join(home, ".kimi-code", "credentials")
	if err := os.MkdirAll(credDir, 0o755); err != nil {
		t.Fatal(err)
	}
	secret := "SUPER-SECRET-KIMI-TOKEN-VALUE"
	writeFile(t, filepath.Join(credDir, "kimi-code.json"), `{"access_token":"`+secret+`"}`)
	t.Setenv("DEEPSEEK_API_KEY", secret)

	for _, provider := range []string{"kimi", "deepseek", "codex", "claude"} {
		status, ok := DescribeProviderCredentials(provider)
		if !ok {
			t.Fatalf("%s: probe missing", provider)
		}
		blob, err := json.Marshal(status)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(blob), secret) {
			t.Fatalf("%s: report leaked the secret value: %s", provider, blob)
		}
	}
}

func TestCredentialProviderNamesCoversRegistry(t *testing.T) {
	names := CredentialProviderNames()
	if len(names) != 14 {
		t.Fatalf("names = %d (%v), want 14", len(names), names)
	}
	for _, name := range names {
		if _, ok := DescribeProviderCredentials(name); !ok {
			t.Fatalf("provider %s has no probe", name)
		}
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func itoa64(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}
