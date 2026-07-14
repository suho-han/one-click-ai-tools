package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"github.com/suho-han/one-click-ai-tools/internal/schedule"
	"github.com/suho-han/one-click-ai-tools/internal/update"
)

type configToolStatus struct {
	Name       string `json:"name"`
	BinaryName string `json:"binary_name"`
	Enabled    bool   `json:"enabled"`
}

type configSnapshot struct {
	ConfigFile             string             `json:"config_file"`
	UsageDisplayMode       string             `json:"usage_display_mode"`
	SessionRefreshEnabled  bool               `json:"session_refresh_enabled"`
	SessionRefreshInterval string             `json:"session_refresh_interval"`
	SessionRefreshHour     int                `json:"session_refresh_hour"`
	Tools                  []configToolStatus `json:"tools"`
}

type configUpdatePayload struct {
	EnabledTools           []string `json:"enabled_tools"`
	UsageDisplayMode       string   `json:"usage_display_mode"`
	SessionRefreshEnabled  *bool    `json:"session_refresh_enabled"`
	SessionRefreshInterval string   `json:"session_refresh_interval"`
	SessionRefreshHour     *int     `json:"session_refresh_hour"`
}

func parseConfigUpdatePayload(raw string) (configUpdatePayload, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return configUpdatePayload{}, fmt.Errorf("missing --json payload")
	}
	var payload configUpdatePayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return configUpdatePayload{}, fmt.Errorf("invalid config update json: %w", err)
	}
	return payload, nil
}

func buildConfigSnapshot(configFile string) configSnapshot {
	enabledTools := viper.GetStringSlice("enabled_tools")
	tools := make([]configToolStatus, 0, len(update.Tools))
	for _, tool := range update.Tools {
		tools = append(tools, configToolStatus{
			Name:       tool.Name,
			BinaryName: tool.BinaryName,
			Enabled:    configToolEnabled(enabledTools, tool),
		})
	}

	return configSnapshot{
		ConfigFile:             configFile,
		UsageDisplayMode:       normalizedConfigUsageMode(viper.GetString("usage_display_mode")),
		SessionRefreshEnabled:  viper.GetBool("session_refresh_enabled"),
		SessionRefreshInterval: normalizedConfigRefreshInterval(viper.GetString("session_refresh_interval")),
		SessionRefreshHour:     normalizedConfigRefreshHour(viper.GetInt("session_refresh_hour")),
		Tools:                  tools,
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
	interval, shouldSetInterval, err := normalizeConfigUpdateInterval(payload.SessionRefreshInterval)
	if err != nil {
		return err
	}
	hour, shouldSetHour, err := normalizeConfigUpdateHour(payload.SessionRefreshHour)
	if err != nil {
		return err
	}

	if shouldSetTools {
		viper.Set("enabled_tools", normalizedTools)
		viper.Set("agent_order", orderedConfigToolNames())
	}
	if shouldSetUsageMode {
		viper.Set("usage_display_mode", usageMode)
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
	return nil
}

func configToolEnabled(enabledTools []string, tool update.Tool) bool {
	if len(enabledTools) == 0 {
		return true
	}
	for _, enabled := range enabledTools {
		if tool.MatchesName(enabled) {
			return true
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
		tool, ok := canonicalConfigTool(rawTool)
		if !ok {
			return nil, false, fmt.Errorf("unknown provider: %s", rawTool)
		}
		if seen[tool.BinaryName] {
			continue
		}
		seen[tool.BinaryName] = true
		normalized = append(normalized, tool.BinaryName)
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
	rawMode = strings.TrimSpace(strings.ToLower(rawMode))
	if rawMode != "used" && rawMode != "remaining" {
		return "remaining"
	}
	return rawMode
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

func orderedConfigToolNames() []string {
	names := make([]string, 0, len(update.Tools))
	for _, tool := range update.Tools {
		names = append(names, tool.BinaryName)
	}
	return names
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
