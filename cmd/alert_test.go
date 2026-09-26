package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/viper"
)

func TestAlertSettingsModel_HasRequiredRows_whenCreated(t *testing.T) {
	// Given
	viper.Reset()
	t.Cleanup(viper.Reset)
	m := newAlertSettingsModel(alertSettingsDraftFromViper())

	// When
	got := make([]string, 0, len(m.items))
	for _, item := range m.items {
		got = append(got, item.key)
	}

	// Then
	want := []string{
		"enabled", "threshold_percent", "critical_percent", "cooldown_minutes",
		"quiet", "threshold.default", "threshold.5h", "threshold.7d", "confirm",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("rows = %v, want %v", got, want)
	}
}

func TestAlertSettingsModel_MovesCursorAndTogglesEnabled_whenKeysPressed(t *testing.T) {
	// Given
	viper.Reset()
	t.Cleanup(viper.Reset)
	m := newAlertSettingsModel(alertSettingsDraftFromViper())

	// When
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(alertSettingsModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(alertSettingsModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})
	m = updated.(alertSettingsModel)

	// Then
	if m.index() != 0 {
		t.Fatalf("cursor = %d, want 0", m.index())
	}
	if m.items[0].value != "true" {
		t.Fatalf("enabled value = %q, want true", m.items[0].value)
	}
}

func TestAlertSettingsModel_UpdatesScalar_whenValidInputSubmitted(t *testing.T) {
	// Given
	viper.Reset()
	t.Cleanup(viper.Reset)
	m := newAlertSettingsModel(alertSettingsDraftFromViper())
	m.items[1].cursor = true
	m.items[0].cursor = false

	// When
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(alertSettingsModel)
	m.editingValue = "85"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(alertSettingsModel)

	// Then
	if m.editing {
		t.Fatal("editing should finish after valid input")
	}
	if m.items[1].value != "85" {
		t.Fatalf("threshold value = %q, want 85", m.items[1].value)
	}
}

func TestAlertSettingsModel_UpdatesScalarWithoutCancelKey_whenEditingString(t *testing.T) {
	// Given
	viper.Reset()
	t.Cleanup(viper.Reset)
	m := newAlertSettingsModel(alertSettingsDraftFromViper())
	for i := range m.items {
		m.items[i].cursor = m.items[i].key == "cooldown_minutes"
	}
	m.items[m.index()].value = ""

	// When
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(alertSettingsModel)
	for _, r := range "45" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(alertSettingsModel)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(alertSettingsModel)

	// Then
	if m.cancelled || m.editing {
		t.Fatalf("edit state = cancelled:%t editing:%t, want committed edit", m.cancelled, m.editing)
	}
	if got := m.items[m.index()].value; got != "45" {
		t.Fatalf("cooldown_minutes = %q, want 45", got)
	}
}

func TestAlertSettingsModel_EscKeepsCurrentValue_whenEditingString(t *testing.T) {
	// Given
	viper.Reset()
	t.Cleanup(viper.Reset)
	m := newAlertSettingsModel(alertSettingsDraftFromViper())
	for i := range m.items {
		m.items[i].cursor = m.items[i].key == "cooldown_minutes"
	}

	// When
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(alertSettingsModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("90")})
	m = updated.(alertSettingsModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(alertSettingsModel)

	// Then
	if m.editing || m.cancelled || m.items[m.index()].value == "90" {
		t.Fatalf("Esc changed edit state/value: editing:%t cancelled:%t cooldown:%q", m.editing, m.cancelled, m.items[m.index()].value)
	}
}

func TestAlertSettingsModel_KeepsInvalidScalarEditing_whenInputRejected(t *testing.T) {
	// Given
	viper.Reset()
	t.Cleanup(viper.Reset)
	m := newAlertSettingsModel(alertSettingsDraftFromViper())
	m.items[1].cursor = true
	m.items[0].cursor = false

	// When
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(alertSettingsModel)
	m.editingValue = "101"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(alertSettingsModel)

	// Then
	if !m.editing {
		t.Fatal("editing should remain active after invalid input")
	}
	if !strings.Contains(m.validationError, "must be > 0 and <= 100") {
		t.Fatalf("validation error = %q", m.validationError)
	}
}

func TestAlertSettingsModel_KeepsNonFinitePercentEditing_whenInputRejected(t *testing.T) {
	// Given
	viper.Reset()
	t.Cleanup(viper.Reset)
	m := newAlertSettingsModel(alertSettingsDraftFromViper())
	m.items[1].cursor = true
	m.items[0].cursor = false
	original := m.items[1].value
	viper.Set("usage_alert_threshold_percent", 80.0)

	// When
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(alertSettingsModel)
	m.editingValue = "NaN"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(alertSettingsModel)

	// Then
	if !m.editing {
		t.Fatal("editing should remain active after non-finite input")
	}
	if m.items[1].value != original {
		t.Fatalf("threshold value = %q, want unchanged %q", m.items[1].value, original)
	}
	if got := viper.GetFloat64("usage_alert_threshold_percent"); got != 80 {
		t.Fatalf("threshold_percent = %v, want unchanged 80", got)
	}
	if m.validationError == "" {
		t.Fatal("non-finite input should produce a validation error")
	}
}

func TestAlertSettingsModel_CancelsWithoutPersisting_whenCancelKeyPressed(t *testing.T) {
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("q")},
		{Type: tea.KeyCtrlC},
		{Type: tea.KeyCtrlQ},
	} {
		t.Run(key.String(), func(t *testing.T) {
			// Given
			viper.Reset()
			t.Cleanup(viper.Reset)
			viper.Set("usage_alert_enabled", false)
			m := newAlertSettingsModel(alertSettingsDraftFromViper())

			// When
			updated, command := m.Update(key)
			m = updated.(alertSettingsModel)

			// Then
			if !m.cancelled || !m.done || command == nil {
				t.Fatalf("cancel state = cancelled:%t done:%t command:%t", m.cancelled, m.done, command != nil)
			}
			if viper.GetBool("usage_alert_enabled") {
				t.Fatal("cancel must not mutate Viper")
			}
		})
	}
}

func TestAlertSettingsModel_QuitsForConfirm_whenConfirmRowSelected(t *testing.T) {
	// Given
	viper.Reset()
	t.Cleanup(viper.Reset)
	m := newAlertSettingsModel(alertSettingsDraftFromViper())
	for i := range m.items {
		m.items[i].cursor = i == len(m.items)-1
	}

	// When
	updated, command := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(alertSettingsModel)

	// Then
	if !m.done || m.cancelled || command == nil {
		t.Fatalf("confirm state = done:%t cancelled:%t command:%t", m.done, m.cancelled, command != nil)
	}
}

func TestApplyAlertSettingsDraft_PersistsAllRows_whenConfirmed(t *testing.T) {
	// Given
	viper.Reset()
	t.Cleanup(viper.Reset)
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	viper.SetConfigFile(configPath)
	draft := alertSettingsDraft{
		values: map[string]string{
			"enabled": "true", "threshold_percent": "82", "critical_percent": "97", "cooldown_minutes": "45",
			"quiet": "2h", "threshold.default": "81", "threshold.5h": "83", "threshold.7d": "84",
		},
	}

	// When
	err := applyAlertSettingsDraft(draft)

	// Then
	if err != nil {
		t.Fatalf("apply draft: %v", err)
	}
	if !viper.GetBool("usage_alert_enabled") || viper.GetFloat64("usage_alert_threshold_percent") != 82 || viper.GetInt("usage_alert_cooldown_minutes") != 45 {
		t.Fatalf("saved alert values were not applied: enabled=%t threshold=%v cooldown=%d", viper.GetBool("usage_alert_enabled"), viper.GetFloat64("usage_alert_threshold_percent"), viper.GetInt("usage_alert_cooldown_minutes"))
	}
	untilRaw := viper.GetString("usage_alert_quiet_until")
	until, err := time.Parse(time.RFC3339, untilRaw)
	if err != nil {
		t.Fatalf("persisted quiet_until %q is not RFC3339: %v", untilRaw, err)
	}
	if remaining := time.Until(until); remaining <= 0 || remaining > 3*time.Hour {
		t.Fatalf("quiet_until = %v, want roughly 2h from now", until)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read persisted config: %v", err)
	}
	if !strings.Contains(string(data), "quiet_until") {
		t.Fatalf("persisted config missing alert values: %s", data)
	}
}

func TestApplyAlertSettingsDraft_SkipsQuietRow_whenTimerUntouched(t *testing.T) {
	// Given: a draft whose quiet row still shows the armed state it loaded
	// with; confirming other settings must not re-arm or clear the timer.
	viper.Reset()
	t.Cleanup(viper.Reset)
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	viper.SetConfigFile(configPath)
	armed := time.Now().Add(2 * time.Hour).Format(time.RFC3339)
	viper.Set("usage_alert_quiet_until", armed)
	draft := alertSettingsDraft{
		values: map[string]string{
			"enabled": "true", "threshold_percent": "80", "critical_percent": "98", "cooldown_minutes": "360",
			"quiet": "on (1h 59m left)", "threshold.default": "80", "threshold.5h": "80", "threshold.7d": "80",
		},
		initialQuiet: "on (1h 59m left)",
	}

	// When
	if err := applyAlertSettingsDraft(draft); err != nil {
		t.Fatalf("apply draft: %v", err)
	}

	// Then
	if got := viper.GetString("usage_alert_quiet_until"); got != armed {
		t.Fatalf("quiet_until = %q, want unchanged %q", got, armed)
	}
}

func TestApplyAlertSettingsDraft_UpdatesQuietTimer_whenCycledToChoice(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	viper.SetConfigFile(configPath)
	armed := time.Now().Add(2 * time.Hour).Format(time.RFC3339)
	viper.Set("usage_alert_quiet_until", armed)
	draft := alertSettingsDraft{
		values: map[string]string{
			"enabled": "true", "threshold_percent": "80", "critical_percent": "98", "cooldown_minutes": "360",
			"quiet": "off", "threshold.default": "80", "threshold.5h": "80", "threshold.7d": "80",
		},
		initialQuiet: "on (1h 59m left)",
	}

	if err := applyAlertSettingsDraft(draft); err != nil {
		t.Fatalf("apply draft: %v", err)
	}
	if got := viper.GetString("usage_alert_quiet_until"); got != "" {
		t.Fatalf("quiet_until = %q, want cleared", got)
	}
}

func TestApplyCompletedAlertSettingsModel_DoesNotPersist_whenProgramStopsBeforeConfirm(t *testing.T) {
	// Given
	viper.Reset()
	t.Cleanup(viper.Reset)
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	viper.SetConfigFile(configPath)
	m := newAlertSettingsModel(alertSettingsDraftFromViper())
	m.items[0].value = "true"

	// When
	err := applyCompletedAlertSettingsModel(m)

	// Then
	if err != nil {
		t.Fatalf("apply incomplete model: %v", err)
	}
	if viper.GetBool("usage_alert_enabled") {
		t.Fatal("an incomplete model must not mutate the alert config")
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("config exists after incomplete model: %v", err)
	}
}

func TestRunInteractiveAlert_DoesNotPersist_whenInputCancels(t *testing.T) {
	// Given
	viper.Reset()
	t.Cleanup(viper.Reset)
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	viper.SetConfigFile(configPath)
	var output bytes.Buffer

	// When
	err := runInteractiveAlert(strings.NewReader("q"), &output)

	// Then
	if err != nil {
		t.Fatalf("run interactive alert: %v", err)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("config exists after interactive cancellation: %v", err)
	}
}

func TestAlertSettingsModel_WindowFitsTerminalHeight_whenResized(t *testing.T) {
	// Given
	viper.Reset()
	t.Cleanup(viper.Reset)
	m := newAlertSettingsModel(alertSettingsDraftFromViper())
	m.applyWindowSize(10)

	// When
	view := m.View()

	// Then
	if lines := strings.Count(view, "\n"); lines > 10 {
		t.Fatalf("view lines = %d, want <= 10", lines)
	}
}

func TestAlertSettingsModel_KeepsCursorInWindow_whenNavigating(t *testing.T) {
	// Given
	viper.Reset()
	t.Cleanup(viper.Reset)
	m := newAlertSettingsModel(alertSettingsDraftFromViper())
	m.applyWindowSize(10)

	// When
	for range 4 {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(alertSettingsModel)
	}

	// Then
	if m.offset == 0 {
		t.Fatal("expected navigation to scroll the window")
	}
	if !strings.Contains(m.View(), m.items[m.index()].label) {
		t.Fatal("selected row should remain visible after scrolling")
	}
}

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

func TestSetAlertConfigValueRejectsNonFinitePercentWithoutMutation(t *testing.T) {
	for _, value := range []string{"NaN", "+Inf", "-Inf"} {
		t.Run(value, func(t *testing.T) {
			// Given
			viper.Reset()
			viper.Set("usage_alert_threshold_percent", 80.0)
			viper.Set("usage_alert_critical_percent", 98.0)
			viper.Set("usage_alert_thresholds", map[string]any{"5h": 85.0})
			viper.Set("usage_alert_provider_thresholds", map[string]any{"codex": map[string]any{"5h": 90.0}})

			// When
			keys := []string{"threshold_percent", "critical_percent", "threshold.5h", "provider.codex.5h"}
			for _, key := range keys {
				if err := setAlertConfigValue(key, value); err == nil {
					t.Fatalf("setAlertConfigValue(%q, %q) accepted non-finite percent", key, value)
				}
			}

			// Then
			if got := viper.GetFloat64("usage_alert_threshold_percent"); got != 80 {
				t.Fatalf("threshold_percent = %v, want unchanged 80", got)
			}
			if got := viper.GetFloat64("usage_alert_critical_percent"); got != 98 {
				t.Fatalf("critical_percent = %v, want unchanged 98", got)
			}
			if got := viper.GetFloat64("usage_alert_thresholds.5h"); got != 85 {
				t.Fatalf("thresholds.5h = %v, want unchanged 85", got)
			}
			if got := viper.GetFloat64("usage_alert_provider_thresholds.codex.5h"); got != 90 {
				t.Fatalf("provider threshold = %v, want unchanged 90", got)
			}
		})
	}
}

func TestParseAlertPercentPreservesDecimalsAndRejectsNonFiniteValues(t *testing.T) {
	// Given
	tests := []struct {
		name  string
		value string
		want  float64
	}{
		{name: "decimal", value: "12.5", want: 12.5},
		{name: "nan", value: "NaN"},
		{name: "positive infinity", value: "+Inf"},
		{name: "negative infinity", value: "-Inf"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			got, err := parseAlertPercent("threshold", tt.value)

			// Then
			if tt.want != 0 {
				if err != nil || got != tt.want {
					t.Fatalf("parseAlertPercent(%q) = %v, %v; want %v, nil", tt.value, got, err, tt.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("parseAlertPercent(%q) accepted non-finite value", tt.value)
			}
		})
	}
}

func TestSetAlertConfigValueRejectsInvalidScalars(t *testing.T) {
	viper.Reset()

	for key, value := range map[string]string{
		"enabled":           "truthy",
		"cooldown_minutes":  "0",
		"threshold_percent": "101",
		"critical_percent":  "-1",
		"quiet":             "3h",
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

func TestSetAlertConfigValueQuietChoices(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	// Arming a preset choice stores an RFC3339 quiet_until about N hours out.
	if err := setAlertConfigValue("quiet", "4h"); err != nil {
		t.Fatalf("set quiet 4h: %v", err)
	}
	until, err := time.Parse(time.RFC3339, viper.GetString("usage_alert_quiet_until"))
	if err != nil {
		t.Fatalf("quiet_until = %q, want RFC3339: %v", viper.GetString("usage_alert_quiet_until"), err)
	}
	remaining := time.Until(until)
	if remaining < 3*time.Hour+50*time.Minute || remaining > 4*time.Hour+10*time.Minute {
		t.Fatalf("quiet_until remaining = %v, want ~4h", remaining)
	}

	// "off" (and its aliases) clears the timer.
	for _, choice := range []string{"off", "0", ""} {
		if err := setAlertConfigValue("quiet", choice); err != nil {
			t.Fatalf("set quiet %q: %v", choice, err)
		}
		if got := viper.GetString("usage_alert_quiet_until"); got != "" {
			t.Fatalf("quiet choice %q left quiet_until = %q, want empty", choice, got)
		}
	}

	// Durations outside the preset choices are rejected.
	if err := setAlertConfigValue("quiet", "3h"); err == nil {
		t.Fatal("expected quiet 3h to be rejected")
	}
}

func TestCycleAlertQuietValue_WalksChoices_andDisarmsRunningTimerFirst(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	got := "off"
	want := []string{"1h", "2h", "4h", "6h", "12h", "off"}
	for _, expected := range want {
		got = cycleAlertQuietValue(got)
		if got != expected {
			t.Fatalf("cycle from start = %q, want %q", got, expected)
		}
	}

	if cycled := cycleAlertQuietValue("on (1h 23m left)"); cycled != "off" {
		t.Fatalf("cycle of armed display = %q, want off", cycled)
	}
}

func TestAlertQuietDraftValue_ReportsRemainingTimeForArmedTimer(t *testing.T) {
	if got := alertQuietDraftValue(time.Time{}); got != "off" {
		t.Fatalf("zero QuietUntil draft value = %q, want off", got)
	}
	if got := alertQuietDraftValue(time.Now().Add(-time.Minute)); got != "off" {
		t.Fatalf("expired timer draft value = %q, want off", got)
	}
	if got := alertQuietDraftValue(time.Now().Add(90 * time.Minute)); !strings.HasPrefix(got, "on (1h ") || !strings.HasSuffix(got, "m left)") {
		t.Fatalf("armed draft value = %q, want on (1h Nm left)", got)
	}
}

func TestAlertSettingsDraft_InheritsGlobalDefaultForWindows_whenWindowOverridesAbsent(t *testing.T) {
	// Given: runtime evaluation falls back window -> default -> legacy percent,
	// so the draft must display the inherited default, not the legacy percent.
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("usage_alert_threshold_percent", 70.0)
	viper.Set("usage_alert_thresholds", map[string]any{"default": 65.0})

	// When
	draft := alertSettingsDraftFromViper()

	// Then
	for _, key := range []string{"threshold.default", "threshold.5h", "threshold.7d"} {
		if got := draft.values[key]; got != "65" {
			t.Fatalf("%s = %q, want inherited default 65", key, got)
		}
	}
}

func TestAlertSettingsModel_InsertsQWhileEditing_andCancelsOnlyFromNavigation(t *testing.T) {
	// Given
	viper.Reset()
	t.Cleanup(viper.Reset)
	m := newAlertSettingsModel(alertSettingsDraftFromViper())
	for i := range m.items {
		m.items[i].cursor = m.items[i].key == "cooldown_minutes"
	}
	m.items[m.index()].value = ""
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(alertSettingsModel)

	// When: free text can contain "q".
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	m = updated.(alertSettingsModel)

	// Then
	if m.cancelled {
		t.Fatal("q while editing cancelled the form; want insertion")
	}
	if m.editingValue != "q" {
		t.Fatalf("editingValue = %q, want q", m.editingValue)
	}

	// When editing is dismissed, plain q cancels again.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(alertSettingsModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	m = updated.(alertSettingsModel)
	if !m.cancelled || !m.done {
		t.Fatalf("q from navigation = cancelled:%t done:%t, want cancel", m.cancelled, m.done)
	}
}
