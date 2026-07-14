package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/suho-han/one-click-ai-tools/internal/sessionrefresh"
	"github.com/suho-han/one-click-ai-tools/internal/update"
	"github.com/suho-han/one-click-ai-tools/internal/usage"
)

var (
	sessionRefreshRun      = sessionrefresh.Refresh
	sessionRefreshGetUsage = usage.GetUsage
)

type sessionRefreshOutput struct {
	RefreshResults []sessionrefresh.RefreshResult `json:"refresh_results"`
	Usage          []usage.UsageResult            `json:"usage,omitempty"`
	UsageDiff      *usageDiffSummary              `json:"usage_diff,omitempty"`
}

type usageDiffSummary struct {
	Changed   []string `json:"changed,omitempty"`
	Added     []string `json:"added,omitempty"`
	Removed   []string `json:"removed,omitempty"`
	Unchanged int      `json:"unchanged"`
}

var sessionRefreshCmd = &cobra.Command{
	Use:     "session-refresh",
	GroupID: "maintenance",
	Short:   "Probe tool sessions without sending prompts",
	Long: `Probe configured AI tool sessions without intentionally sending prompts.

This command does not intentionally send prompts. It uses token-free probes such as auth-status checks and local session/auth artifact inspection,
then re-runs local usage collection so the report reflects the latest detectable session state.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		providers, _ := cmd.Flags().GetStringSlice("provider")
		jsonMode, _ := cmd.Flags().GetBool("json")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		strict, _ := cmd.Flags().GetBool("strict")

		var beforeUsage []usage.UsageResult
		if !dryRun {
			usageResults, err := sessionRefreshGetUsage()
			if err != nil {
				return err
			}
			beforeUsage = usageResults
		}

		results := sessionRefreshRun(sessionrefresh.RefreshOptions{
			Providers: selectedRefreshProviders(providers),
			DryRun:    dryRun,
		})

		output := sessionRefreshOutput{RefreshResults: results}
		if !dryRun {
			usageResults, err := sessionRefreshGetUsage()
			if err != nil {
				return err
			}
			output.Usage = usageResults
			diff := buildUsageDiffSummary(beforeUsage, usageResults)
			output.UsageDiff = &diff
		}

		if jsonMode {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			if err := enc.Encode(output); err != nil {
				return err
			}
		} else {
			printSessionRefreshResults(cmd.OutOrStdout(), output.RefreshResults)
			if len(output.Usage) > 0 {
				fmt.Fprintln(cmd.OutOrStdout())
				fmt.Fprintln(cmd.OutOrStdout(), "refreshed usage")
				usage.RenderTable(cmd.OutOrStdout(), output.Usage)
				if output.UsageDiff != nil {
					fmt.Fprintln(cmd.OutOrStdout())
					printUsageDiffSummary(cmd.OutOrStdout(), *output.UsageDiff)
				}
			}
		}

		if strict {
			for _, result := range results {
				if result.Status == "unsupported" || result.Status == "error" {
					return fmt.Errorf("strict mode failed on provider %s (%s)", result.Provider, result.Status)
				}
			}
		}
		return nil
	},
}

func selectedRefreshProviders(flagProviders []string) []string {
	if len(flagProviders) > 0 {
		return splitCommaProviders(flagProviders)
	}
	enabled := viper.GetStringSlice("enabled_tools")
	order := viper.GetStringSlice("agent_order")
	tools := update.GetFilteredTools(enabled, update.GetOrderedTools(order))
	if len(tools) == 0 {
		tools = update.GetOrderedTools(order)
	}
	providers := make([]string, 0, len(tools))
	for _, tool := range tools {
		providers = append(providers, tool.BinaryName)
	}
	return providers
}

func splitCommaProviders(values []string) []string {
	providers := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				providers = append(providers, part)
			}
		}
	}
	return providers
}

func buildUsageDiffSummary(before, after []usage.UsageResult) usageDiffSummary {
	beforeMap := make(map[string]usage.UsageResult, len(before))
	afterMap := make(map[string]usage.UsageResult, len(after))
	for _, item := range before {
		beforeMap[item.Provider] = item
	}
	for _, item := range after {
		afterMap[item.Provider] = item
	}

	summary := usageDiffSummary{}
	for provider, current := range afterMap {
		prev, ok := beforeMap[provider]
		if !ok {
			summary.Added = append(summary.Added, provider)
			continue
		}
		if sameUsageResult(prev, current) {
			summary.Unchanged++
			continue
		}
		summary.Changed = append(summary.Changed, provider)
	}
	for provider := range beforeMap {
		if _, ok := afterMap[provider]; !ok {
			summary.Removed = append(summary.Removed, provider)
		}
	}
	sort.Strings(summary.Changed)
	sort.Strings(summary.Added)
	sort.Strings(summary.Removed)
	return summary
}

func sameUsageResult(a, b usage.UsageResult) bool {
	return a.Provider == b.Provider &&
		a.Status == b.Status &&
		a.Used == b.Used &&
		a.Limit == b.Limit &&
		a.Unit == b.Unit &&
		a.Message == b.Message &&
		a.Period == b.Period &&
		a.Source == b.Source
}

func printSessionRefreshResults(w io.Writer, results []sessionrefresh.RefreshResult) {
	fmt.Fprintf(w, "%-14s %-12s %-12s %-16s %s\n", "provider", "status", "confidence", "mode", "message")
	for _, result := range results {
		message := result.Message
		if result.SourcePath != "" {
			message += " [" + result.SourcePath + "]"
		}
		confidence := result.Confidence
		if confidence == "" {
			confidence = "-"
		}
		fmt.Fprintf(w, "%-14s %-12s %-12s %-16s %s\n", result.Provider, result.Status, confidence, result.Mode, message)
	}
}

func printUsageDiffSummary(w io.Writer, diff usageDiffSummary) {
	fmt.Fprintf(w, "usage diff: changed=%d unchanged=%d added=%d removed=%d\n", len(diff.Changed), diff.Unchanged, len(diff.Added), len(diff.Removed))
	if len(diff.Changed) > 0 {
		fmt.Fprintf(w, "  changed: %s\n", strings.Join(diff.Changed, ", "))
	}
	if len(diff.Added) > 0 {
		fmt.Fprintf(w, "  added: %s\n", strings.Join(diff.Added, ", "))
	}
	if len(diff.Removed) > 0 {
		fmt.Fprintf(w, "  removed: %s\n", strings.Join(diff.Removed, ", "))
	}
}

func init() {
	rootCmd.AddCommand(sessionRefreshCmd)
	sessionRefreshCmd.Flags().StringSlice("provider", nil, "Provider(s) to probe; repeat or use comma-separated values")
	sessionRefreshCmd.Flags().Bool("json", false, "Output in JSON format")
	sessionRefreshCmd.Flags().Bool("dry-run", false, "Show what would be probed without running checks")
	sessionRefreshCmd.Flags().Bool("strict", false, "Return non-zero if any provider is unsupported or errors")
}
