package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/suho-han/one-click-ai-tools/internal/usage"
)

var usageResetCreditFlags struct {
	provider string
	yes      bool
	json     bool
}

var usageResetCreditCmd = &cobra.Command{
	Use:   "reset-credit",
	Short: "🎁 Spend a Codex rate-limit reset credit (clears usage windows)",
	Long: `Spend one of Codex's banked rate-limit reset credits for the given
provider row. Spending a credit clears the account's rate-limit windows
immediately, so a fresh 5h/7d allowance is available without waiting for
the scheduled reset.

Run with --json (and without --yes) to only list the available credits —
a safe query that spends nothing. The Swift menubar uses that query to
render the (🎁 N) badge and passes --yes after its own confirmation dialog.`,
	SilenceUsage: true,
	RunE:         runUsageResetCredit,
}

func init() {
	usageCmd.AddCommand(usageResetCreditCmd)
	usageResetCreditCmd.Flags().StringVar(&usageResetCreditFlags.provider, "provider", "codex", "provider row to spend on: codex or codex:<account>")
	usageResetCreditCmd.Flags().BoolVar(&usageResetCreditFlags.yes, "yes", false, "spend a credit without prompting")
	usageResetCreditCmd.Flags().BoolVar(&usageResetCreditFlags.json, "json", false, "output in JSON format")
}

type resetCreditOutput struct {
	Provider             string                   `json:"provider"`
	AvailableCountBefore int                      `json:"available_count_before"`
	Credits              []usage.CodexResetCredit `json:"credits"`
	Consumed             bool                     `json:"consumed"`
	Code                 string                   `json:"code,omitempty"`
	WindowsReset         int                      `json:"windows_reset,omitempty"`
	RedeemRequestID      string                   `json:"redeem_request_id,omitempty"`
	AvailableCountAfter  int                      `json:"available_count_after,omitempty"`
	UsageAfter           map[string]string        `json:"usage_after,omitempty"`
	Error                string                   `json:"error,omitempty"`
}

func runUsageResetCredit(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	label := strings.TrimSpace(usageResetCreditFlags.provider)

	home, err := resolveCodexAccountHome(label)
	if err != nil {
		return err
	}

	credits, err := usage.FetchCodexResetCredits(ctx, home)
	if err != nil {
		return resetCreditFailure(label, err)
	}

	// Safe query mode: --json without --yes lists what is spendable and
	// never touches the consume endpoint.
	if usageResetCreditFlags.json && !usageResetCreditFlags.yes {
		return printResetCreditJSON(resetCreditOutput{
			Provider:             label,
			AvailableCountBefore: credits.AvailableCount,
			Credits:              credits.Credits,
		})
	}

	if !usageResetCreditFlags.yes {
		if !stdinIsTerminal() {
			return fmt.Errorf("refusing to spend a reset credit non-interactively; pass --yes")
		}
		listResetCredits(label, credits)
		confirmed, err := promptYesNo(fmt.Sprintf("Spend 1 of %d reset credit(s) on %s now?", credits.AvailableCount, label), false)
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("Cancelled; no credit spent.")
			return nil
		}
	}

	result, err := usage.ConsumeCodexResetCredit(ctx, home, "")
	if err != nil {
		return resetCreditFailure(label, err)
	}

	// Report what the spend actually left behind: the remaining credit
	// count and the account's fresh usage, so "did it go back to full?"
	// is answerable from one command.
	after, afterErr := usage.FetchCodexResetCredits(ctx, home)
	availableAfter := 0
	if afterErr == nil {
		availableAfter = after.AvailableCount
	}
	usageAfter := usage.FetchCodexUsageForHome(ctx, home, label)

	if usageResetCreditFlags.json {
		out := resetCreditOutput{
			Provider:             label,
			AvailableCountBefore: credits.AvailableCount,
			Credits:              credits.Credits,
			Consumed:             true,
			Code:                 result.Code,
			WindowsReset:         result.WindowsReset,
			RedeemRequestID:      result.RedeemRequestID,
			AvailableCountAfter:  availableAfter,
			UsageAfter:           usageAfter.Buckets,
		}
		if afterErr != nil {
			out.Error = afterErr.Error()
		}
		return printResetCreditJSON(out)
	}

	fmt.Printf("✓ Reset applied (%s, windows_reset=%d). Remaining credits: %d.\n", result.Code, result.WindowsReset, availableAfter)
	five, _ := usage.BucketValue(usageAfter, "5h")
	seven, _ := usage.BucketValue(usageAfter, "7d")
	month, _ := usage.BucketValue(usageAfter, "1m")
	fmt.Printf("Usage after reset: 5h %s used · 7d %s used · 1m %s used\n", five, seven, month)
	if afterErr != nil {
		fmt.Printf("(could not refresh credit count: %v)\n", afterErr)
	}
	return nil
}

// resolveCodexAccountHome maps a --provider row label to its credential
// home: plain "codex" (and codex:<alias> rows) only — no other provider
// reports reset credits today.
func resolveCodexAccountHome(label string) (string, error) {
	provider, alias, hasAlias := usage.SplitAccountProviderLabel(label)
	if !hasAlias {
		if strings.EqualFold(label, "codex") {
			return "", nil
		}
		return "", fmt.Errorf("rate-limit reset credits are a Codex feature; pass codex or codex:<account>, got %q", label)
	}
	if !strings.EqualFold(provider, "codex") {
		return "", fmt.Errorf("rate-limit reset credits are a Codex feature; pass codex or codex:<account>, got %q", label)
	}
	for _, account := range usage.AccountsForProvider("codex") {
		if strings.EqualFold(account.Name, alias) {
			return account.Home, nil
		}
	}
	return "", fmt.Errorf("no codex account named %q; check 'oct config account list'", alias)
}

func listResetCredits(label string, credits usage.CodexResetCredits) {
	fmt.Printf("Codex rate-limit reset credits — %s\n", label)
	for _, credit := range credits.Credits {
		fmt.Printf("  • %s — expires %s (%s)\n", strings.TrimSpace(credit.Title), credit.ExpiresAt, credit.Status)
	}
	fmt.Printf("Available: %d\n", credits.AvailableCount)
}

func printResetCreditJSON(out resetCreditOutput) error {
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// resetCreditFailure keeps the failure message short: HTTP error bodies from
// the wham endpoint already carry the actionable part ("no available credit",
// auth expired, ...).
func resetCreditFailure(label string, err error) error {
	return fmt.Errorf("reset credit on %s: %w", label, err)
}

func stdinIsTerminal() bool {
	info, err := os.Stdin.Stat()
	return err == nil && (info.Mode()&os.ModeCharDevice) != 0
}
