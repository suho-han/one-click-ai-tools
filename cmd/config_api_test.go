package cmd

import (
	"encoding/json"
	"testing"

	"github.com/spf13/viper"
	"github.com/suho-han/one-click-tools/internal/update"
)

func TestBuildConfigSnapshot_enablesAllToolsWhenEnabledToolsUnset(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()
	viper.Set("usage_display_mode", "remaining")
	viper.Set("session_refresh_enabled", true)
	viper.Set("session_refresh_interval", "weekly")
	viper.Set("session_refresh_hour", 7)

	got := buildConfigSnapshot("/tmp/oct.yaml")

	if got.ConfigFile != "/tmp/oct.yaml" {
		t.Fatalf("ConfigFile = %q, want /tmp/oct.yaml", got.ConfigFile)
	}
	if got.UsageDisplayMode != "remaining" {
		t.Fatalf("UsageDisplayMode = %q, want remaining", got.UsageDisplayMode)
	}
	if !got.SessionRefreshEnabled {
		t.Fatal("SessionRefreshEnabled = false, want true")
	}
	if got.SessionRefreshInterval != "weekly" {
		t.Fatalf("SessionRefreshInterval = %q, want weekly", got.SessionRefreshInterval)
	}
	if got.SessionRefreshHour != 7 {
		t.Fatalf("SessionRefreshHour = %d, want 7", got.SessionRefreshHour)
	}
	if len(got.Tools) != len(update.Tools) {
		t.Fatalf("Tools length = %d, want %d", len(got.Tools), len(update.Tools))
	}
	for _, tool := range got.Tools {
		if !tool.Enabled {
			t.Fatalf("tool %s disabled, want all enabled when enabled_tools is unset", tool.BinaryName)
		}
	}
}

func TestConfigUpdatePayload_applyConfigUpdatePersistsValidValues(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()

	payload := configUpdatePayload{
		EnabledTools:           []string{"codex", "claude-code"},
		UsageDisplayMode:       "used",
		SessionRefreshEnabled:  boolPtr(true),
		SessionRefreshInterval: "weekly",
		SessionRefreshHour:     intPtr(22),
	}

	if err := applyConfigUpdate(payload); err != nil {
		t.Fatalf("applyConfigUpdate returned error: %v", err)
	}

	gotTools := viper.GetStringSlice("enabled_tools")
	if len(gotTools) != 2 || gotTools[0] != "codex" || gotTools[1] != "claude" {
		t.Fatalf("enabled_tools = %v, want [codex claude]", gotTools)
	}
	if got := viper.GetString("usage_display_mode"); got != "used" {
		t.Fatalf("usage_display_mode = %q, want used", got)
	}
	if !viper.GetBool("session_refresh_enabled") {
		t.Fatal("session_refresh_enabled = false, want true")
	}
	if got := viper.GetString("session_refresh_interval"); got != "weekly" {
		t.Fatalf("session_refresh_interval = %q, want weekly", got)
	}
	if got := viper.GetInt("session_refresh_hour"); got != 22 {
		t.Fatalf("session_refresh_hour = %d, want 22", got)
	}
}

func TestConfigUpdatePayload_rejectsInvalidPayloadWithoutMutation(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()
	viper.Set("enabled_tools", []string{"codex"})
	viper.Set("usage_display_mode", "remaining")
	viper.Set("session_refresh_enabled", false)
	viper.Set("session_refresh_interval", "daily")
	viper.Set("session_refresh_hour", 9)

	payload := configUpdatePayload{
		EnabledTools:           []string{"not-a-provider"},
		UsageDisplayMode:       "spent",
		SessionRefreshEnabled:  boolPtr(true),
		SessionRefreshInterval: "monthly",
		SessionRefreshHour:     intPtr(24),
	}

	if err := applyConfigUpdate(payload); err == nil {
		t.Fatal("applyConfigUpdate returned nil, want validation error")
	}

	if got := viper.GetStringSlice("enabled_tools"); len(got) != 1 || got[0] != "codex" {
		t.Fatalf("enabled_tools mutated to %v, want [codex]", got)
	}
	if got := viper.GetString("usage_display_mode"); got != "remaining" {
		t.Fatalf("usage_display_mode mutated to %q, want remaining", got)
	}
	if viper.GetBool("session_refresh_enabled") {
		t.Fatal("session_refresh_enabled mutated to true, want false")
	}
	if got := viper.GetString("session_refresh_interval"); got != "daily" {
		t.Fatalf("session_refresh_interval mutated to %q, want daily", got)
	}
	if got := viper.GetInt("session_refresh_hour"); got != 9 {
		t.Fatalf("session_refresh_hour mutated to %d, want 9", got)
	}
}

func TestConfigSnapshot_marshalJSONUsesMachineReadableShape(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()
	viper.Set("enabled_tools", []string{"codex"})
	viper.Set("usage_display_mode", "used")
	viper.Set("session_refresh_interval", "daily")
	viper.Set("session_refresh_hour", 5)

	snapshot := buildConfigSnapshot("/tmp/config.yaml")
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}

	var decoded struct {
		ConfigFile             string `json:"config_file"`
		UsageDisplayMode       string `json:"usage_display_mode"`
		SessionRefreshInterval string `json:"session_refresh_interval"`
		Tools                  []struct {
			BinaryName string `json:"binary_name"`
			Enabled    bool   `json:"enabled"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if decoded.ConfigFile != "/tmp/config.yaml" || decoded.UsageDisplayMode != "used" || decoded.SessionRefreshInterval != "daily" {
		t.Fatalf("decoded snapshot = %+v", decoded)
	}
	if len(decoded.Tools) == 0 || decoded.Tools[0].BinaryName == "" {
		t.Fatalf("decoded tools missing binary names: %+v", decoded.Tools)
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func intPtr(value int) *int {
	return &value
}
