package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/suho-han/one-click-tools/internal/sessionrefresh"
	"github.com/suho-han/one-click-tools/internal/update"
)

var sessionRefreshCmd = &cobra.Command{
	Use:     "session-refresh",
	GroupID: "maintenance",
	Short:   "Probe tool sessions without sending prompts",
	Long: `Probe configured AI tool sessions without intentionally sending prompts.

This command does not fetch usage/quota and does not intentionally send prompts.
It only uses token-free probes such as auth-status checks and local session/auth artifact inspection.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		providers, _ := cmd.Flags().GetStringSlice("provider")
		jsonMode, _ := cmd.Flags().GetBool("json")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		strict, _ := cmd.Flags().GetBool("strict")

		results := sessionrefresh.Refresh(sessionrefresh.RefreshOptions{
			Providers: selectedRefreshProviders(providers),
			DryRun:    dryRun,
		})

		if jsonMode {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			if err := enc.Encode(results); err != nil {
				return err
			}
		} else {
			printSessionRefreshResults(cmd.OutOrStdout(), results)
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

func printSessionRefreshResults(w io.Writer, results []sessionrefresh.RefreshResult) {
	fmt.Fprintf(w, "%-14s %-12s %-16s %s\n", "provider", "status", "mode", "message")
	for _, result := range results {
		message := result.Message
		if result.SourcePath != "" {
			message += " [" + result.SourcePath + "]"
		}
		fmt.Fprintf(w, "%-14s %-12s %-16s %s\n", result.Provider, result.Status, result.Mode, message)
	}
}

func init() {
	rootCmd.AddCommand(sessionRefreshCmd)
	sessionRefreshCmd.Flags().StringSlice("provider", nil, "Provider(s) to probe; repeat or use comma-separated values")
	sessionRefreshCmd.Flags().Bool("json", false, "Output in JSON format")
	sessionRefreshCmd.Flags().Bool("dry-run", false, "Show what would be probed without running checks")
	sessionRefreshCmd.Flags().Bool("strict", false, "Return non-zero if any provider is unsupported or errors")
}
