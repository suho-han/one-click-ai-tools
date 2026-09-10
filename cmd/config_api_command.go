package cmd

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

var (
	configListJSON          bool
	configUpdateJSON        string
	configUpdatePayloadFlag string
)

var configUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update configuration from a machine-readable payload",
	Long: `Apply a JSON configuration payload (same key names as config list --json).

Prefer --payload <json>; --json <payload> is the deprecated legacy
spelling kept for older callers. Use --payload - to read the payload
from stdin.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if configUpdatePayloadFlag != "" && configUpdateJSON != "" {
			return errors.New("use one of --payload or --json, not both")
		}

		raw := configUpdatePayloadFlag
		if raw == "-" {
			data, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("failed to read payload from stdin: %w", err)
			}
			raw = string(data)
		} else if raw == "" {
			raw = configUpdateJSON
			if raw != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "[oct] config update --json <payload> is deprecated; use --payload <json|-> instead")
			}
		}

		payload, err := parseConfigUpdatePayload(raw)
		if err != nil {
			return err
		}
		if err := applyConfigUpdate(payload); err != nil {
			return err
		}
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Config updated.")
		return nil
	},
}

func init() {
	configListCmd.Flags().BoolVar(&configListJSON, "json", false, "print configuration as JSON")

	configUpdateCmd.Flags().StringVar(&configUpdatePayloadFlag, "payload", "", "configuration update JSON payload ('-' reads stdin)")
	configUpdateCmd.Flags().StringVar(&configUpdateJSON, "json", "", "deprecated: use --payload")
	configCmd.AddCommand(configUpdateCmd)
}
