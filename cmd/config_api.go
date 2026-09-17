package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/suho-han/one-click-ai-tools/internal/schedule"
	"github.com/suho-han/one-click-ai-tools/internal/update"
	"github.com/suho-han/one-click-ai-tools/internal/usage"
)

type configToolStatus struct {
	Name       string `json:"name"`
	BinaryName string `json:"binary_name"`
	Enabled    bool   `json:"enabled"`
}

type configSnapshot struct {
	ConfigFile             string              `json:"config_file"`
	UsageDisplayMode       string              `json:"usage_display_mode"`
	MenubarTitleMode       string              `json:"menubar_title_mode"`
	MenubarRefreshInterval string              `json:"menubar_refresh_interval"`
	SessionRefreshEnabled  bool                `json:"session_refresh_enabled"`
	SessionRefreshInterval string              `json:"session_refresh_interval"`
	SessionRefreshHour     int                 `json:"session_refresh_hour"`
	AgentOrder             []string            `json:"agent_order"`
	Tools                  []configToolStatus  `json:"tools"`
	Alert                  configAlertSnapshot `json:"alert"`
}

type configAlertSnapshot struct {
	Enabled          bool                  `json:"enabled"`
	ThresholdPercent float64               `json:"threshold_percent"`
	CriticalPercent  float64               `json:"critical_percent"`
	CooldownMinutes  int                   `json:"cooldown_minutes"`
	QuietHours       string                `json:"quiet_hours"`
	Timezone         string                `json:"timezone"`
	Thresholds       configAlertThresholds `json:"thresholds"`
}

type configAlertThresholds struct {
	Default   float64 `json:"default"`
	FiveHours float64 `json:"5h"`
	SevenDays float64 `json:"7d"`
}

type configUpdatePayload struct {
	EnabledTools           []string                  `json:"enabled_tools"`
	UsageDisplayMode       string                    `json:"usage_display_mode"`
	MenubarTitleMode       string                    `json:"menubar_title_mode"`
	SessionRefreshEnabled  *bool                     `json:"session_refresh_enabled"`
	SessionRefreshInterval string                    `json:"session_refresh_interval"`
	SessionRefreshHour     *int                      `json:"session_refresh_hour"`
	AgentOrder             []string                  `json:"agent_order"`
	Alert                  *configAlertUpdatePayload `json:"alert"`
}

type configAlertUpdatePayload struct {
	Enabled          *bool                        `json:"enabled"`
	ThresholdPercent *float64                     `json:"threshold_percent"`
	CriticalPercent  *float64                     `json:"critical_percent"`
	CooldownMinutes  *int                         `json:"cooldown_minutes"`
	QuietHours       *string                      `json:"quiet_hours"`
	Timezone         *string                      `json:"timezone"`
	Thresholds       *configAlertThresholdPayload `json:"thresholds"`
}

type configAlertThresholdPayload struct {
	Default   *float64 `json:"default"`
	FiveHours *float64 `json:"5h"`
	SevenDays *float64 `json:"7d"`
}

type configAlertUpdate struct {
	Enabled          *bool
	ThresholdPercent *float64
	CriticalPercent  *float64
	CooldownMinutes  *int
	QuietHours       *string
	Timezone         *string
	Thresholds       *configAlertThresholdUpdate
}

type configAlertThresholdUpdate struct {
	Default   *float64
	FiveHours *float64
	SevenDays *float64
}

func parseConfigUpdatePayload(raw string) (configUpdatePayload, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return configUpdatePayload{}, fmt.Errorf("missing --payload payload")
	}
	var payload configUpdatePayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return configUpdatePayload{}, fmt.Errorf("invalid config update json: %w", err)
	}
	return payload, nil
}

func buildConfigSnapshot(configFile string) configSnapshot {
	enabledTools := viper.GetStringSlice("enabled_tools")
	rawOrder := viper.GetStringSlice("agent_order")
	orderedTools := update.GetOrderedTools(rawOrder)

	toolByBinary := make(map[string]update.Tool, len(orderedTools))
	for _, tool := range orderedTools {
		toolByBinary[tool.BinaryName] = tool
	}

	agentOrder := make([]string, 0, len(orderedTools))
	tools := make([]configToolStatus, 0, len(orderedTools))
	seen := map[string]bool{}
	var appendTool func(update.Tool)
	var appendStandalone func(usage.Provider)
	appendTool = func(tool update.Tool) {
		if seen[tool.BinaryName] {
			return
		}
		seen[tool.BinaryName] = true
		agentOrder = append(agentOrder, tool.BinaryName)
		tools = append(tools, configToolStatus{
			Name:       tool.Name,
			BinaryName: tool.BinaryName,
			Enabled:    configToolEnabled(enabledTools, tool),
		})
	}
	appendStandalone = func(p usage.Provider) {
		if seen[p.Name] {
			return
		}
		seen[p.Name] = true
		agentOrder = append(agentOrder, p.Name)
		tools = append(tools, configToolStatus{
			Name:       p.Name,
			BinaryName: p.Name,
			Enabled:    standaloneProviderEnabled(enabledTools, p),
		})
	}

	// Walk the requested order first so installable tools and standalone
	// usage providers appear exactly where the user put them.
	for _, entry := range rawOrder {
		for _, part := range strings.Split(entry, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if p, ok := usage.LookupStandalone(part); ok {
				appendStandalone(p)
				continue
			}
			if tool, ok := toolByBinary[update.NormalizeToolName(part)]; ok {
				appendTool(tool)
			}
		}
	}
	// An unset enabled_tools means "every installable tool", so the whole
	// catalog must surface: a settings save round-trips this list, and a
	// shorter one would shrink the effective enabled set.
	if len(enabledTools) == 0 {
		for _, tool := range orderedTools {
			appendTool(tool)
		}
	}
	// Entries enabled in config but absent from agent_order (installable or
	// standalone) must still surface, or a settings save would silently drop
	// them from enabled_tools. With a curated enabled_tools this keeps the
	// snapshot equal to the file instead of padding it with catalog defaults.
	for _, name := range enabledTools {
		for _, part := range strings.Split(name, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if p, ok := usage.LookupStandalone(part); ok {
				appendStandalone(p)
				continue
			}
			if tool, ok := toolByBinary[update.NormalizeToolName(part)]; ok {
				appendTool(tool)
			}
		}
	}

	return configSnapshot{
		ConfigFile:             configFile,
		UsageDisplayMode:       normalizedConfigUsageMode(viper.GetString("usage_display_mode")),
		MenubarTitleMode:       normalizedMenubarTitleMode(viper.GetString("menubar_title_mode")),
		MenubarRefreshInterval: normalizedMenubarRefreshInterval(viper.GetString("menubar_refresh_interval")),
		SessionRefreshEnabled:  viper.GetBool("session_refresh_enabled"),
		SessionRefreshInterval: normalizedConfigRefreshInterval(viper.GetString("session_refresh_interval")),
		SessionRefreshHour:     normalizedConfigRefreshHour(viper.GetInt("session_refresh_hour")),
		AgentOrder:             agentOrder,
		Tools:                  tools,
		Alert:                  buildConfigAlertSnapshot(),
	}
}

func applyConfigUpdate(payload configUpdatePayload) error {
	normalizedTools, shouldSetTools, err := normalizeConfigUpdateTools(payload.EnabledTools)
	if err != nil {
		return err
	}
	usageMode, shouldSetUsageMode, err := normalizeConfigUpdateUsageMode(payload.UsageDisplayMode)
	if err != nil {
		return err
	}
	titleMode, shouldSetTitleMode, err := normalizeConfigUpdateMenubarTitleMode(payload.MenubarTitleMode)
	if err != nil {
		return err
	}
	interval, shouldSetInterval, err := normalizeConfigUpdateInterval(payload.SessionRefreshInterval)
	if err != nil {
		return err
	}
	hour, shouldSetHour, err := normalizeConfigUpdateHour(payload.SessionRefreshHour)
	if err != nil {
		return err
	}
	agentOrder, shouldSetOrder, err := normalizeConfigUpdateOrder(payload.AgentOrder)
	if err != nil {
		return err
	}
	alert, err := normalizeConfigAlertUpdate(payload.Alert)
	if err != nil {
		return err
	}

	if shouldSetTools {
		viper.Set("enabled_tools", normalizedTools)
	}
	if shouldSetOrder {
		viper.Set("agent_order", agentOrder)
	}
	if shouldSetUsageMode {
		viper.Set("usage_display_mode", usageMode)
	}
	if shouldSetTitleMode {
		viper.Set("menubar_title_mode", titleMode)
	}
	if payload.SessionRefreshEnabled != nil {
		viper.Set("session_refresh_enabled", *payload.SessionRefreshEnabled)
	}
	if shouldSetInterval {
		viper.Set("session_refresh_interval", interval)
	}
	if shouldSetHour {
		viper.Set("session_refresh_hour", hour)
	}
	applyConfigAlertUpdate(alert)
	return nil
}

func buildConfigAlertSnapshot() configAlertSnapshot {
	alert := buildAlertConfigFromViper(viper.GetBool("usage_alert_enabled"))
	// Match alert evaluation's effective defaults: a config explicitly storing
	// 0 is treated as unset, and normalizeConfigAlertUpdate rejects 0, so a
	// raw snapshot would deadlock every menubar save until repaired by hand.
	threshold := alert.ThresholdPct
	if threshold <= 0 {
		threshold = 80
	}
	critical := alert.CriticalPct
	if critical <= 0 {
		critical = 98
	}
	cooldown := alert.CooldownMinutes
	if cooldown <= 0 {
		cooldown = 360
	}
	// Runtime evaluation inherits 5h/7d windows from the global default
	// (thresholdFor: window -> default -> legacy percent). The Swift settings
	// payload always sends the complete alert object, so reporting the legacy
	// percent here would persist it as an explicit window override on save.
	effectiveDefault := configAlertThreshold(alert.GlobalThresholds, "default", threshold)
	return configAlertSnapshot{
		Enabled:          alert.Enabled,
		ThresholdPercent: threshold,
		CriticalPercent:  critical,
		CooldownMinutes:  cooldown,
		QuietHours:       alert.QuietHours,
		Timezone:         alert.Timezone,
		Thresholds: configAlertThresholds{
			Default:   effectiveDefault,
			FiveHours: configAlertThreshold(alert.GlobalThresholds, "5h", effectiveDefault),
			SevenDays: configAlertThreshold(alert.GlobalThresholds, "7d", effectiveDefault),
		},
	}
}

func configAlertThreshold(thresholds map[string]float64, window string, fallback float64) float64 {
	if value, ok := thresholds[window]; ok && value > 0 {
		return value
	}
	return fallback
}

func normalizeConfigAlertUpdate(payload *configAlertUpdatePayload) (configAlertUpdate, error) {
	if payload == nil {
		return configAlertUpdate{}, nil
	}
	thresholdPercent, err := normalizeConfigAlertPercent("threshold_percent", payload.ThresholdPercent)
	if err != nil {
		return configAlertUpdate{}, err
	}
	criticalPercent, err := normalizeConfigAlertPercent("critical_percent", payload.CriticalPercent)
	if err != nil {
		return configAlertUpdate{}, err
	}
	if payload.CooldownMinutes != nil && *payload.CooldownMinutes <= 0 {
		return configAlertUpdate{}, fmt.Errorf("invalid cooldown_minutes %d: must be a positive integer", *payload.CooldownMinutes)
	}

	var quietHours *string
	if payload.QuietHours != nil {
		value := strings.TrimSpace(*payload.QuietHours)
		if err := validateAlertQuietHours(value); err != nil {
			return configAlertUpdate{}, err
		}
		quietHours = &value
	}

	var timezone *string
	if payload.Timezone != nil {
		value := strings.TrimSpace(*payload.Timezone)
		if value != "" {
			if _, err := time.LoadLocation(value); err != nil {
				return configAlertUpdate{}, fmt.Errorf("invalid timezone %q: %w", value, err)
			}
		}
		timezone = &value
	}

	thresholds, err := normalizeConfigAlertThresholds(payload.Thresholds)
	if err != nil {
		return configAlertUpdate{}, err
	}
	return configAlertUpdate{
		Enabled:          payload.Enabled,
		ThresholdPercent: thresholdPercent,
		CriticalPercent:  criticalPercent,
		CooldownMinutes:  payload.CooldownMinutes,
		QuietHours:       quietHours,
		Timezone:         timezone,
		Thresholds:       thresholds,
	}, nil
}

func normalizeConfigAlertPercent(name string, value *float64) (*float64, error) {
	if value == nil {
		return nil, nil
	}
	parsed, err := parseAlertPercent(name, strconv.FormatFloat(*value, 'f', -1, 64))
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func normalizeConfigAlertThresholds(payload *configAlertThresholdPayload) (*configAlertThresholdUpdate, error) {
	if payload == nil {
		return nil, nil
	}
	defaultThreshold, err := normalizeConfigAlertPercent("thresholds.default", payload.Default)
	if err != nil {
		return nil, err
	}
	fiveHours, err := normalizeConfigAlertPercent("thresholds.5h", payload.FiveHours)
	if err != nil {
		return nil, err
	}
	sevenDays, err := normalizeConfigAlertPercent("thresholds.7d", payload.SevenDays)
	if err != nil {
		return nil, err
	}
	return &configAlertThresholdUpdate{
		Default:   defaultThreshold,
		FiveHours: fiveHours,
		SevenDays: sevenDays,
	}, nil
}

func applyConfigAlertUpdate(alert configAlertUpdate) {
	if alert.Enabled != nil {
		viper.Set("usage_alert_enabled", *alert.Enabled)
	}
	if alert.ThresholdPercent != nil {
		viper.Set("usage_alert_threshold_percent", *alert.ThresholdPercent)
	}
	if alert.CriticalPercent != nil {
		viper.Set("usage_alert_critical_percent", *alert.CriticalPercent)
	}
	if alert.CooldownMinutes != nil {
		viper.Set("usage_alert_cooldown_minutes", *alert.CooldownMinutes)
	}
	if alert.QuietHours != nil {
		viper.Set("usage_alert_quiet_hours", *alert.QuietHours)
	}
	if alert.Timezone != nil {
		viper.Set("usage_alert_timezone", *alert.Timezone)
	}
	if alert.Thresholds == nil {
		return
	}
	if alert.Thresholds.Default != nil {
		viper.Set("usage_alert_thresholds.default", *alert.Thresholds.Default)
	}
	if alert.Thresholds.FiveHours != nil {
		viper.Set("usage_alert_thresholds.5h", *alert.Thresholds.FiveHours)
	}
	if alert.Thresholds.SevenDays != nil {
		viper.Set("usage_alert_thresholds.7d", *alert.Thresholds.SevenDays)
	}
}

func configToolEnabled(enabledTools []string, tool update.Tool) bool {
	if len(enabledTools) == 0 {
		return true
	}
	for _, enabled := range enabledTools {
		// Comma-joined entries are legal config values (GetStringSlice keeps
		// them intact) and every other consumer splits them, so a snapshot
		// flag must not disagree with what usage actually fetches.
		for _, part := range strings.Split(enabled, ",") {
			if tool.MatchesName(strings.TrimSpace(part)) {
				return true
			}
		}
	}
	return false
}

func normalizeConfigUpdateTools(rawTools []string) ([]string, bool, error) {
	if rawTools == nil {
		return nil, false, nil
	}
	if len(rawTools) == 0 {
		return nil, false, fmt.Errorf("enabled_tools must include at least one provider")
	}
	normalized := make([]string, 0, len(rawTools))
	seen := map[string]bool{}
	for _, rawTool := range rawTools {
		name, ok := canonicalConfigProvider(rawTool)
		if !ok {
			return nil, false, fmt.Errorf("unknown provider: %s", rawTool)
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		normalized = append(normalized, name)
	}
	return normalized, true, nil
}

// normalizeConfigUpdateOrder validates and canonicalizes the order sent by a
// settings save. It deliberately does not pad the list with catalog
// defaults: the snapshot mirrors the config file, so the save must write
// back exactly what the UI showed the user.
func normalizeConfigUpdateOrder(rawOrder []string) ([]string, bool, error) {
	if rawOrder == nil {
		return nil, false, nil
	}
	if len(rawOrder) == 0 {
		return nil, false, fmt.Errorf("agent_order must include at least one provider")
	}

	normalized := make([]string, 0, len(rawOrder))
	seen := map[string]bool{}
	for _, rawTool := range rawOrder {
		name, ok := canonicalConfigProvider(rawTool)
		if !ok {
			return nil, false, fmt.Errorf("unknown provider in agent_order: %s", rawTool)
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		normalized = append(normalized, name)
	}
	return normalized, true, nil
}

func normalizeConfigUpdateUsageMode(rawMode string) (string, bool, error) {
	rawMode = strings.TrimSpace(strings.ToLower(rawMode))
	if rawMode == "" {
		return "", false, nil
	}
	if rawMode != "used" && rawMode != "remaining" {
		return "", false, fmt.Errorf("invalid usage_display_mode %q (use used or remaining)", rawMode)
	}
	return rawMode, true, nil
}

func normalizeConfigUpdateMenubarTitleMode(rawMode string) (string, bool, error) {
	rawMode = strings.TrimSpace(strings.ToLower(rawMode))
	if rawMode == "" {
		return "", false, nil
	}
	if rawMode != "oct" && rawMode != "compact" {
		return "", false, fmt.Errorf("invalid menubar_title_mode %q (use oct or compact)", rawMode)
	}
	return rawMode, true, nil
}

func normalizeConfigUpdateInterval(rawInterval string) (string, bool, error) {
	rawInterval = strings.TrimSpace(rawInterval)
	if rawInterval == "" {
		return "", false, nil
	}
	interval, err := schedule.ParseInterval(rawInterval)
	if err != nil {
		return "", false, err
	}
	return interval, true, nil
}

func normalizeConfigUpdateHour(rawHour *int) (int, bool, error) {
	if rawHour == nil {
		return 0, false, nil
	}
	if *rawHour < 0 || *rawHour > 23 {
		return 0, false, fmt.Errorf("invalid session_refresh_hour %d (must be 0-23)", *rawHour)
	}
	return *rawHour, true, nil
}

func normalizedConfigUsageMode(rawMode string) string {
	return usage.NormalizeDisplayMode(rawMode)
}

func normalizedMenubarTitleMode(rawMode string) string {
	rawMode = strings.TrimSpace(strings.ToLower(rawMode))
	if rawMode != "oct" && rawMode != "compact" {
		return "oct"
	}
	return rawMode
}

// normalizedMenubarRefreshInterval mirrors the Go menubar's own parser
// (menubarRefreshInterval): Go-duration strings, defaulting to "1m" when
// empty or invalid, so the Swift app receives exactly what the legacy
// menubar would act on.
func normalizedMenubarRefreshInterval(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "1m"
	}
	if _, err := time.ParseDuration(raw); err != nil {
		return "1m"
	}
	return raw
}

func normalizedConfigRefreshInterval(rawInterval string) string {
	interval, err := schedule.ParseInterval(rawInterval)
	if err != nil {
		return "daily"
	}
	return interval
}

func normalizedConfigRefreshHour(rawHour int) int {
	if rawHour < 0 || rawHour > 23 {
		return 9
	}
	return rawHour
}

func canonicalConfigTool(rawTool string) (update.Tool, bool) {
	for _, tool := range update.Tools {
		if tool.MatchesName(rawTool) {
			return tool, true
		}
	}
	return update.Tool{}, false
}

// canonicalConfigProvider resolves a raw config entry to its canonical config
// value, accepting both installable tools and standalone usage providers.
func canonicalConfigProvider(rawTool string) (string, bool) {
	if tool, ok := canonicalConfigTool(rawTool); ok {
		return tool.BinaryName, true
	}
	if p, ok := usage.LookupStandalone(rawTool); ok {
		return p.Name, true
	}
	return "", false
}

// standaloneProviderEnabled reports whether a standalone usage provider was
// explicitly listed in enabled_tools. Unlike installable tools they are
// opt-in: an empty enabled_tools list does NOT enable them.
func standaloneProviderEnabled(enabledTools []string, p usage.Provider) bool {
	for _, enabled := range enabledTools {
		for _, part := range strings.Split(enabled, ",") {
			if resolved, ok := usage.LookupStandalone(part); ok && resolved.Name == p.Name {
				return true
			}
		}
	}
	return false
}

func configPathForDisplay() string {
	if used := viper.ConfigFileUsed(); used != "" {
		return used
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "$HOME/.oct/config.yaml"
	}
	return filepath.Join(home, ".oct", "config.yaml")
}
