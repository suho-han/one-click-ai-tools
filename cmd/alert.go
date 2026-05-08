package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/suho-han/one-click-tools/internal/notify"
	"github.com/suho-han/one-click-tools/internal/usage"
)

var alertCmd = &cobra.Command{
	Use:   "alert",
	Short: "Usage alert configuration and testing",
}

var alertConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage usage alert config",
}

var alertConfigShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show effective usage alert config",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := buildAlertConfigFromViper(true)
		payload, _ := json.MarshalIndent(cfg, "", "  ")
		fmt.Println(string(payload))
	},
}

var alertConfigSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set usage alert config value",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		key := strings.TrimSpace(args[0])
		val := strings.TrimSpace(args[1])
		switch key {
		case "enabled":
			viper.Set("usage_alert_enabled", val == "1" || strings.EqualFold(val, "true") || strings.EqualFold(val, "yes"))
		case "cooldown_minutes":
			n, err := strconv.Atoi(val)
			if err != nil {
				fmt.Printf("invalid cooldown_minutes: %v\n", err)
				return
			}
			viper.Set("usage_alert_cooldown_minutes", n)
		case "threshold_percent":
			f, err := strconv.ParseFloat(val, 64)
			if err != nil {
				fmt.Printf("invalid threshold_percent: %v\n", err)
				return
			}
			viper.Set("usage_alert_threshold_percent", f)
		case "critical_percent":
			f, err := strconv.ParseFloat(val, 64)
			if err != nil {
				fmt.Printf("invalid critical_percent: %v\n", err)
				return
			}
			viper.Set("usage_alert_critical_percent", f)
		case "quiet_hours":
			viper.Set("usage_alert_quiet_hours", val)
		case "timezone":
			viper.Set("usage_alert_timezone", val)
		default:
			fmt.Println("supported keys: enabled, cooldown_minutes, threshold_percent, critical_percent, quiet_hours, timezone")
			return
		}
		if err := persistViperConfig(); err != nil {
			fmt.Printf("failed to write config: %v\n", err)
			return
		}
		fmt.Println("alert config updated.")
	},
}

var alertTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Test usage alert decision with synthetic input",
	Run: func(cmd *cobra.Command, args []string) {
		provider, _ := cmd.Flags().GetString("provider")
		window, _ := cmd.Flags().GetString("window")
		value, _ := cmd.Flags().GetFloat64("value")
		quietNow, _ := cmd.Flags().GetBool("quiet-now")

		if provider == "" {
			provider = "codex"
		}
		if window == "" {
			window = "5h"
		}

		cfg := buildAlertConfigFromViper(true)
		statePath := viper.GetString("usage_alert_state_path")
		if statePath == "" {
			statePath = ""
		}
		cfg.StatePath = statePath

		r := usage.UsageResult{Provider: provider, Unit: "percent", Used: fmt.Sprintf("%.1f", value), Buckets: map[string]string{window: fmt.Sprintf("%.1f", value)}}
		now := time.Now()
		if quietNow {
			viper.Set("usage_alert_quiet_hours", "00:00-23:59")
			cfg.QuietHours = "00:00-23:59"
		}
		if err := notify.MaybeSendUsageAlerts([]usage.UsageResult{r}, cfg, now); err != nil {
			fmt.Printf("test failed: %v\n", err)
			return
		}

		threshold := cfg.ThresholdPct
		if t, ok := cfg.GlobalThresholds[window]; ok && t > 0 {
			threshold = t
		}
		if pm, ok := cfg.ProviderThreshold[strings.ToLower(provider)]; ok {
			if t, ok := pm[window]; ok && t > 0 {
				threshold = t
			} else if t, ok := pm["default"]; ok && t > 0 {
				threshold = t
			}
		}

		fmt.Printf("provider=%s window=%s value=%.1f threshold=%.1f quiet_hours=%s critical=%.1f\n", provider, window, value, threshold, cfg.QuietHours, cfg.CriticalPct)
		fmt.Println("test executed (notification may be suppressed by cooldown/quiet hours/snooze).")
	},
}

var alertSnoozeCmd = &cobra.Command{Use: "snooze", Short: "Manage alert snooze"}

var alertSnoozeSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set snooze duration",
	Run: func(cmd *cobra.Command, args []string) {
		duration, _ := cmd.Flags().GetDuration("duration")
		provider, _ := cmd.Flags().GetString("provider")
		window, _ := cmd.Flags().GetString("window")
		if duration <= 0 {
			fmt.Println("duration must be > 0, e.g. --duration 2h")
			return
		}
		statePath := getAlertStatePath()
		until := time.Now().Add(duration)
		if err := notify.SetSnooze(statePath, provider, window, until); err != nil {
			fmt.Printf("failed to set snooze: %v\n", err)
			return
		}
		fmt.Printf("snooze set key=%s until=%s\n", snoozeDisplayKey(provider, window), until.Format(time.RFC3339))
	},
}

var alertSnoozeShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show active snoozes",
	Run: func(cmd *cobra.Command, args []string) {
		statePath := getAlertStatePath()
		m, err := notify.GetSnooze(statePath)
		if err != nil {
			fmt.Printf("failed to load snooze: %v\n", err)
			return
		}
		if len(m) == 0 {
			fmt.Println("no active snooze")
			return
		}
		now := time.Now()
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			until := m[k]
			status := "expired"
			if now.Before(until) {
				status = "active"
			}
			fmt.Printf("%s -> %s (%s)\n", k, until.Format(time.RFC3339), status)
		}
	},
}

var alertSnoozeClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear snooze",
	Run: func(cmd *cobra.Command, args []string) {
		provider, _ := cmd.Flags().GetString("provider")
		window, _ := cmd.Flags().GetString("window")
		statePath := getAlertStatePath()
		if err := notify.ClearSnooze(statePath, provider, window); err != nil {
			fmt.Printf("failed to clear snooze: %v\n", err)
			return
		}
		fmt.Printf("snooze cleared key=%s\n", snoozeDisplayKey(provider, window))
	},
}

func getAlertStatePath() string {
	p := strings.TrimSpace(viper.GetString("usage_alert_state_path"))
	if p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".oct/state/usage-alert-state.json"
	}
	return filepath.Join(home, ".oct", "state", "usage-alert-state.json")
}

func persistViperConfig() error {
	cfg := viper.ConfigFileUsed()
	if strings.TrimSpace(cfg) == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		cfg = filepath.Join(home, ".oct", "config.yaml")
	}
	if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(cfg); err == nil {
		return viper.WriteConfigAs(cfg)
	}
	return viper.SafeWriteConfigAs(cfg)
}

func snoozeDisplayKey(provider, window string) string {
	provider = strings.TrimSpace(provider)
	window = strings.TrimSpace(window)
	if provider == "" && window == "" {
		return "global"
	}
	if provider != "" && window == "" {
		return "provider:" + provider
	}
	if provider == "" {
		return "window:" + window
	}
	return "provider:" + provider + ":window:" + window
}

func parseThresholdMap(raw map[string]any) map[string]float64 {
	out := map[string]float64{}
	for k, v := range raw {
		switch x := v.(type) {
		case int:
			out[strings.ToLower(k)] = float64(x)
		case int64:
			out[strings.ToLower(k)] = float64(x)
		case float64:
			out[strings.ToLower(k)] = x
		case string:
			if f, err := strconv.ParseFloat(strings.TrimSpace(x), 64); err == nil {
				out[strings.ToLower(k)] = f
			}
		}
	}
	return out
}

func parseProviderThresholdMap(raw map[string]any) map[string]map[string]float64 {
	out := map[string]map[string]float64{}
	for provider, v := range raw {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		out[strings.ToLower(provider)] = parseThresholdMap(m)
	}
	return out
}

func buildAlertConfigFromViper(enabled bool) notify.UsageAlertConfig {
	cfg := notify.UsageAlertConfig{
		Enabled:           enabled,
		ThresholdPct:      viper.GetFloat64("usage_alert_threshold_percent"),
		CooldownMinutes:   viper.GetInt("usage_alert_cooldown_minutes"),
		StatePath:         viper.GetString("usage_alert_state_path"),
		QuietHours:        viper.GetString("usage_alert_quiet_hours"),
		Timezone:          viper.GetString("usage_alert_timezone"),
		CriticalPct:       viper.GetFloat64("usage_alert_critical_percent"),
		GlobalThresholds:  parseThresholdMap(viper.GetStringMap("usage_alert_thresholds")),
		ProviderThreshold: parseProviderThresholdMap(viper.GetStringMap("usage_alert_provider_thresholds")),
	}
	if cfg.GlobalThresholds == nil {
		cfg.GlobalThresholds = map[string]float64{}
	}
	if _, ok := cfg.GlobalThresholds["default"]; !ok && cfg.ThresholdPct > 0 {
		cfg.GlobalThresholds["default"] = cfg.ThresholdPct
	}
	return cfg
}

func init() {
	rootCmd.AddCommand(alertCmd)
	alertCmd.AddCommand(alertConfigCmd)
	alertCmd.AddCommand(alertTestCmd)
	alertCmd.AddCommand(alertSnoozeCmd)
	alertConfigCmd.AddCommand(alertConfigShowCmd)
	alertConfigCmd.AddCommand(alertConfigSetCmd)
	alertSnoozeCmd.AddCommand(alertSnoozeSetCmd)
	alertSnoozeCmd.AddCommand(alertSnoozeShowCmd)
	alertSnoozeCmd.AddCommand(alertSnoozeClearCmd)

	alertTestCmd.Flags().String("provider", "codex", "provider name")
	alertTestCmd.Flags().String("window", "5h", "window key (e.g. 5h, 7d, current)")
	alertTestCmd.Flags().Float64("value", 90, "synthetic usage value percent")
	alertTestCmd.Flags().Bool("quiet-now", false, "force quiet-hours simulation")

	alertSnoozeSetCmd.Flags().Duration("duration", 2*time.Hour, "snooze duration")
	alertSnoozeSetCmd.Flags().String("provider", "", "provider scope")
	alertSnoozeSetCmd.Flags().String("window", "", "window scope")
	alertSnoozeClearCmd.Flags().String("provider", "", "provider scope")
	alertSnoozeClearCmd.Flags().String("window", "", "window scope")
}
