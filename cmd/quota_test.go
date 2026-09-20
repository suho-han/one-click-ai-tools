package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQuotaStats_AggregatesLocalSessionsAndSimulatesCosts(t *testing.T) {
	home := t.TempDir()
	isolateTestHome(t, home)
	cfgPath := writeTempConfig(t)
	viperResetForTest(t)

	codexDir := filepath.Join(home, ".codex", "sessions")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	codexLog := `{"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":2000000,"cache_read_input_tokens":40000000,"output_tokens":100000}}}}
`
	if err := os.WriteFile(filepath.Join(codexDir, "session-a.jsonl"), []byte(codexLog), 0o644); err != nil {
		t.Fatalf("write codex log: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := runCLI([]string{"--config", cfgPath, "quota", "stats", "--days", "30"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr.String())
	}

	out := stdout.String()
	for _, want := range []string{
		"[Local Session Stats - Last 30 Days]",
		"Cache Hit Rate:",
		"[Estimated Cost Comparison]",
		"MiMo-V2.5",
		"recommended",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q, got:\n%s", want, out)
		}
	}
}

func TestQuotaStats_ReportsMissingLogsFriendly(t *testing.T) {
	isolateTestHome(t, t.TempDir())
	cfgPath := writeTempConfig(t)
	viperResetForTest(t)

	var stdout, stderr bytes.Buffer
	code := runCLI([]string{"--config", cfgPath, "quota", "stats"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "No local session logs found") {
		t.Fatalf("stdout = %q, want friendly missing-logs note", stdout.String())
	}
}

func TestQuotaShow_ExplainsMissingCredentialsWithoutNetwork(t *testing.T) {
	isolateTestHome(t, t.TempDir())
	cfgPath := writeTempConfig(t)
	viperResetForTest(t)
	t.Setenv("OPENCODE_API_KEY", "")

	var stdout, stderr bytes.Buffer
	code := runCLI([]string{"--config", cfgPath, "quota", "show"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "[OpenCode Go Quota Status]") || !strings.Contains(out, "API key") {
		t.Fatalf("stdout = %q, want credential guidance", out)
	}
}

func TestQuotaShow_RendersBarsAndCountdownFromMockEndpoint(t *testing.T) {
	isolateTestHome(t, t.TempDir())
	cfgPath := writeTempConfig(t)
	viperResetForTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, `{"usage":{
			"rolling":{"status":"ok","percent":43,"resetsAt":"2099-01-01T03:12:00Z"},
			"weekly":{"status":"ok","percent":13,"resetsAt":"2099-01-06T00:00:00Z"},
			"monthly":{"status":"ok","percent":6,"resetsAt":"2099-10-01T00:00:00Z"}
		}}`)
	}))
	defer server.Close()

	t.Setenv("OPENCODE_API_KEY", "test-key")
	t.Setenv("OCT_OPENCODE_USAGE_ENDPOINT", server.URL)

	var stdout, stderr bytes.Buffer
	code := runCLI([]string{"--config", cfgPath, "quota", "show"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr.String())
	}

	out := stdout.String()
	for _, want := range []string{
		"[OpenCode Go Quota Status]",
		"- Rolling 5-Hour:",
		"57% left",
		"(Reset in ",
		"- Weekly Cap:",
		"87% left",
		"- Monthly Cap:",
		"94% left",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q, got:\n%s", want, out)
		}
	}
}

func TestQuotaShow_JsonModeEmitsResult(t *testing.T) {
	isolateTestHome(t, t.TempDir())
	cfgPath := writeTempConfig(t)
	viperResetForTest(t)
	t.Setenv("OPENCODE_API_KEY", "")

	var stdout, stderr bytes.Buffer
	code := runCLI([]string{"--config", cfgPath, "quota", "show", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr.String())
	}
	var payload struct {
		Provider string `json:"provider"`
		Status   string `json:"status"`
		Unit     string `json:"unit"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("stdout is not JSON: %v (%q)", err, stdout.String())
	}
	if payload.Provider != "opencode" {
		t.Fatalf("provider = %q, want opencode", payload.Provider)
	}
}
