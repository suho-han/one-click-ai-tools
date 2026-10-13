package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/suho-han/one-click-ai-tools/internal/telemetry"
	"github.com/suho-han/one-click-ai-tools/internal/usage"
)

type telemetryContextKey struct{}

type telemetryInvocation struct {
	session        *telemetry.Session
	configPath     string
	explicitConfig bool
}

const analyticsNotice = "oct: Usage analytics is on (self-hosted server; random installation ID, command names, selected provider families, device product family, OS/version, timing and outcome). No prompts, credentials, arguments or error text; IPs are not stored in analytics. Disable: oct config set telemetry-enabled false or DO_NOT_TRACK=1."

func telemetryAllowedSetting(enabled bool) bool {
	if telemetry.Endpoint == "" || !enabled || os.Getenv("DO_NOT_TRACK") == "1" {
		return false
	}
	if ci := strings.TrimSpace(os.Getenv("CI")); ci != "" && ci != "0" && !strings.EqualFold(ci, "false") {
		return false
	}
	if raw := os.Getenv("OCT_TELEMETRY_ENABLED"); raw != "" {
		if _, err := strconv.ParseBool(raw); err != nil {
			return false
		}
	}
	return true
}

func telemetryAllowedAfterCommand(inv *telemetryInvocation) bool {
	if inv == nil || !telemetryAllowedSetting(true) {
		return false
	}
	fresh := viper.New()
	fresh.SetDefault("telemetry_enabled", true)
	fresh.SetEnvPrefix("OCT")
	fresh.AutomaticEnv()
	fresh.SetConfigFile(inv.configPath)
	if err := fresh.ReadInConfig(); err != nil {
		if inv.explicitConfig || !errors.Is(err, os.ErrNotExist) {
			return false
		}
	}
	return telemetryAllowedSetting(fresh.GetBool("telemetry_enabled"))
}

func beginCommandTelemetry(cmd *cobra.Command, _ []string) {
	inv, _ := rootCmd.Context().Value(telemetryContextKey{}).(*telemetryInvocation)
	if inv == nil || !telemetryAllowedSetting(viper.GetBool("telemetry_enabled")) {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	inv.explicitConfig = cfgFile != ""
	inv.configPath = cfgFile
	if !inv.explicitConfig {
		inv.configPath = filepath.Join(home, ".oct", "config.yaml")
	}
	if isTerminalPrompt(cmd.InOrStdin(), cmd.ErrOrStderr()) && (commandEligible(cmd) || cmd == configCmd) {
		showAnalyticsNotice(cmd, home)
	}
	if !commandEligible(cmd) {
		return
	}
	source := "noninteractive"
	if os.Getenv("OCT_TELEMETRY_SOURCE") == "menubar" {
		source = "menubar"
	} else if isTerminalPrompt(cmd.InOrStdin(), cmd.OutOrStdout()) {
		source = "cli"
	}
	if source == "menubar" && (cmd == usageCmd || cmd == menubarDaemonCmd) {
		return
	}
	inv.session = telemetry.Start(rootCmd.Context(), telemetry.Options{
		Endpoint:  telemetry.Endpoint,
		HomeDir:   home,
		Version:   rootCmd.Version,
		Providers: selectedTelemetryProviders(viper.GetStringSlice("enabled_tools")),
	}, telemetry.Command{Path: cmd.CommandPath(), Source: source})
}

// An empty enabled_tools setting means "all tools" to other commands, but
// analytics only reports providers the installation explicitly selected.
func selectedTelemetryProviders(configured []string) []string {
	providers := make([]string, 0, len(configured))
	seen := make(map[string]bool, len(configured))
	for _, part := range splitConfigEntries(configured) {
		{
			name, _, _ := strings.Cut(part, ":")
			canonical, ok := usage.CanonicalProviderName(name)
			if !ok || seen[canonical] {
				continue
			}
			seen[canonical] = true
			providers = append(providers, canonical)
		}
	}
	sort.Strings(providers)
	return providers
}

func commandEligible(cmd *cobra.Command) bool {
	if cmd == rootCmd || cmd == configCmd || cmd == releaseDoctorCmd {
		return false
	}
	for parent := cmd.Parent(); parent != nil; parent = parent.Parent() {
		if parent == configCmd {
			return false
		}
	}
	if cmd.Name() == "help" || cmd.Name() == "completion" {
		return false
	}
	if cmd == usageCmd {
		fromSnapshot, _ := cmd.Flags().GetBool("from-snapshot")
		format, _ := cmd.Flags().GetString("format")
		if fromSnapshot {
			return false
		}
		switch strings.ToLower(strings.TrimSpace(format)) {
		case "waybar", "polybar", "swiftbar":
			return false
		}
	}
	if cmd == agentUpdateCmd {
		check, _ := cmd.Flags().GetBool("check")
		if check {
			return false
		}
	}
	return true
}

func showAnalyticsNotice(cmd *cobra.Command, home string) {
	dir := filepath.Join(home, ".oct", "telemetry")
	marker := filepath.Join(dir, "notice-v1")
	if _, err := os.Lstat(marker); err == nil || !os.IsNotExist(err) {
		return
	}
	if _, err := fmt.Fprintln(cmd.ErrOrStderr(), analyticsNotice); err != nil {
		return
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	file, err := os.OpenFile(marker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err == nil {
		_ = file.Close()
	}
}
