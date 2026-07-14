package sessionrefresh

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/suho-han/one-click-ai-tools/internal/update"
)

func TestRefreshUnknownProvider(t *testing.T) {
	results := Refresh(RefreshOptions{Providers: []string{"nope"}, DryRun: true})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "unsupported" {
		t.Fatalf("expected unsupported, got %#v", results[0])
	}
}

func TestRefreshAliasMapsGeminiToAntigravity(t *testing.T) {
	results := Refresh(RefreshOptions{Providers: []string{"gemini"}, DryRun: true})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Provider != "agy" {
		t.Fatalf("expected gemini alias to resolve to agy, got %#v", results[0])
	}
}

func TestResolveToolMatchesAlias(t *testing.T) {
	tool, ok := resolveTool(update.NormalizeToolName("agent"))
	if !ok {
		t.Fatal("expected to resolve Cursor tool from agent alias")
	}
	if tool.BinaryName != "cursor-agent" {
		t.Fatalf("expected cursor-agent, got %q", tool.BinaryName)
	}
}

func TestFirstNonEmptyLine(t *testing.T) {
	if got := firstNonEmptyLine("\n hello\nworld"); got != "hello" {
		t.Fatalf("unexpected first non-empty line: %q", got)
	}
}

func TestProbeCodexSessionUsesBootstrappedPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is unix-only")
	}

	home := t.TempDir()
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	codexPath := filepath.Join(binDir, "codex")
	if err := os.WriteFile(codexPath, []byte("#!/bin/sh\necho Logged in\n"), 0o755); err != nil {
		t.Fatalf("write codex fixture failed: %v", err)
	}

	t.Setenv("HOME", home)
	t.Setenv("PATH", "/usr/bin:/bin")

	result := probeCodexSession(RefreshOptions{}, update.Tool{BinaryName: "codex"})
	if result.Status != "ok" {
		t.Fatalf("expected ok result via bootstrapped PATH, got %#v", result)
	}
	if result.Confidence != confidenceVerified {
		t.Fatalf("expected verified confidence, got %#v", result)
	}
	if result.Message != "Logged in" {
		t.Fatalf("unexpected codex probe message: %q", result.Message)
	}
}

func TestFindFirstBinaryUsesBootstrappedPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is unix-only")
	}

	home := t.TempDir()
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	agyPath := filepath.Join(binDir, "agy")
	if err := os.WriteFile(agyPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write agy fixture failed: %v", err)
	}

	t.Setenv("HOME", home)
	t.Setenv("PATH", "/usr/bin:/bin")

	got, err := findFirstBinary("agy")
	if err != nil {
		t.Fatalf("expected agy to resolve via bootstrapped PATH: %v", err)
	}
	if got != agyPath {
		t.Fatalf("unexpected agy path: got %q want %q", got, agyPath)
	}
}

func TestProbeClaudeSessionReportsLoggedOutState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is unix-only")
	}

	home := t.TempDir()
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	claudePath := filepath.Join(binDir, "claude")
	if err := os.WriteFile(claudePath, []byte("#!/bin/sh\nprintf '{\"loggedIn\":false,\"authMethod\":\"none\",\"apiProvider\":\"firstParty\"}'\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write claude fixture failed: %v", err)
	}

	t.Setenv("HOME", home)
	t.Setenv("PATH", "/usr/bin:/bin")

	result := probeClaudeSession(RefreshOptions{}, update.Tool{BinaryName: "claude"})
	if result.Status != "skipped" {
		t.Fatalf("expected logged-out claude status to be skipped, got %#v", result)
	}
	if result.Mode != "auth-status" {
		t.Fatalf("unexpected mode: %#v", result)
	}
	if result.Confidence != confidenceVerified {
		t.Fatalf("expected verified confidence, got %#v", result)
	}
	if !strings.Contains(strings.ToLower(result.Message), "not logged in") {
		t.Fatalf("expected logged-out message, got %q", result.Message)
	}
}

func TestProbeOpenCodeSessionDetectsCredentialInventory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is unix-only")
	}

	home := t.TempDir()
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	opencodePath := filepath.Join(binDir, "opencode")
	script := "#!/bin/sh\ncat <<'EOF'\n┌  Credentials ~/.local/share/opencode/auth.json\n│\n│  1 credentials\n└\nEOF\n"
	if err := os.WriteFile(opencodePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write opencode fixture failed: %v", err)
	}

	t.Setenv("HOME", home)
	t.Setenv("PATH", "/usr/bin:/bin")

	result := probeOpenCodeSession(RefreshOptions{}, update.Tool{BinaryName: "opencode"})
	if result.Status != "ok" {
		t.Fatalf("expected ok result for detected OpenCode credentials, got %#v", result)
	}
	if result.Confidence != confidenceVerified {
		t.Fatalf("expected verified confidence, got %#v", result)
	}
	if result.Mode != "providers-list" {
		t.Fatalf("unexpected mode: %#v", result)
	}
}

func TestProbeOpenCodeSessionMarksEnvironmentHintsAsPartial(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is unix-only")
	}

	home := t.TempDir()
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	opencodePath := filepath.Join(binDir, "opencode")
	script := "#!/bin/sh\ncat <<'EOF'\nOpenAI (environment)\nAnthropic (environment)\nEOF\n"
	if err := os.WriteFile(opencodePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write opencode fixture failed: %v", err)
	}

	t.Setenv("HOME", home)
	t.Setenv("PATH", "/usr/bin:/bin")

	result := probeOpenCodeSession(RefreshOptions{}, update.Tool{BinaryName: "opencode"})
	if result.Status != "skipped" {
		t.Fatalf("expected skipped result for environment-only OpenCode hints, got %#v", result)
	}
	if result.Confidence != confidencePartial {
		t.Fatalf("expected partial confidence, got %#v", result)
	}
	if result.Mode != "providers-list" {
		t.Fatalf("unexpected mode: %#v", result)
	}
	if !strings.Contains(strings.ToLower(result.Message), "environment credential hints") {
		t.Fatalf("unexpected message: %q", result.Message)
	}
}

func TestProbeCopilotSessionReportsPartialGithubAuth(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fixture is unix-only")
	}

	home := t.TempDir()
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	ghPath := filepath.Join(binDir, "gh")
	ghScript := "#!/bin/sh\nif [ \"$1\" = \"auth\" ] && [ \"$2\" = \"status\" ]; then\n  echo 'github.com'\n  echo '  ✓ Logged in to github.com account tester'\n  exit 0\nfi\nexit 1\n"
	if err := os.WriteFile(ghPath, []byte(ghScript), 0o755); err != nil {
		t.Fatalf("write gh fixture failed: %v", err)
	}
	copilotPath := filepath.Join(binDir, "copilot")
	if err := os.WriteFile(copilotPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write copilot fixture failed: %v", err)
	}

	t.Setenv("HOME", home)
	t.Setenv("PATH", "/usr/bin:/bin")
	t.Setenv("GH_TOKEN", "test-gh-token")

	result := probeCopilotSession(RefreshOptions{}, update.Tool{BinaryName: "copilot"})
	if result.Status != "skipped" {
		t.Fatalf("expected partial Copilot probe to be skipped, got %#v", result)
	}
	if result.Confidence != confidencePartial {
		t.Fatalf("expected partial confidence, got %#v", result)
	}
	if result.Mode != "partial-auth" {
		t.Fatalf("unexpected mode: %#v", result)
	}
	if !strings.Contains(strings.ToLower(result.Message), "github auth detected") {
		t.Fatalf("unexpected partial probe message: %q", result.Message)
	}
}
