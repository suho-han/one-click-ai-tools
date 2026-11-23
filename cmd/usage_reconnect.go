package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/suho-han/one-click-ai-tools/internal/usage"
	"golang.org/x/term"
)

func init() {
	usageCmd.AddCommand(&cobra.Command{
		Use:       "reconnect antigravity",
		Short:     "Re-authenticate Antigravity in its interactive CLI",
		ValidArgs: []string{"antigravity"},
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				return fmt.Errorf("Antigravity reconnection requires an interactive terminal")
			}
			fmt.Fprintln(cmd.OutOrStdout(), "At the agy prompt, enter /logout and follow the sign-in screen. If agy already asks you to sign in, sign in directly.")
			fmt.Fprintln(cmd.OutOrStdout(), "After signing in, enter /exit, then refresh usage. If logout exits agy, run this command again to sign in.")
			return usage.ReconnectAntigravity(cmd.Context())
		},
	})
}
