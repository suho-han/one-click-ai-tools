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
	cfg := "enabled_tools:\n  - codex\nmenubar_title_mode: oct\n"
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

func TestRunCLIAlertHelpRemainsAvailable_whenInteractiveModeIsDefault(t *testing.T) {
	// Given
	cfgPath := writeTempConfig(t)
	viperResetForTest(t)
	var stdout, stderr bytes.Buffer

	// When
	code := runCLI([]string{"--config", cfgPath, "alert", "--help"}, &stdout, &stderr)

	// Then
	if code != 0 {
		t.Fatalf("exit code = %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Available Commands:") || !strings.Contains(stdout.String(), "config") || !strings.Contains(stdout.String(), "usage alert config") {
		t.Fatalf("help = %q, want alert subcommands", stdout.String())
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

// TestConfigUpdatePayloadFlag covers the --json collision fix: --payload is
// the input flag, legacy --json warns, both together error, "-" reads stdin,
// and generic failures never retry.
func TestConfigUpdatePayloadFlag(t *testing.T) {
	setup := func(t *testing.T) string {
		cfgPath := writeTempConfig(t)
		viperResetForTest(t)
		// Flag values are package vars; reset them so subtests stay isolated.
		configUpdatePayloadFlag = ""
		configUpdateJSON = ""
		return cfgPath
	}

	t.Run("payload round trip", func(t *testing.T) {
		cfgPath := setup(t)
		var stdout, stderr bytes.Buffer
		payload := `{"menubar_title_mode":"compact"}`
		code := runCLI([]string{"--config", cfgPath, "config", "update", "--payload", payload}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit = %d, stderr: %s", code, stderr.String())
		}
		data, err := os.ReadFile(cfgPath)
		if err != nil {
			t.Fatalf("read config: %v", err)
		}
		if !strings.Contains(string(data), "compact") {
			t.Fatalf("payload not applied, config: %s", data)
		}
	})

	t.Run("legacy json warns on stderr", func(t *testing.T) {
		cfgPath := setup(t)
		var stdout, stderr bytes.Buffer
		code := runCLI([]string{"--config", cfgPath, "config", "update", "--json", `{"menubar_title_mode":"compact"}`}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit = %d, stderr: %s", code, stderr.String())
		}
		if !strings.Contains(stderr.String(), "deprecated") {
			t.Fatalf("stderr = %q, want deprecation warning", stderr.String())
		}
	})

	t.Run("both flags error", func(t *testing.T) {
		cfgPath := setup(t)
		var stdout, stderr bytes.Buffer
		code := runCLI([]string{"--config", cfgPath, "config", "update", "--payload", "{}", "--json", "{}"}, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("exit = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), "not both") {
			t.Fatalf("stderr = %q, want both-flags error", stderr.String())
		}
	})

	t.Run("payload from stdin", func(t *testing.T) {
		cfgPath := setup(t)
		var stdout, stderr bytes.Buffer
		rootCmd.InOrStdin() // keep cobra init consistent
		// runCLI does not wire stdin; feed the payload through SetArgs path by
		// pointing rootCmd's stdin at a reader.
		oldIn := rootCmd.InOrStdin()
		rootCmd.SetIn(strings.NewReader(`{"menubar_title_mode":"oct"}`))
		defer rootCmd.SetIn(oldIn)
		code := runCLI([]string{"--config", cfgPath, "config", "update", "--payload", "-"}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("exit = %d, stderr: %s", code, stderr.String())
		}
	})
}
