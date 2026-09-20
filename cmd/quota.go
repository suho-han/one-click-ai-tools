package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/suho-han/one-click-ai-tools/internal/quota"
	"github.com/suho-han/one-click-ai-tools/internal/usage"
)

var quotaCmd = &cobra.Command{
	Use:     "quota",
	GroupID: "core",
	Short:   "🎯 Show OpenCode Go quota and cost simulator",
	Long: `Show OpenCode Go quota windows with reset countdowns, and simulate
what the same token mix would cost on other coding models.

Authentication follows the usage command: set OPENCODE_API_KEY or run
'opencode auth login' (opencode-go entry).`,
}

var quotaShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show OpenCode Go quota bars with reset countdowns",
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonMode, _ := cmd.Flags().GetBool("json")
		if endpoint, _ := cmd.Flags().GetString("endpoint"); endpoint != "" {
			os.Setenv("OCT_OPENCODE_USAGE_ENDPOINT", endpoint)
		}

		result := usage.FetchOpenCodeUsage(cmd.Context())
		if jsonMode {
			data, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		}
		return renderQuotaShow(cmd, result)
	},
}

// quotaShowWindows pairs bucket keys with the labels from the feature spec.
var quotaShowWindows = []struct {
	bucket string
	label  string
}{
	{"5h", "Rolling 5-Hour"},
	{"7d", "Weekly Cap"},
	{"1m", "Monthly Cap"},
}

func renderQuotaShow(cmd *cobra.Command, result usage.UsageResult) error {
	out := cmd.OutOrStdout()
	if result.Status == "error" || len(result.Buckets) == 0 {
		fmt.Fprintf(out, "[OpenCode Go Quota Status]\nNo quota data: %s\n", strings.TrimSpace(result.Message))
		return nil
	}

	fmt.Fprintln(out, "[OpenCode Go Quota Status]")
	for _, window := range quotaShowWindows {
		usedRaw, ok := result.Buckets[window.bucket]
		if !ok || usedRaw == "" {
			continue
		}
		var used float64
		if _, err := fmt.Sscanf(usedRaw, "%f", &used); err != nil {
			continue
		}
		line := fmt.Sprintf("- %s: %s %.0f%% left", window.label, quota.RenderBar(100-used, 40), clampPercent(100-used))
		if resetRaw, ok := result.BucketResets[window.bucket]; ok {
			if resetAt, valid := quota.ParseResetTime(resetRaw); valid {
				line += fmt.Sprintf(" (Reset in %s)", quota.FormatCountdown(time.Until(resetAt)))
			}
		}
		fmt.Fprintln(out, line)
	}
	return nil
}

func clampPercent(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

var quotaStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Summarize local session tokens and simulate model costs",
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonMode, _ := cmd.Flags().GetBool("json")
		days, _ := cmd.Flags().GetInt("days")
		if days < 1 {
			return fmt.Errorf("invalid --days %d: must be >= 1", days)
		}

		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot determine home directory: %w", err)
		}
		stats, err := quota.CollectSessionTokens(home, days, time.Now())
		if err != nil {
			return fmt.Errorf("collect session stats: %w", err)
		}

		if jsonMode {
			data, err := json.MarshalIndent(stats, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		}
		return renderQuotaStats(cmd, stats)
	},
}

func renderQuotaStats(cmd *cobra.Command, stats quota.SessionStats) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "[Local Session Stats - Last %d Days]\n", stats.Days)
	if stats.Sessions == 0 {
		fmt.Fprintf(out, "No local session logs found (~/.codex/sessions, ~/.claude/projects).\n")
		return nil
	}

	fmt.Fprintf(out, "- Cache Miss Input: %s tokens\n", quota.HumanizeTokens(stats.Tokens.Input))
	fmt.Fprintf(out, "- Cache Hit Input:  %s tokens\n", quota.HumanizeTokens(stats.Tokens.CacheRead))
	fmt.Fprintf(out, "- Output Generated: %s tokens\n", quota.HumanizeTokens(stats.Tokens.Output))
	fmt.Fprintf(out, "- Cache Hit Rate: %.1f%%\n\n", quota.CacheHitRate(stats.Tokens)*100)

	fmt.Fprintln(out, "[Estimated Cost Comparison]")
	estimates := quota.Simulate(stats.Tokens, quota.SimulatorModels)
	for _, estimate := range estimates {
		line := fmt.Sprintf("- %-16s $%.2f", estimate.Model, estimate.Cost)
		if estimate.Best {
			line += "  (" + estimate.Note + ")"
		}
		fmt.Fprintln(out, line)
	}
	return nil
}

func init() {
	quotaShowCmd.Flags().Bool("json", false, "output raw quota result as JSON")
	quotaShowCmd.Flags().String("endpoint", "", "override the OpenCode Go usage API endpoint")
	quotaStatsCmd.Flags().Int("days", 30, "window of local session logs to aggregate")
	quotaStatsCmd.Flags().Bool("json", false, "output session stats as JSON")
	quotaCmd.AddCommand(quotaShowCmd, quotaStatsCmd)
	rootCmd.AddCommand(quotaCmd)
}
