package cmd

import (
	"encoding/json"
	"fmt"
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
		case "quiet_hours":
			viper.Set("usage_alert_quiet_hours", val)
		case "timezone":
			viper.Set("usage_alert_timezone", val)
		default:
			fmt.Println("supported keys: enabled, cooldown_minutes, threshold_percent, quiet_hours, timezone")
			return
		}
		if err := writeConfig(); err != nil {
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

		fmt.Printf("provider=%s window=%s value=%.1f threshold=%.1f quiet_hours=%s\n", provider, window, value, threshold, cfg.QuietHours)
		fmt.Println("test executed (notification may be suppressed by cooldown/quiet hours).")
	},
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
	alertConfigCmd.AddCommand(alertConfigShowCmd)
	alertConfigCmd.AddCommand(alertConfigSetCmd)

	alertTestCmd.Flags().String("provider", "codex", "provider name")
	alertTestCmd.Flags().String("window", "5h", "window key (e.g. 5h, 7d, current)")
	alertTestCmd.Flags().Float64("value", 90, "synthetic usage value percent")
	alertTestCmd.Flags().Bool("quiet-now", false, "force quiet-hours simulation")
}
