package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestRootCommand(t *testing.T) {
	root := rootCmd
	b := bytes.NewBufferString("")
	root.SetOut(b)
	root.SetArgs([]string{"--help"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("root.Execute() failed: %v", err)
	}

	out := b.String()
	if !contains(out, "A high-performance CLI tool") {
		t.Errorf("expected help message to contain description, got: %s", out)
	}
	if !contains(out, "agent-update") {
		t.Errorf("expected help to include agent-update command, got: %s", out)
	}
	if !contains(out, "update") {
		t.Errorf("expected help to include update command, got: %s", out)
	}
}

func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}

func TestInitConfig_IgnoresNonPrefixedEnabledToolsEnv(t *testing.T) {
	tmpHome := t.TempDir()
	cfgDir := filepath.Join(tmpHome, ".oct")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	cfgPath := filepath.Join(cfgDir, "config.yaml")
	cfg := "enabled_tools:\n  - codex\nagent_order:\n  - codex\n  - gemini\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	oldHome := os.Getenv("HOME")
	oldEnv := os.Getenv("ENABLED_TOOLS")
	t.Cleanup(func() {
		_ = os.Setenv("HOME", oldHome)
		if oldEnv == "" {
			_ = os.Unsetenv("ENABLED_TOOLS")
		} else {
			_ = os.Setenv("ENABLED_TOOLS", oldEnv)
		}
		cfgFile = ""
		viper.Reset()
	})

	if err := os.Setenv("HOME", tmpHome); err != nil {
		t.Fatalf("set HOME failed: %v", err)
	}
	if err := os.Setenv("ENABLED_TOOLS", "gemini,copilot"); err != nil {
		t.Fatalf("set ENABLED_TOOLS failed: %v", err)
	}

	cfgFile = ""
	viper.Reset()
	initConfig()

	got := viper.GetStringSlice("enabled_tools")
	if len(got) != 1 || got[0] != "codex" {
		t.Fatalf("expected config enabled_tools [codex], got %v", got)
	}
}
