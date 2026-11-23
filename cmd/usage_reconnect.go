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
			fmt.Fprintln(cmd.OutOrStdout(), "Opening Antigravity sign-in. Follow the login screen, then enter /exit to allow the next usage refresh.")
			return usage.ReconnectAntigravity(cmd.Context())
		},
	})
}
