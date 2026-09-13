package cmd

import (
	"encoding/json"
	"testing"

	"github.com/spf13/viper"
	"github.com/suho-han/one-click-ai-tools/internal/update"
)

func TestBuildConfigSnapshot_enablesAllToolsWhenEnabledToolsUnset(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()
	viper.Set("usage_display_mode", "remaining")
	viper.Set("menubar_title_mode", "compact")
	viper.Set("session_refresh_enabled", true)
	viper.Set("session_refresh_interval", "weekly")
	viper.Set("session_refresh_hour", 7)
	viper.Set("agent_order", []string{"codex", "agy", "claude"})

	got := buildConfigSnapshot("/tmp/oct.yaml")

	if got.ConfigFile != "/tmp/oct.yaml" {
		t.Fatalf("ConfigFile = %q, want /tmp/oct.yaml", got.ConfigFile)
	}
	if got.UsageDisplayMode != "remaining" {
		t.Fatalf("UsageDisplayMode = %q, want remaining", got.UsageDisplayMode)
	}
	if got.MenubarTitleMode != "compact" {
		t.Fatalf("MenubarTitleMode = %q, want compact", got.MenubarTitleMode)
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
	if got.Tools[0].BinaryName != "codex" || got.Tools[1].BinaryName != "agy" || got.Tools[2].BinaryName != "claude" {
		t.Fatalf("Tools order starts with %v, want [codex agy claude]", []string{got.Tools[0].BinaryName, got.Tools[1].BinaryName, got.Tools[2].BinaryName})
	}
	if got.AgentOrder[0] != "codex" || got.AgentOrder[1] != "agy" || got.AgentOrder[2] != "claude" {
		t.Fatalf("AgentOrder starts with %v, want [codex agy claude]", got.AgentOrder[:3])
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
		MenubarTitleMode:       "compact",
		SessionRefreshEnabled:  boolPtr(true),
		SessionRefreshInterval: "weekly",
		SessionRefreshHour:     intPtr(22),
		AgentOrder:             []string{"claude", "codex"},
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
	if got := viper.GetString("menubar_title_mode"); got != "compact" {
		t.Fatalf("menubar_title_mode = %q, want compact", got)
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
	gotOrder := viper.GetStringSlice("agent_order")
	if len(gotOrder) < 2 || gotOrder[0] != "claude" || gotOrder[1] != "codex" {
		t.Fatalf("agent_order = %v, want prefix [claude codex]", gotOrder)
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
	viper.Set("menubar_title_mode", "oct")
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
		MenubarTitleMode       string `json:"menubar_title_mode"`
		SessionRefreshInterval string `json:"session_refresh_interval"`
		Tools                  []struct {
			BinaryName string `json:"binary_name"`
			Enabled    bool   `json:"enabled"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if decoded.ConfigFile != "/tmp/config.yaml" || decoded.UsageDisplayMode != "used" || decoded.MenubarTitleMode != "oct" || decoded.SessionRefreshInterval != "daily" {
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

func TestConfigSnapshotExposesMenubarRefreshInterval(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()

	// Default: missing config value normalizes to the menubar default "1m".
	got := buildConfigSnapshot("/tmp/oct.yaml")
	if got.MenubarRefreshInterval != "1m" {
		t.Fatalf("default MenubarRefreshInterval = %q, want 1m", got.MenubarRefreshInterval)
	}

	viper.Set("menubar_refresh_interval", "90s")
	got = buildConfigSnapshot("/tmp/oct.yaml")
	if got.MenubarRefreshInterval != "90s" {
		t.Fatalf("MenubarRefreshInterval = %q, want 90s", got.MenubarRefreshInterval)
	}

	viper.Set("menubar_refresh_interval", "bogus")
	got = buildConfigSnapshot("/tmp/oct.yaml")
	if got.MenubarRefreshInterval != "1m" {
		t.Fatalf("invalid MenubarRefreshInterval = %q, want fallback 1m", got.MenubarRefreshInterval)
	}

	// JSON shape: the key must be present for the Swift app.
	data, err := json.Marshal(buildConfigSnapshot("/tmp/oct.yaml"))
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if _, ok := decoded["menubar_refresh_interval"]; !ok {
		t.Fatalf("snapshot JSON missing menubar_refresh_interval key: %s", data)
	}
}

func TestBuildConfigSnapshotIncludesStandaloneProviders(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()
	viper.Set("enabled_tools", []string{"codex", "zai"})
	viper.Set("agent_order", []string{"zai", "codex"})

	got := buildConfigSnapshot("/tmp/oct.yaml")

	var zai *configToolStatus
	for i := range got.Tools {
		if got.Tools[i].BinaryName == "zai" {
			zai = &got.Tools[i]
		}
	}
	if zai == nil {
		t.Fatalf("standalone provider zai missing from snapshot tools: %+v", got.Tools)
	}
	if !zai.Enabled {
		t.Errorf("zai.Enabled = false, want true (listed in enabled_tools)")
	}
	// Opt-in: an unconfigured standalone provider must not appear.
	for _, tool := range got.Tools {
		if tool.BinaryName == "grok" {
			t.Fatalf("unconfigured standalone provider grok leaked into snapshot: %+v", got.Tools)
		}
	}
	// The requested position is carried into AgentOrder.
	if got.AgentOrder[0] != "zai" {
		t.Fatalf("AgentOrder = %v, want zai first", got.AgentOrder)
	}
}

func TestApplyConfigUpdateKeepsStandaloneProviders(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()

	payload := configUpdatePayload{
		EnabledTools: []string{"codex", "zai"},
		AgentOrder:   []string{"zai", "codex", "deepseek"},
	}
	if err := applyConfigUpdate(payload); err != nil {
		t.Fatalf("applyConfigUpdate() error = %v", err)
	}
	enabled := viper.GetStringSlice("enabled_tools")
	if len(enabled) != 2 || enabled[0] != "codex" || enabled[1] != "zai" {
		t.Fatalf("enabled_tools = %v, want [codex zai]", enabled)
	}
	order := viper.GetStringSlice("agent_order")
	if len(order) < 3 || order[0] != "zai" || order[1] != "codex" || order[2] != "deepseek" {
		t.Fatalf("agent_order = %v, want [zai codex deepseek ...]", order)
	}

	// Unknown names are still rejected.
	bad := configUpdatePayload{EnabledTools: []string{"zai", "not-a-provider"}}
	if err := applyConfigUpdate(bad); err == nil {
		t.Fatal("applyConfigUpdate accepted unknown provider")
	}
}
