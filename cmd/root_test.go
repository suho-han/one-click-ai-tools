package cmd

import (
	"bytes"
	"encoding/json"
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
	if !contains(out, "session-refresh") {
		t.Errorf("expected help to include session-refresh command, got: %s", out)
	}
	if !contains(out, "update") {
		t.Errorf("expected help to include update command, got: %s", out)
	}
	if !contains(out, "menubar") {
		t.Errorf("expected help to include menubar command, got: %s", out)
	}
}

func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}

func TestRootVersionMatchesPackageJSON(t *testing.T) {
	type pkg struct {
		Version string `json:"version"`
	}

	data, err := os.ReadFile("../package.json")
	if err != nil {
		t.Fatalf("read package.json failed: %v", err)
	}
	var p pkg
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("parse package.json failed: %v", err)
	}
	if p.Version == "" {
		t.Fatal("package.json version is empty")
	}
	if rootCmd.Version != p.Version {
		t.Fatalf("version mismatch: rootCmd=%q package.json=%q", rootCmd.Version, p.Version)
	}
}

func TestInitConfig_IgnoresNonPrefixedEnabledToolsEnv(t *testing.T) {
	tmpHome := t.TempDir()
	cfgDir := filepath.Join(tmpHome, ".oct")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	cfgPath := filepath.Join(cfgDir, "config.yaml")
	cfg := "enabled_tools:\n  - codex\nagent_order:\n  - codex\n  - agy\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	oldEnv := os.Getenv("ENABLED_TOOLS")
	t.Cleanup(func() {
		if oldEnv == "" {
			_ = os.Unsetenv("ENABLED_TOOLS")
		} else {
			_ = os.Setenv("ENABLED_TOOLS", oldEnv)
		}
		cfgFile = ""
		viper.Reset()
	})

	if err := os.Setenv("ENABLED_TOOLS", "agy,copilot"); err != nil {
		t.Fatalf("set ENABLED_TOOLS failed: %v", err)
	}

	cfgFile = cfgPath
	viper.Reset()
	initConfig()

	got := viper.GetStringSlice("enabled_tools")
	if len(got) != 1 || got[0] != "codex" {
		t.Fatalf("expected config enabled_tools [codex], got %v", got)
	}
}

func TestInitConfigSessionRefreshDefaults(t *testing.T) {
	cfgFile = t.TempDir() + "/missing-config.yaml"
	viper.Reset()
	defer func() {
		cfgFile = ""
		viper.Reset()
	}()

	initConfig()

	if viper.GetBool("session_refresh_enabled") {
		t.Fatal("expected session_refresh_enabled default false")
	}
	if got := viper.GetString("session_refresh_interval"); got != "daily" {
		t.Fatalf("expected session_refresh_interval=daily, got %q", got)
	}
	if got := viper.GetInt("session_refresh_hour"); got != 9 {
		t.Fatalf("expected session_refresh_hour=9, got %d", got)
	}
	if got := viper.GetString("menubar_refresh_interval"); got != "1m" {
		t.Fatalf("expected menubar_refresh_interval=1m, got %q", got)
	}
}
