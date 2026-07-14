package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	configListJSON   bool
	configUpdateJSON string
)

var configUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update configuration from a machine-readable payload",
	RunE: func(cmd *cobra.Command, args []string) error {
		payload, err := parseConfigUpdatePayload(configUpdateJSON)
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
	oldRun := configListCmd.Run
	configListCmd.Flags().BoolVar(&configListJSON, "json", false, "print configuration as JSON")
	configListCmd.Run = func(cmd *cobra.Command, args []string) {
		if !configListJSON {
			oldRun(cmd, args)
			return
		}
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(buildConfigSnapshot(configPathForDisplay())); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "failed to encode config json: %v\n", err)
		}
	}

	configUpdateCmd.Flags().StringVar(&configUpdateJSON, "json", "", "configuration update JSON payload")
	configCmd.AddCommand(configUpdateCmd)
}
