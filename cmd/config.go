package cmd

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/suho-han/one-click-tools/internal/update"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration (interactive selection if no sub-command)",
	Run: func(cmd *cobra.Command, args []string) {
		// If no args, enter interactive mode
		var options []string
		var defaults []string
		enabledTools := viper.GetStringSlice("enabled_tools")

		for _, t := range update.Tools {
			options = append(options, t.Name)
			enabled := false
			if len(enabledTools) == 0 {
				enabled = true
			} else {
				for _, et := range enabledTools {
					if strings.EqualFold(et, t.BinaryName) {
						enabled = true
						break
					}
				}
			}
			if enabled {
				defaults = append(defaults, t.Name)
			}
		}

		prompt := &survey.MultiSelect{
			Message:  "Select tools to enable for agent-update:",
			Options:  options,
			Default:  defaults,
			PageSize: 10,
		}

		var selected []string
		err := survey.AskOne(prompt, &selected)
		if err != nil {
			if err == terminal.InterruptErr {
				fmt.Println("\nConfiguration cancelled.")
				return
			}
			fmt.Printf("Prompt failed: %v\n", err)
			return
		}

		var newEnabledTools []string
		for _, s := range selected {
			for _, t := range update.Tools {
				if t.Name == s {
					newEnabledTools = append(newEnabledTools, t.BinaryName)
					break
				}
			}
		}

		// If all tools selected, we can just empty the list to mean "all" (default behavior)
		// Or we can keep them explicitly. Let's keep them explicit if the user manually selected.
		viper.Set("enabled_tools", newEnabledTools)
		if err := viper.WriteConfig(); err != nil {
			fmt.Printf("Failed to write config: %v\n", err)
			return
		}
		fmt.Println("Config updated successfully.")
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set configuration value",
}

var configSetToolsCmd = &cobra.Command{
	Use:   "tools <tool1,tool2,...>",
	Short: "Set enabled tools",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		tools := strings.Split(args[0], ",")
		var validTools []string
		for _, tool := range tools {
			tool = strings.TrimSpace(tool)
			found := false
			for _, t := range update.Tools {
				if strings.EqualFold(tool, t.BinaryName) {
					validTools = append(validTools, t.BinaryName)
					found = true
					break
				}
			}
			if !found {
				fmt.Printf("Unknown tool: %s\n", tool)
				return
			}
		}
		
		viper.Set("enabled_tools", validTools)
		if err := viper.WriteConfig(); err != nil {
			fmt.Printf("Failed to write config: %v\n", err)
			return
		}
		fmt.Println("Config updated.")
	},
}

var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset configuration to defaults",
	Run: func(cmd *cobra.Command, args []string) {
		viper.Set("enabled_tools", []string{})
		if err := viper.WriteConfig(); err != nil {
			fmt.Printf("Failed to write config: %v\n", err)
			return
		}
		fmt.Println("Config reset to defaults.")
	},
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show current configuration",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("=== one-click-tools config ===")
		fmt.Printf("Config file: %s\n\n", viper.ConfigFileUsed())

		enabledTools := viper.GetStringSlice("enabled_tools")
		fmt.Println("Enabled tools (agent-update):")

		for _, t := range update.Tools {
			enabled := false
			if len(enabledTools) == 0 {
				enabled = true
			} else {
				for _, et := range enabledTools {
					if strings.EqualFold(et, t.BinaryName) {
						enabled = true
						break
					}
				}
			}

			if enabled {
				fmt.Printf("  ✓ %s\n", t.Name)
			} else {
				fmt.Printf("  ✗ %s\n", t.Name)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configResetCmd)
	configSetCmd.AddCommand(configSetToolsCmd)
}
