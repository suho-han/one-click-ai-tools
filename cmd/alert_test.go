package cmd

import (
	"testing"

	"github.com/spf13/viper"
)

func TestSetAlertConfigValueThresholdWindow(t *testing.T) {
	viper.Reset()
	viper.Set("usage_alert_thresholds", map[string]any{"default": 80.0})

	if err := setAlertConfigValue("threshold.5h", "91"); err != nil {
		t.Fatalf("setAlertConfigValue failed: %v", err)
	}
	cfg := buildAlertConfigFromViper(true)
	if cfg.GlobalThresholds["5h"] != 91 {
		t.Fatalf("expected threshold.5h=91, got %v", cfg.GlobalThresholds["5h"])
	}
}

func TestSetAlertConfigValueProviderWindow(t *testing.T) {
	viper.Reset()
	viper.Set("usage_alert_provider_thresholds", map[string]any{})

	if err := setAlertConfigValue("provider.codex.5h", "94"); err != nil {
		t.Fatalf("setAlertConfigValue failed: %v", err)
	}
	cfg := buildAlertConfigFromViper(true)
	if cfg.ProviderThreshold["codex"]["5h"] != 94 {
		t.Fatalf("expected provider codex 5h=94, got %v", cfg.ProviderThreshold["codex"]["5h"])
	}
}

func TestSetAlertConfigValueInvalidProviderKey(t *testing.T) {
	viper.Reset()
	if err := setAlertConfigValue("provider.codex", "94"); err == nil {
		t.Fatalf("expected error for invalid provider key")
	}
}

func TestSetAlertConfigValueRejectsInvalidScalars(t *testing.T) {
	viper.Reset()

	for key, value := range map[string]string{
		"enabled":           "truthy",
		"cooldown_minutes":  "0",
		"threshold_percent": "101",
		"critical_percent":  "-1",
		"quiet_hours":       "9:00-18:00",
		"timezone":          "No/Such_Zone",
	} {
		if err := setAlertConfigValue(key, value); err == nil {
			t.Fatalf("expected %s=%q to be rejected", key, value)
		}
	}
}

func TestBuildAlertConfigFromViperIncludesCriticalPercent(t *testing.T) {
	viper.Reset()
	viper.Set("usage_alert_critical_percent", 95.0)

	cfg := buildAlertConfigFromViper(true)
	if cfg.CriticalPct != 95 {
		t.Fatalf("expected critical percent 95, got %v", cfg.CriticalPct)
	}
}

func TestProviderOptionsIncludesCursor(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("enabled_tools", []string{"cursor-agent", "agy", "opencode"})
	opts := providerOptions()
	has := func(name string) bool {
		for _, o := range opts {
			if o == name {
				return true
			}
		}
		return false
	}
	// Options are canonical registry keys matching UsageResult.Provider, so
	// stored thresholds and snoozes resolve at evaluation time.
	if !has("cursor-agent") {
		t.Fatalf("expected cursor-agent in provider options: %v", opts)
	}
	if !has("agy") {
		t.Fatalf("expected agy in provider options: %v", opts)
	}
}

func TestProviderOptionsCoverDefaultAndOptInProviders(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	// Fresh config: enabled_tools is empty, yet every installable provider --
	// including the newer Kimi/Qwen/MiniMax ones -- is fetched by default and
	// must be selectable for provider-specific thresholds.
	opts := providerOptions()
	has := func(name string) bool {
		for _, o := range opts {
			if o == name {
				return true
			}
		}
		return false
	}
	for _, name := range []string{"kimi", "qwen", "minimax", "claude", "codex"} {
		if !has(name) {
			t.Fatalf("expected %s in default provider options: %v", name, opts)
		}
	}
	// Standalone providers stay opt-in.
	if has("zai") {
		t.Fatalf("unconfigured standalone provider leaked into options: %v", opts)
	}

	viper.Set("enabled_tools", []string{"codex", "zai"})
	opts = providerOptions()
	for _, o := range opts {
		if o == "zai" {
			return
		}
	}
	t.Fatalf("expected opted-in standalone zai in provider options: %v", opts)
}

func TestAlertPriorityLabel(t *testing.T) {
	if got := alertPriorityLabel(99, 90, 98); got != "CRITICAL" {
		t.Fatalf("expected CRITICAL, got %s", got)
	}
	if got := alertPriorityLabel(92, 90, 98); got != "HIGH" {
		t.Fatalf("expected HIGH, got %s", got)
	}
	if got := alertPriorityLabel(89, 90, 98); got != "NORMAL" {
		t.Fatalf("expected NORMAL, got %s", got)
	}
}
