package cmd

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/suho-han/one-click-tools/internal/ui"
	"github.com/suho-han/one-click-tools/internal/update"
)

type toolItem struct {
	tool   update.Tool
	icon3  [3]string
	check  bool
	cursor bool
}

type configModel struct {
	items     []toolItem
	cancelled bool
	done      bool
}

func newConfigModel(enabledTools []string) configModel {
	items := make([]toolItem, 0, len(update.Tools))
	for i, t := range update.Tools {
		enabled := len(enabledTools) == 0
		if len(enabledTools) > 0 {
			for _, et := range enabledTools {
				if strings.EqualFold(et, t.BinaryName) {
					enabled = true
					break
				}
			}
		}
		// 3 lines = 12 dots high. For 1:1 aspect ratio, we need 12 dots wide = 6 Braille chars.
		lines := ui.InlineIconLines(t.LobeIcon, 6, 3)
		var icon3 [3]string
		if len(lines) >= 3 {
			icon3[0], icon3[1], icon3[2] = lines[0], lines[1], lines[2]
		} else {
			icon3[1] = "•"
		}
		items = append(items, toolItem{
			tool:   t,
			icon3:  icon3,
			check:  enabled,
			cursor: i == 0,
		})
	}
	return configModel{items: items}
}

func (m configModel) Init() tea.Cmd { return nil }

func (m configModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.cancelled = true
			m.done = true
			return m, tea.Quit
		case "up", "k":
			m.move(-1)
		case "down", "j":
			m.move(1)
		case "space", " ":
			i := m.index()
			if i >= 0 {
				m.items[i].check = !m.items[i].check
			}
		case "right":
			for i := range m.items {
				m.items[i].check = true
			}
		case "left":
			for i := range m.items {
				m.items[i].check = false
			}
		case "enter":
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *configModel) move(delta int) {
	i := m.index()
	if i < 0 {
		return
	}
	m.items[i].cursor = false
	n := i + delta
	if n < 0 {
		n = len(m.items) - 1
	}
	if n >= len(m.items) {
		n = 0
	}
	m.items[n].cursor = true
}

func (m configModel) index() int {
	for i := range m.items {
		if m.items[i].cursor {
			return i
		}
	}
	return -1
}

func (m configModel) View() string {
	var b strings.Builder
	b.WriteString("? Select tools to enable for agent-update:\n")
	for _, it := range m.items {
		mark := "[ ]"
		if it.check {
			mark = "[x]"
		}
		cursor := " "
		if it.cursor {
			cursor = ">"
		}

		// Each item line starts with "X[X] ", where X is cursor and [X] is mark.
		// That's 1 (cursor) + 3 (mark) + 1 (space) = 5 characters.
		// To align icon top/bottom with the center row, they should have 5 spaces.
		indent := "     "

		b.WriteString(fmt.Sprintf("%s%s\n", indent, it.icon3[0]))
		name := it.tool.Colorize(it.tool.Name)
		if it.cursor {
			name = it.tool.ColorizeWithBackgroundBlackText(it.tool.Name)
		}
		b.WriteString(fmt.Sprintf("%s%s %s %s\n", cursor, mark, it.icon3[1], name))
		b.WriteString(fmt.Sprintf("%s%s\n", indent, it.icon3[2]))
	}
	b.WriteString("\n[Use arrows to move, space to select, <right> to all, <left> to none]\n")
	b.WriteString("[Enter to Confirm, Ctrl+C to exit]\n")
	return b.String()
}

func runInteractiveConfig() ([]string, bool, error) {
	enabledTools := viper.GetStringSlice("enabled_tools")
	model := newConfigModel(enabledTools)
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, false, err
	}
	m, ok := finalModel.(configModel)
	if !ok {
		return nil, false, fmt.Errorf("unexpected model type")
	}
	if m.cancelled {
		return nil, true, nil
	}
	var selected []string
	for _, it := range m.items {
		if it.check {
			selected = append(selected, it.tool.BinaryName)
		}
	}
	return selected, false, nil
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration (interactive selection if no sub-command)",
	Run: func(cmd *cobra.Command, args []string) {
		newEnabledTools, cancelled, err := runInteractiveConfig()
		if err != nil {
			fmt.Printf("Prompt failed: %v\n", err)
			return
		}
		if cancelled {
			fmt.Println("Configuration cancelled.")
			return
		}
		viper.Set("enabled_tools", newEnabledTools)
		if err := viper.WriteConfig(); err != nil {
			fmt.Printf("Failed to write config: %v\n", err)
			return
		}
		fmt.Println("Config updated successfully.")
		if len(newEnabledTools) == 0 {
			fmt.Println("Summary: no tools selected.")
		} else {
			fmt.Printf("Summary: %d selected (%s)\n", len(newEnabledTools), strings.Join(newEnabledTools, ", "))
		}
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
				fmt.Printf("  ✓ %s\n", t.Colorize(t.Name))
			} else {
				fmt.Printf("  ✗ %s\n", t.Colorize(t.Name))
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
