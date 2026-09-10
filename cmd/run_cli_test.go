package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// writeTempConfig gives initConfig a valid explicit config so command
// execution does not depend on the real user home.
func writeTempConfig(t *testing.T) string {
	t.Helper()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := "enabled_tools:\n  - codex\nusage_display_mode: remaining\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}
	return cfgPath
}

// TestRunCLIRoutesCommandErrorsToStderr drives failures through the same
// runCLI entry the binary uses, asserting the production contract:
// non-zero exit, the error on stderr only, and no cobra usage spam.
func TestRunCLIRoutesCommandErrorsToStderr(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "schedule enable invalid interval", args: []string{"schedule", "enable", "--interval", "bogus", "--hour", "9"}, wantErr: "invalid interval"},
		{name: "schedule invalid task", args: []string{"schedule", "--task", "nope"}, wantErr: "invalid task"},
		{name: "schedule config rejects non-session-refresh", args: []string{"schedule", "config", "--task", "agent-update"}, wantErr: "session-refresh only"},
		{name: "alert config set validation", args: []string{"alert", "config", "set", "threshold_percent", "abc"}, wantErr: "invalid alert config"},
		{name: "alert config set unknown key", args: []string{"alert", "config", "set", "nope", "1"}, wantErr: "invalid alert config"},
		{name: "alert snooze set zero duration", args: []string{"alert", "snooze", "set", "--duration", "0s"}, wantErr: "duration must be > 0"},
		{name: "config set-tools unknown tool", args: []string{"config", "set", "tools", "nope"}, wantErr: "unknown tool: nope"},
		{name: "config usage-mode invalid", args: []string{"config", "set", "usage-mode", "sideways"}, wantErr: "invalid usage mode"},
		{name: "config menubar-title-mode invalid", args: []string{"config", "set", "menubar-title-mode", "wide"}, wantErr: "invalid menubar title mode"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgPath := writeTempConfig(t)
			viperResetForTest(t)

			var stdout, stderr bytes.Buffer
			args := append([]string{"--config", cfgPath}, tt.args...)
			code := runCLI(args, &stdout, &stderr)

			if code != 1 {
				t.Fatalf("exit code = %d, want 1 (stderr: %s)", code, stderr.String())
			}
			if !strings.Contains(stderr.String(), tt.wantErr) {
				t.Fatalf("stderr = %q, want containing %q", stderr.String(), tt.wantErr)
			}
			if !strings.HasPrefix(stderr.String(), "oct: ") {
				t.Fatalf("stderr = %q, want prefixed with %q", stderr.String(), "oct: ")
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestRunCLISuccessExitsZeroWithCleanStderr(t *testing.T) {
	cfgPath := writeTempConfig(t)
	viperResetForTest(t)

	var stdout, stderr bytes.Buffer
	code := runCLI([]string{"--config", cfgPath, "alert", "config", "show"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ThresholdPct") {
		t.Fatalf("stdout = %q, want alert config JSON", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func viperResetForTest(t *testing.T) {
	t.Helper()
	cfgFile = ""
	viper.Reset()
	t.Cleanup(func() {
		cfgFile = ""
		viper.Reset()
	})
}

// TestFlagHeavyCommandsDeclareExamples guards the learning-curve fix: the
// flag-heavy commands must ship Examples (rendered as "Examples:" in help).
func TestFlagHeavyCommandsDeclareExamples(t *testing.T) {
	for _, tc := range []struct{ name, example string }{
		{"monitor", monitorCmd.Example},
		{"schedule", scheduleCmd.Example},
		{"alert", alertCmd.Example},
	} {
		if strings.TrimSpace(tc.example) == "" {
			t.Errorf("%s command has no Example", tc.name)
		}
	}

	cfgPath := writeTempConfig(t)
	viperResetForTest(t)
	var stdout, stderr bytes.Buffer
	if code := runCLI([]string{"--config", cfgPath, "monitor", "--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("monitor --help exit = %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Examples:") {
		t.Fatalf("monitor help missing Examples section, got:\n%s", stdout.String())
	}
}
