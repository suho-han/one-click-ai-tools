package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// isolateDoctorCredentialEnv gives the doctor a clean slate: an isolated
// home (via completion_test.go's isolateTestHome) and every credential env
// var blanked, so probes only see what the test writes.
func isolateDoctorCredentialEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	isolateTestHome(t, home)
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

// resetDoctorCredentialFlags clears bool flag values left by a previous
// in-process runCLI call: cobra's flag sets are package-level singletons and
// keep the last parsed value, unlike the one-shot real binary.
func resetDoctorCredentialFlags(t *testing.T) {
	t.Helper()
	for _, name := range []string{"json", "verbose"} {
		if f := doctorCredentialsCmd.Flags().Lookup(name); f != nil {
			if err := f.Value.Set("false"); err != nil {
				t.Fatalf("reset --%s: %v", name, err)
			}
			f.Changed = false
		}
	}
}

func runDoctorCredentialsJSON(t *testing.T, stdout, stderr *bytes.Buffer) map[string]any {
	t.Helper()
	cfgPath := writeTempConfig(t)
	viperResetForTest(t)
	resetDoctorCredentialFlags(t)
	if code := runCLI([]string{"--config", cfgPath, "doctor", "credentials", "--json"}, stdout, stderr); code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, stderr.String())
	}
	var report map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\nstdout: %s", err, stdout.String())
	}
	return report
}

func doctorReportProviders(t *testing.T, report map[string]any) map[string]map[string]any {
	t.Helper()
	entries, ok := report["providers"].([]any)
	if !ok {
		t.Fatalf("report has no providers array: %s", report)
	}
	byName := make(map[string]map[string]any, len(entries))
	for _, raw := range entries {
		entry, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("provider entry not an object: %v", raw)
		}
		name, _ := entry["provider"].(string)
		byName[name] = entry
	}
	return byName
}

func TestDoctorCredentialsCommandJSONStructure(t *testing.T) {
	isolateDoctorCredentialEnv(t)
	var stdout, stderr bytes.Buffer
	report := runDoctorCredentialsJSON(t, &stdout, &stderr)

	summary, ok := report["summary"].(map[string]any)
	if !ok {
		t.Fatalf("report has no summary object: %s", report)
	}
	for _, field := range []string{"total", "ok", "missing", "info"} {
		if _, ok := summary[field]; !ok {
			t.Fatalf("summary missing %q: %v", field, summary)
		}
	}
	providers := doctorReportProviders(t, report)
	if len(providers) == 0 {
		t.Fatalf("no providers reported: %s", report)
	}
	for _, name := range []string{"codex", "claude", "kimi", "deepseek", "openrouter", "qwen", "agy"} {
		if _, ok := providers[name]; !ok {
			t.Fatalf("provider %s missing from report: %v", name, providers)
		}
	}
	// qwen needs no credential and must not be reported as missing.
	if providers["qwen"]["status"] != "info" {
		t.Fatalf("qwen status = %v, want info", providers["qwen"]["status"])
	}
}

func TestDoctorCredentialsDetectsEnvCredential(t *testing.T) {
	isolateDoctorCredentialEnv(t)
	t.Setenv("DEEPSEEK_API_KEY", "doctor-test-key")

	var stdout, stderr bytes.Buffer
	report := runDoctorCredentialsJSON(t, &stdout, &stderr)
	providers := doctorReportProviders(t, report)

	entry := providers["deepseek"]
	if entry["status"] != "ok" {
		t.Fatalf("deepseek status = %v, want ok", entry["status"])
	}
	if entry["resolved"] != "env DEEPSEEK_API_KEY" {
		t.Fatalf("deepseek resolved = %v, want env source", entry["resolved"])
	}
}

func TestDoctorCredentialsDetectsCredentialFile(t *testing.T) {
	home := isolateDoctorCredentialEnv(t)
	kimiDir := filepath.Join(home, ".kimi-code", "credentials")
	if err := os.MkdirAll(kimiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kimiDir, "kimi-code.json"), []byte(`{"access_token":"tok"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	report := runDoctorCredentialsJSON(t, &stdout, &stderr)
	providers := doctorReportProviders(t, report)

	if providers["kimi"]["status"] != "ok" {
		t.Fatalf("kimi status = %v, want ok", providers["kimi"]["status"])
	}
	if resolved, _ := providers["kimi"]["resolved"].(string); !strings.Contains(resolved, "kimi-code.json") {
		t.Fatalf("kimi resolved = %v, want credential file", providers["kimi"]["resolved"])
	}
}

func TestDoctorCredentialsPlainOutput(t *testing.T) {
	isolateDoctorCredentialEnv(t)
	cfgPath := writeTempConfig(t)
	viperResetForTest(t)
	resetDoctorCredentialFlags(t)

	var stdout, stderr bytes.Buffer
	if code := runCLI([]string{"--config", cfgPath, "doctor", "credentials"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.HasPrefix(out, "credentials doctor: ok=") {
		t.Fatalf("output = %q, want credentials doctor header", out)
	}
	if !strings.Contains(out, "- kimi") || !strings.Contains(out, "- codex") {
		t.Fatalf("output missing provider lines: %q", out)
	}

	stdout.Reset()
	if code := runCLI([]string{"--config", cfgPath, "doctor", "credentials", "--verbose"}, &stdout, &stderr); code != 0 {
		t.Fatalf("verbose exit = %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "[x] env") && !strings.Contains(stdout.String(), "[ ] env") {
		t.Fatalf("verbose output missing source markers: %q", stdout.String())
	}
}
