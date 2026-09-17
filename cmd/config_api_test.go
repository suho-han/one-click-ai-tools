package cmd

import (
	"encoding/json"
	"math"
	"strconv"
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
	if len(gotOrder) != 2 || gotOrder[0] != "claude" || gotOrder[1] != "codex" {
		t.Fatalf("agent_order = %v, want exactly [claude codex] (no catalog padding)", gotOrder)
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

func floatPtr(value float64) *float64 {
	return &value
}

func TestConfigSnapshot_marshalJSONIncludesSafeAlertSettings(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()
	viper.Set("usage_alert_enabled", true)
	viper.Set("usage_alert_threshold_percent", 80.0)
	viper.Set("usage_alert_critical_percent", 98.0)
	viper.Set("usage_alert_cooldown_minutes", 120)
	viper.Set("usage_alert_quiet_hours", "00:00-08:00")
	viper.Set("usage_alert_timezone", "Asia/Seoul")
	viper.Set("usage_alert_thresholds", map[string]any{
		"default": 80,
		"5h":      85,
		"7d":      90,
	})
	viper.Set("usage_alert_provider_thresholds", map[string]map[string]float64{"codex": {"5h": 95}})
	viper.Set("usage_alert_state_path", "/private/state.json")

	data, err := json.Marshal(buildConfigSnapshot("/tmp/oct.yaml"))
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}

	alert, ok := decoded["alert"].(map[string]any)
	if !ok {
		t.Fatalf("snapshot JSON missing alert object: %s", data)
	}
	if got, want := alert["enabled"], true; got != want {
		t.Fatalf("alert.enabled = %#v, want %v", got, want)
	}
	if got, want := alert["threshold_percent"], 80.0; got != want {
		t.Fatalf("alert.threshold_percent = %#v, want %v", got, want)
	}
	thresholds, ok := alert["thresholds"].(map[string]any)
	if !ok {
		t.Fatalf("alert.thresholds = %#v, want object", alert["thresholds"])
	}
	for key, want := range map[string]float64{"default": 80, "5h": 85, "7d": 90} {
		if got := thresholds[key]; got != want {
			t.Fatalf("alert.thresholds.%s = %#v, want %v", key, got, want)
		}
	}
	for _, forbidden := range []string{"provider_thresholds", "state_path", "snooze_state", "token", "private"} {
		if _, ok := alert[forbidden]; ok {
			t.Fatalf("alert JSON leaked forbidden key %q: %s", forbidden, data)
		}
	}
}

func TestConfigUpdatePayload_applyConfigUpdatePersistsAlertSettings(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()

	payload, err := parseConfigUpdatePayload(`{"alert":{"enabled":true,"threshold_percent":85,"critical_percent":98,"cooldown_minutes":120,"quiet_hours":"00:00-08:00","timezone":"Asia/Seoul","thresholds":{"default":80,"5h":85,"7d":90}}}`)
	if err != nil {
		t.Fatalf("parseConfigUpdatePayload() error = %v", err)
	}
	if err := applyConfigUpdate(payload); err != nil {
		t.Fatalf("applyConfigUpdate() error = %v", err)
	}

	if got := viper.GetBool("usage_alert_enabled"); !got {
		t.Fatal("usage_alert_enabled = false, want true")
	}
	if got := viper.GetFloat64("usage_alert_threshold_percent"); got != 85 {
		t.Fatalf("usage_alert_threshold_percent = %v, want 85", got)
	}
	if got := viper.GetFloat64("usage_alert_critical_percent"); got != 98 {
		t.Fatalf("usage_alert_critical_percent = %v, want 98", got)
	}
	if got := viper.GetInt("usage_alert_cooldown_minutes"); got != 120 {
		t.Fatalf("usage_alert_cooldown_minutes = %d, want 120", got)
	}
	if got := viper.GetString("usage_alert_quiet_hours"); got != "00:00-08:00" {
		t.Fatalf("usage_alert_quiet_hours = %q, want 00:00-08:00", got)
	}
	if got := viper.GetString("usage_alert_timezone"); got != "Asia/Seoul" {
		t.Fatalf("usage_alert_timezone = %q, want Asia/Seoul", got)
	}
	for key, want := range map[string]float64{"default": 80, "5h": 85, "7d": 90} {
		if got := viper.GetFloat64("usage_alert_thresholds." + key); got != want {
			t.Fatalf("usage_alert_thresholds.%s = %v, want %v", key, got, want)
		}
	}
}

func TestConfigUpdatePayload_rejectsInvalidAlertWithoutMutation(t *testing.T) {
	invalidPayloads := map[string]string{
		"percent":     `{"usage_display_mode":"used","alert":{"threshold_percent":101}}`,
		"cooldown":    `{"usage_display_mode":"used","alert":{"cooldown_minutes":0}}`,
		"quiet hours": `{"usage_display_mode":"used","alert":{"quiet_hours":"25:00-08:00"}}`,
		"timezone":    `{"usage_display_mode":"used","alert":{"timezone":"Mars/Olympus"}}`,
	}

	for name, raw := range invalidPayloads {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(viper.Reset)
			viper.Reset()
			viper.Set("usage_alert_threshold_percent", 80.0)
			viper.Set("usage_alert_cooldown_minutes", 360)
			viper.Set("usage_alert_quiet_hours", "")
			viper.Set("usage_alert_timezone", "UTC")
			viper.Set("usage_display_mode", "remaining")

			payload, err := parseConfigUpdatePayload(raw)
			if err != nil {
				t.Fatalf("parseConfigUpdatePayload() error = %v", err)
			}
			if err := applyConfigUpdate(payload); err == nil {
				t.Fatal("applyConfigUpdate() accepted invalid alert payload")
			}
			if got := viper.GetFloat64("usage_alert_threshold_percent"); got != 80 {
				t.Fatalf("usage_alert_threshold_percent = %v, want unchanged 80", got)
			}
			if got := viper.GetInt("usage_alert_cooldown_minutes"); got != 360 {
				t.Fatalf("usage_alert_cooldown_minutes = %d, want unchanged 360", got)
			}
			if got := viper.GetString("usage_alert_quiet_hours"); got != "" {
				t.Fatalf("usage_alert_quiet_hours = %q, want unchanged empty", got)
			}
			if got := viper.GetString("usage_alert_timezone"); got != "UTC" {
				t.Fatalf("usage_alert_timezone = %q, want unchanged UTC", got)
			}
			if got := viper.GetString("usage_display_mode"); got != "remaining" {
				t.Fatalf("usage_display_mode = %q, want unchanged remaining", got)
			}
		})
	}
}

func TestConfigUpdatePayload_rejectsNonFiniteAlertPercentWithoutMutation(t *testing.T) {
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		t.Run(strconv.FormatFloat(value, 'f', -1, 64), func(t *testing.T) {
			// Given
			t.Cleanup(viper.Reset)
			viper.Reset()
			viper.Set("usage_alert_threshold_percent", 80.0)
			viper.Set("usage_alert_critical_percent", 98.0)
			viper.Set("usage_alert_thresholds", map[string]any{"default": 81.0, "5h": 82.0, "7d": 83.0})
			payloads := []configUpdatePayload{
				{Alert: &configAlertUpdatePayload{ThresholdPercent: floatPtr(value)}},
				{Alert: &configAlertUpdatePayload{CriticalPercent: floatPtr(value)}},
				{Alert: &configAlertUpdatePayload{Thresholds: &configAlertThresholdPayload{Default: floatPtr(value)}}},
				{Alert: &configAlertUpdatePayload{Thresholds: &configAlertThresholdPayload{FiveHours: floatPtr(value)}}},
				{Alert: &configAlertUpdatePayload{Thresholds: &configAlertThresholdPayload{SevenDays: floatPtr(value)}}},
			}

			// When
			for _, payload := range payloads {
				if err := applyConfigUpdate(payload); err == nil {
					t.Fatalf("applyConfigUpdate accepted non-finite alert percent %v", value)
				}
			}

			// Then
			if got := viper.GetFloat64("usage_alert_threshold_percent"); got != 80 {
				t.Fatalf("threshold_percent = %v, want unchanged 80", got)
			}
			if got := viper.GetFloat64("usage_alert_critical_percent"); got != 98 {
				t.Fatalf("critical_percent = %v, want unchanged 98", got)
			}
			for key, want := range map[string]float64{"default": 81, "5h": 82, "7d": 83} {
				if got := viper.GetFloat64("usage_alert_thresholds." + key); got != want {
					t.Fatalf("thresholds.%s = %v, want unchanged %v", key, got, want)
				}
			}
		})
	}
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

// TestBuildConfigSnapshot_MirrorsConfigWhenEnabledToolsSet pins the menubar
// contract: with a curated enabled_tools the snapshot lists exactly what the
// config file contains — no catalog defaults padded on — so the menubar
// Settings provider list cannot drift from the file.
func TestBuildConfigSnapshot_MirrorsConfigWhenEnabledToolsSet(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()
	viper.Set("enabled_tools", []string{"codex", "agy"})
	viper.Set("agent_order", []string{"codex", "agy", "claude"})

	got := buildConfigSnapshot("/tmp/oct.yaml")

	want := []string{"codex", "agy", "claude"}
	if len(got.Tools) != len(want) || len(got.AgentOrder) != len(want) {
		t.Fatalf("snapshot = %v / %v, want exactly %v", got.AgentOrder, toolNames(got.Tools), want)
	}
	for i, name := range want {
		if got.Tools[i].BinaryName != name || got.AgentOrder[i] != name {
			t.Fatalf("snapshot position %d = %s / %s, want %s", i, got.Tools[i].BinaryName, got.AgentOrder[i], name)
		}
	}
	if !got.Tools[0].Enabled || !got.Tools[1].Enabled {
		t.Fatalf("codex/agy enabled flags = %v/%v, want true/true", got.Tools[0].Enabled, got.Tools[1].Enabled)
	}
	if got.Tools[2].Enabled {
		t.Fatal("claude Enabled = true, want false (absent from enabled_tools)")
	}
}

// TestBuildConfigSnapshot_SurfacesEnabledEntryMissingFromOrder: a provider
// listed only in enabled_tools must still appear (at the end), or a settings
// save would silently drop it.
func TestBuildConfigSnapshot_SurfacesEnabledEntryMissingFromOrder(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()
	viper.Set("enabled_tools", []string{"codex", "zai"})
	viper.Set("agent_order", []string{"codex"})

	got := buildConfigSnapshot("/tmp/oct.yaml")

	names := toolNames(got.Tools)
	if len(names) != 2 || names[0] != "codex" || names[1] != "zai" {
		t.Fatalf("tools = %v, want [codex zai]", names)
	}
	if !got.Tools[1].Enabled {
		t.Fatal("zai Enabled = false, want true (listed in enabled_tools)")
	}
}

// TestBuildConfigSnapshot_SplitsCommaJoinedEntries: a comma-joined entry is a
// legal enabled_tools/agent_order value that usage fetches by splitting, so
// the snapshot enabled flags must split too and agree with usage.
func TestBuildConfigSnapshot_SplitsCommaJoinedEntries(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()
	viper.Set("enabled_tools", []string{"codex,agy"})
	viper.Set("agent_order", []string{"codex,agy"})

	got := buildConfigSnapshot("/tmp/oct.yaml")

	enabled := map[string]bool{}
	for _, tool := range got.Tools {
		enabled[tool.BinaryName] = tool.Enabled
	}
	if len(got.Tools) != 2 {
		t.Fatalf("tools = %v, want exactly [codex agy]", toolNames(got.Tools))
	}
	if !enabled["codex"] || !enabled["agy"] {
		t.Fatalf("enabled flags = %v, want codex and agy enabled", enabled)
	}
}

func toolNames(tools []configToolStatus) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.BinaryName)
	}
	return names
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
	if len(order) != 3 || order[0] != "zai" || order[1] != "codex" || order[2] != "deepseek" {
		t.Fatalf("agent_order = %v, want exactly [zai codex deepseek] (no catalog padding)", order)
	}

	// Unknown names are still rejected.
	bad := configUpdatePayload{EnabledTools: []string{"zai", "not-a-provider"}}
	if err := applyConfigUpdate(bad); err == nil {
		t.Fatal("applyConfigUpdate accepted unknown provider")
	}
}

func TestBuildConfigAlertSnapshot_InheritsGlobalDefaultForWindows_whenWindowOverridesAbsent(t *testing.T) {
	// Given: the Swift settings payload always sends the complete alert object,
	// so reporting the legacy percent for 5h/7d would persist it as an explicit
	// override on save; report the inherited default instead.
	t.Cleanup(viper.Reset)
	viper.Reset()
	viper.Set("usage_alert_threshold_percent", 70.0)
	viper.Set("usage_alert_thresholds", map[string]any{"default": 65.0})

	// When
	got := buildConfigAlertSnapshot()

	// Then
	if got.Thresholds.Default != 65 {
		t.Fatalf("Default = %v, want 65", got.Thresholds.Default)
	}
	if got.Thresholds.FiveHours != 65 || got.Thresholds.SevenDays != 65 {
		t.Fatalf("5h/7d = %v/%v, want inherited default 65", got.Thresholds.FiveHours, got.Thresholds.SevenDays)
	}
}
