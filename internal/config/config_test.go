package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// isolateTestHome points os.UserHomeDir at a temp dir on every platform:
// windows reads USERPROFILE, unix reads HOME.
func isolateTestHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

func TestMigrateLegacyConfig(t *testing.T) {
	tempHome := t.TempDir()
	isolateTestHome(t, tempHome)

	octDir := filepath.Join(tempHome, ".oct")
	if err := os.MkdirAll(octDir, 0o755); err != nil {
		t.Fatalf("mkdir .oct failed: %v", err)
	}
	legacyPath := filepath.Join(octDir, "config")
	legacyContent := "enabled_tools=claude,gemini\nschedule_enabled=true\n"
	if err := os.WriteFile(legacyPath, []byte(legacyContent), 0o644); err != nil {
		t.Fatalf("write legacy config failed: %v", err)
	}

	viper.Reset()
	t.Cleanup(viper.Reset)

	if err := MigrateLegacyConfig(); err != nil {
		t.Fatalf("MigrateLegacyConfig() error = %v", err)
	}

	newPath := filepath.Join(octDir, "config.yaml")
	data, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("migrated config.yaml not readable: %v", err)
	}
	contents := string(data)

	// enabled_tools is the only key migrated out of the legacy file.
	if !strings.Contains(contents, "claude") || !strings.Contains(contents, "gemini") {
		t.Fatalf("migrated config missing enabled_tools values, got:\n%s", contents)
	}
	// Pinned current behavior: legacy keys other than enabled_tools are
	// dropped, not carried into the new yaml.
	if strings.Contains(contents, "schedule_enabled") {
		t.Fatalf("migrated config should drop legacy schedule_enabled, got:\n%s", contents)
	}

	info, err := os.Stat(newPath)
	if err != nil {
		t.Fatalf("stat config.yaml failed: %v", err)
	}
	if runtime.GOOS != "windows" {
		// Windows has no POSIX permission bits, so the 0600 chmod cannot
		// be observed there.
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("config.yaml perms = %o, want 600", perm)
		}
	}

	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("legacy config should be renamed away, stat err = %v", err)
	}
	if _, err := os.Stat(legacyPath + ".bak"); err != nil {
		t.Fatalf("legacy backup missing: %v", err)
	}

	if got := viper.GetStringSlice("enabled_tools"); len(got) != 2 || got[0] != "claude" || got[1] != "gemini" {
		t.Fatalf("viper enabled_tools = %v, want [claude gemini]", got)
	}
}

func TestMigrateLegacyConfigSkipsWhenNewConfigExists(t *testing.T) {
	tempHome := t.TempDir()
	isolateTestHome(t, tempHome)

	octDir := filepath.Join(tempHome, ".oct")
	if err := os.MkdirAll(octDir, 0o755); err != nil {
		t.Fatalf("mkdir .oct failed: %v", err)
	}
	legacyPath := filepath.Join(octDir, "config")
	if err := os.WriteFile(legacyPath, []byte("enabled_tools=claude\n"), 0o644); err != nil {
		t.Fatalf("write legacy config failed: %v", err)
	}
	newPath := filepath.Join(octDir, "config.yaml")
	if err := os.WriteFile(newPath, []byte("enabled_tools:\n  - codex\n"), 0o600); err != nil {
		t.Fatalf("write new config failed: %v", err)
	}

	viper.Reset()
	t.Cleanup(viper.Reset)

	if err := MigrateLegacyConfig(); err != nil {
		t.Fatalf("MigrateLegacyConfig() error = %v", err)
	}
	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("legacy config should be untouched when config.yaml exists: %v", err)
	}
}

func TestMigrateLegacyConfigNoopWithoutLegacyFile(t *testing.T) {
	tempHome := t.TempDir()
	isolateTestHome(t, tempHome)

	viper.Reset()
	t.Cleanup(viper.Reset)

	if err := MigrateLegacyConfig(); err != nil {
		t.Fatalf("MigrateLegacyConfig() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(tempHome, ".oct", "config.yaml")); !os.IsNotExist(err) {
		t.Fatalf("config.yaml should not be created without a legacy config, stat err = %v", err)
	}
}
