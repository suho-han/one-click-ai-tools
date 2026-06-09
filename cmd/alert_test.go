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

func TestProviderOptionsIncludesCursor(t *testing.T) {
	viper.Reset()
	viper.Set("enabled_tools", []string{"cursor-agent", "agy", "opencode"})
	opts := providerOptions()
	hasCursor := false
	hasAntigravity := false
	for _, o := range opts {
		if o == "cursor" {
			hasCursor = true
		}
		if o == "antigravity" {
			hasAntigravity = true
		}
	}
	if !hasCursor {
		t.Fatalf("expected cursor in provider options")
	}
	if !hasAntigravity {
		t.Fatalf("expected antigravity in provider options")
	}
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
