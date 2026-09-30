package update

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestExplainResolvedManagerFallsBackWhenNpmMissing(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	manager, reason := explainResolvedManager(t.Context(), Tool{
		Name:       "Kimi Code",
		Package:    "@moonshot-ai/kimi-code",
		BinaryName: "kimi",
	})
	if manager != KimiInstaller {
		t.Fatalf("explainResolvedManager(kimi, npm missing) = %q, want %q", manager, KimiInstaller)
	}
	if reason != "official install script (npm not found)" {
		t.Fatalf("reason = %q, want the npm-missing fallback reason", reason)
	}
}

func TestExplainResolvedManagerReportsMissingNpmForNpmOnlyTools(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	manager, reason := explainResolvedManager(t.Context(), Tool{
		Name:       "MiniMax",
		Package:    "mmx-cli",
		BinaryName: "mmx",
	})
	if manager != Npm {
		t.Fatalf("explainResolvedManager(minimax, npm missing) = %q, want %q", manager, Npm)
	}
	if reason != "npm not found" {
		t.Fatalf("reason = %q, want %q", reason, "npm not found")
	}
}

func TestExplainResolvedManagerKeepsNpmDefaultWhenNpmAvailable(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()
	binaryLookup = func(name string) (string, error) {
		if name == "npm" {
			return "/usr/local/bin/npm", nil
		}
		return "", errExecutableNotFound
	}

	manager, reason := explainResolvedManager(t.Context(), Tool{
		Name:       "Kimi Code",
		Package:    "@moonshot-ai/kimi-code",
		BinaryName: "kimi",
	})
	if manager != Npm || reason != "default fallback" {
		t.Fatalf("explainResolvedManager(kimi, npm present) = (%q, %q), want (npm, default fallback)", manager, reason)
	}
}

func TestInstallerManagersRunOfficialScripts(t *testing.T) {
	wantScripts := map[Manager]string{
		ClaudeInstaller:   "https://claude.ai/install.sh",
		CodexInstaller:    "https://github.com/openai/codex/releases/latest/download/install.sh",
		CopilotInstaller:  "https://gh.io/copilot-install",
		KimiInstaller:     "https://code.kimi.com/kimi-code/install.sh",
		QwenInstaller:     "https://qwen-code-assets.oss-cn-hangzhou.aliyuncs.com/installation/install-qwen-standalone.sh",
		OpenCodeInstaller: "https://opencode.ai/install",
	}

	for manager, wantScript := range wantScripts {
		cmd := manager.InstallCommand(Tool{Name: "Any", BinaryName: "any"})
		if got := commandBase(cmd.Path); got != "bash" {
			t.Fatalf("%s install command path = %q, want bash", manager, got)
		}
		if len(cmd.Args) != 3 || cmd.Args[1] != "-lc" {
			t.Fatalf("%s install command args = %v, want [bash -lc <script>]", manager, cmd.Args)
		}
		joined := cmd.Args[2]
		if !strings.HasPrefix(joined, "curl -fsSL ") || !strings.HasSuffix(joined, " | bash") {
			t.Fatalf("%s install command = %q, want `curl -fsSL <script> | bash`", manager, joined)
		}
		if !strings.Contains(joined, wantScript) {
			t.Fatalf("%s install command = %q, want it to contain %q", manager, joined, wantScript)
		}
	}
}

func TestInstallerManagersReadVersionFromBinary(t *testing.T) {
	binDir := t.TempDir()
	fakes := map[Manager]string{
		ClaudeInstaller:   "claude",
		CodexInstaller:    "codex",
		CopilotInstaller:  "copilot",
		KimiInstaller:     "kimi",
		QwenInstaller:     "qwen",
		OpenCodeInstaller: "opencode",
	}
	for _, binary := range fakes {
		path := filepath.Join(binDir, executableName(binary))
		script := "#!/bin/sh\necho \"fake-tool 9.9.9\"\n"
		if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", binary, err)
		}
	}

	t.Setenv("PATH", binDir)

	for manager, binary := range fakes {
		tool := Tool{Name: "Fake", BinaryName: binary}
		if got := manager.GetInstalledVersion(context.Background(), tool); !strings.Contains(got, "9.9.9") {
			t.Fatalf("%s.GetInstalledVersion() = %q, want it to report 9.9.9", manager, got)
		}
	}
}

func TestMissingBinaryProbesBackingExecutableOnly(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	if got := Npm.missingBinary(); got != "npm" {
		t.Fatalf("Npm.missingBinary() = %q, want %q when npm is absent", got, "npm")
	}
	if got := Brew.missingBinary(); got != "brew" {
		t.Fatalf("Brew.missingBinary() = %q, want %q when brew is absent", got, "brew")
	}
	if got := KimiInstaller.missingBinary(); got != "" {
		t.Fatalf("KimiInstaller.missingBinary() = %q, want \"\" (script installers have no probe)", got)
	}

	binaryLookup = func(name string) (string, error) {
		if name == "npm" {
			return "/usr/local/bin/npm", nil
		}
		return "", errExecutableNotFound
	}
	if got := Npm.missingBinary(); got != "" {
		t.Fatalf("Npm.missingBinary() = %q, want \"\" when npm is present", got)
	}
}

func TestMissingManagerGuidance(t *testing.T) {
	tool := Tool{Name: "MiniMax", BinaryName: "mmx"}

	got := missingManagerGuidance("npm", tool)
	for _, want := range []string{"npm not found in PATH", "MiniMax installs via npm", "install Node.js first", "oct agent-update"} {
		if !strings.Contains(got, want) {
			t.Fatalf("guidance = %q, want it to mention %q", got, want)
		}
	}

	got = missingManagerGuidance("brew", tool)
	if !strings.Contains(got, "brew not found in PATH") || !strings.Contains(got, "https://brew.sh") {
		t.Fatalf("guidance = %q, want Homebrew install guidance", got)
	}

	got = missingManagerGuidance("cargo", tool)
	if !strings.Contains(got, "cargo not found in PATH") {
		t.Fatalf("guidance = %q, want generic %q guidance", got, "cargo")
	}
}

// TestExplainPlansUseInstallersWithoutNpm pins the plan-level view end to
// end: on a machine without npm, Kimi's plan carries the official script as
// the install command, while npm-only MiniMax stays on npm with the
// missing-npm reason the run loop turns into guidance.
func TestExplainPlansUseInstallersWithoutNpm(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	tools := []Tool{
		{Name: "Kimi Code", Package: "@moonshot-ai/kimi-code", BinaryName: "kimi"},
		{Name: "MiniMax", Package: "mmx-cli", BinaryName: "mmx"},
	}
	byName := map[string]Plan{}
	for _, plan := range ExplainPlans(t.Context(), tools) {
		byName[plan.Tool.BinaryName] = plan
	}

	kimi := byName["kimi"]
	if kimi.Manager != KimiInstaller {
		t.Fatalf("plan manager for kimi = %q, want %q", kimi.Manager, KimiInstaller)
	}
	joined := strings.Join(kimi.InstallCommand, " ")
	if !strings.Contains(joined, "https://code.kimi.com/kimi-code/install.sh") {
		t.Fatalf("kimi plan command = %q, want the official install script", joined)
	}

	mmx := byName["mmx"]
	if mmx.Manager != Npm || mmx.Reason != "npm not found" {
		t.Fatalf("plan for mmx = (%q, %q), want (npm, npm not found)", mmx.Manager, mmx.Reason)
	}
}

// TestRunGuidesMissingNpmInsteadOfExecError drives the full Run loop for an
// npm-only tool (MiniMax) on a machine without npm: the tool must be skipped
// with actionable guidance instead of the raw exec failure.
func TestRunGuidesMissingNpmInsteadOfExecError(t *testing.T) {
	reset := stubManagerDetection(t)
	defer reset()

	origEnabled := viper.Get("enabled_tools")
	origOrder := viper.Get("agent_order")
	viper.Set("enabled_tools", []string{"minimax"})
	viper.Set("agent_order", []string{"minimax"})
	t.Cleanup(func() {
		viper.Set("enabled_tools", origEnabled)
		viper.Set("agent_order", origOrder)
	})

	var out bytes.Buffer
	err := Run(t.Context(), Options{
		Input:  strings.NewReader("y\n"),
		Output: &out,
	})
	if err == nil || !strings.Contains(err.Error(), "1 tool(s) failed") {
		t.Fatalf("Run() error = %v, want 1 tool failure", err)
	}

	text := out.String()
	if !strings.Contains(text, "npm not found in PATH") {
		t.Fatalf("Run output missing npm guidance:\n%s", text)
	}
	if !strings.Contains(text, "install Node.js first") {
		t.Fatalf("Run output missing Node.js install hint:\n%s", text)
	}
	if strings.Contains(text, "executable file not found") {
		t.Fatalf("Run output leaked the raw exec error:\n%s", text)
	}
	if !strings.Contains(text, "✗ Skipped") {
		t.Fatalf("Run output should mark the tool as skipped:\n%s", text)
	}
}
