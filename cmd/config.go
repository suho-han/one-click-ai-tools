package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/suho-han/one-click-ai-tools/internal/ui"
	"github.com/suho-han/one-click-ai-tools/internal/update"
	"github.com/suho-han/one-click-ai-tools/internal/usage"
)

type toolItem struct {
	tool   update.Tool
	icon3  [3]string
	check  bool
	cursor bool
	// choose all/none control row (not a real tool)
	isToggleControl bool
	// confirm control row (not a real tool)
	isConfirmControl bool
}

type configModel struct {
	items     []toolItem
	cancelled bool
	done      bool
	// Terminal-fit windowing: bubbletea paints the whole View() into the
	// alt screen, and the alt screen has no scrollback -- an oversized view
	// loses its top. The model therefore renders a sliding window of items
	// sized from tea.WindowSizeMsg. viewHeight == 0 (no size known yet, e.g.
	// unit tests) renders everything.
	viewHeight  int
	offset      int // first visible item index
	visibleRows int // visible item count at the current rowHeight
	rowHeight   int // 3 (icon + name + icon) or 1 (name only, tiny terminals)
}

// viewChrome is the number of lines around the item list: 1 header + 1 blank
// + 4 help lines. Scroll indicators are carved out of the item budget below.
const viewChrome = 6

// layoutForHeight resolves (rowHeight, visibleRows) for a terminal height.
func layoutForHeight(height int) (int, int) {
	itemBudget := height - viewChrome - 2 // reserve up to 2 scroll-indicator rows
	if itemBudget < 5 {
		// Too small for a 3-line item: degrade to one line per item.
		visible := itemBudget
		if visible < 1 {
			visible = 1
		}
		return 1, visible
	}
	visible := itemBudget / 3
	if visible < 1 {
		visible = 1
	}
	return 3, visible
}

// applyWindowSize records the terminal height and keeps the cursor inside
// the visible window.
func (m *configModel) applyWindowSize(height int) {
	if height <= 0 {
		return
	}
	m.viewHeight = height
	m.rowHeight, m.visibleRows = layoutForHeight(height)
	m.clampOffset()
}

// clampOffset shifts offset so the cursor row stays within the window.
func (m *configModel) clampOffset() {
	if m.visibleRows <= 0 || m.viewHeight == 0 {
		return
	}
	if last := len(m.items) - 1; last < 0 {
		m.offset = 0
		return
	}
	i := m.index()
	if i < m.offset {
		m.offset = i
	}
	if i >= m.offset+m.visibleRows {
		m.offset = i - m.visibleRows + 1
	}
	if max := len(m.items) - m.visibleRows; m.offset > max {
		m.offset = max
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

// visibleItems returns the item index range to render.
func (m configModel) visibleItems() (from, to int) {
	if m.viewHeight == 0 {
		return 0, len(m.items)
	}
	to = m.offset + m.visibleRows
	if to > len(m.items) {
		to = len(m.items)
	}
	return m.offset, to
}

func newConfigModel(enabledTools []string, agentOrder []string) configModel {
	orderedTools := update.GetOrderedTools(agentOrder)
	items := make([]toolItem, 0, len(orderedTools))
	for i, t := range orderedTools {
		enabled := len(enabledTools) == 0
		if len(enabledTools) > 0 {
			for _, et := range enabledTools {
				if update.NormalizeToolName(et) == update.NormalizeToolName(t.BinaryName) {
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

	items = append(items, toolItem{
		tool: update.Tool{
			Name:       "Choose all / Choose none",
			BinaryName: "__toggle_all_none__",
			Icon:       "⇄",
			HexColor:   "#9CA3AF",
		},
		icon3:           [3]string{"", "⇄", ""},
		check:           false,
		cursor:          false,
		isToggleControl: true,
	})
	items = append(items, toolItem{
		tool: update.Tool{
			Name:       "Confirm",
			BinaryName: "__confirm__",
			Icon:       "✓",
			HexColor:   "#10B981",
		},
		icon3:            [3]string{"", "✓", ""},
		check:            false,
		cursor:           false,
		isConfirmControl: true,
	})
	return configModel{items: items}
}

func (m configModel) Init() tea.Cmd { return nil }

func (m configModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.applyWindowSize(msg.Height)
	case tea.KeyMsg:
		if msg.Type == tea.KeyEnter {
			i := m.index()
			if i >= 0 {
				if m.items[i].isConfirmControl {
					m.done = true
					return m, tea.Quit
				}
				if m.items[i].isToggleControl {
					allChecked := true
					for j := range m.items {
						if m.items[j].isToggleControl || m.items[j].isConfirmControl {
							continue
						}
						if !m.items[j].check {
							allChecked = false
							break
						}
					}
					for j := range m.items {
						if m.items[j].isToggleControl || m.items[j].isConfirmControl {
							continue
						}
						m.items[j].check = !allChecked
					}
				} else {
					m.items[i].check = !m.items[i].check
				}
			}
			return m, nil
		}
		switch msg.String() {
		case "ctrl+c", "ctrl+q", "q":
			m.cancelled = true
			m.done = true
			return m, tea.Quit
		case "up":
			m.move(-1)
		case "down":
			m.move(1)

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
	m.clampOffset()
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
	from, to := m.visibleItems()

	// Assemble the view as ranked lines: when the terminal is too small for
	// the full layout, chrome drops in rank order (help lines, blank, scroll
	// indicators, header) before any item row is sacrificed. The alt screen
	// has no scrollback, so an oversized view would clip its top.
	type viewLine struct {
		text string
		rank int // higher drops first; 0 never drops
	}
	lines := []viewLine{{text: "? Select tools to enable for agent-update:", rank: 1}}
	if from > 0 {
		lines = append(lines, viewLine{text: fmt.Sprintf("  ↑ %d more", from), rank: 2})
	}
	for _, it := range m.items[from:to] {
		for _, text := range m.renderItemLines(it) {
			lines = append(lines, viewLine{text: text})
		}
	}
	if more := len(m.items) - to; more > 0 {
		lines = append(lines, viewLine{text: fmt.Sprintf("  ↓ %d more", more), rank: 2})
	}
	lines = append(lines,
		viewLine{text: "", rank: 3},
		viewLine{text: "[Use ↑/↓ to move]", rank: 4},
		viewLine{text: "[Use Enter to toggle current item]", rank: 4},
		viewLine{text: "[Choose all/none row toggles all tools]", rank: 4},
		viewLine{text: "[Move to last 'Confirm' row and press Enter to save, Ctrl+C/Ctrl+Q to exit]", rank: 4},
	)

	for m.viewHeight > 0 && len(lines) > m.viewHeight {
		drop := -1
		best := 0
		for i, l := range lines {
			if l.rank > best {
				best = l.rank
				drop = i
			}
		}
		if drop == -1 {
			break
		}
		lines = append(lines[:drop], lines[drop+1:]...)
	}

	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l.text)
		b.WriteString("\n")
	}
	return b.String()
}

// renderItemLines renders one item as 1 (compact) or 3 (icon) view lines.
func (m configModel) renderItemLines(it toolItem) []string {
	mark := "[ ]"
	if it.check {
		mark = "[x]"
	}
	cursor := " "
	if it.cursor {
		cursor = ">"
	}

	nameText := it.tool.Name
	if it.isToggleControl {
		allChecked := true
		for _, x := range m.items {
			if x.isToggleControl || x.isConfirmControl {
				continue
			}
			if !x.check {
				allChecked = false
				break
			}
		}
		if allChecked {
			nameText = "Choose none"
		} else {
			nameText = "Choose all"
		}
	}

	name := it.tool.Colorize(nameText)
	if it.cursor {
		name = it.tool.ColorizeWithBackgroundBlackText(nameText)
	}

	// 3 lines = 12 dots high; compact mode (very short terminals) drops the
	// Braille icon and renders the name row only.
	if m.rowHeight == 3 {
		indent := "     "
		return []string{
			indent + it.icon3[0],
			fmt.Sprintf("%s%s %s %s", cursor, mark, it.icon3[1], name),
			indent + it.icon3[2],
		}
	}
	return []string{fmt.Sprintf("%s%s %s %s", cursor, mark, it.icon3[1], name)}
}

func writeConfig() error {
	configPath := viper.ConfigFileUsed()
	if configPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		configPath = filepath.Join(home, ".oct", "config.yaml")
	}

	err := os.MkdirAll(filepath.Dir(configPath), 0o700)
	if err != nil {
		return err
	}

	if err := viper.WriteConfigAs(configPath); err != nil {
		return err
	}
	return os.Chmod(configPath, 0o600)
}

func runInteractiveConfig() ([]string, []string, string, bool, error) {
	enabledTools := viper.GetStringSlice("enabled_tools")
	agentOrder := viper.GetStringSlice("agent_order")
	model := newConfigModel(enabledTools, agentOrder)
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, nil, "", false, err
	}
	m, ok := finalModel.(configModel)
	if !ok {
		return nil, nil, "", false, fmt.Errorf("unexpected model type")
	}
	if m.cancelled {
		return nil, nil, "", true, nil
	}
	var selected []string
	var order []string
	for _, it := range m.items {
		if it.isToggleControl || it.isConfirmControl {
			continue
		}
		if it.check {
			selected = append(selected, it.tool.BinaryName)
		}
		order = append(order, it.tool.BinaryName)
	}
	mode := usage.NormalizeDisplayMode(viper.GetString("usage_display_mode"))
	mode, err = promptUsageMode(mode)
	if err != nil {
		return nil, nil, "", false, err
	}
	return selected, order, mode, false, nil
}

// configPromptReader is shared across the multi-prompt config flow. A fresh
// bufio.Reader per prompt would buffer (and silently swallow) input meant for
// the following prompts in the same run.
var configPromptReader *bufio.Reader

func promptReader() *bufio.Reader {
	if configPromptReader == nil {
		configPromptReader = bufio.NewReader(os.Stdin)
	}
	return configPromptReader
}

// readPromptLine returns the next line of input. io.EOF counts as an empty
// line (non-interactive runs); any other read error is surfaced.
func readPromptLine() (string, error) {
	text, err := promptReader().ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("reading input: %w", err)
	}
	return strings.TrimSpace(text), nil
}

func promptToken(prompt string) (string, error) {
	fmt.Print(prompt)
	return readPromptLine()
}

func promptYesNo(prompt string, defaultYes bool) (bool, error) {
	defaultLabel := "n"
	if defaultYes {
		defaultLabel = "y"
	}
	fmt.Printf("%s [default: %s]: ", prompt, defaultLabel)
	text, err := readPromptLine()
	if err != nil {
		return false, err
	}
	text = strings.ToLower(text)
	if text == "" {
		return defaultYes, nil
	}
	return text == "y" || text == "yes", nil
}

func promptUsageMode(defaultMode string) (string, error) {
	// Keep interactive default deterministic for consistency.
	defaultMode = "remaining"
	fmt.Print("Usage display mode: remaining(r) / used(u) [default: r]: ")
	text, err := readPromptLine()
	if err != nil {
		return "", err
	}
	if text == "" {
		return defaultMode, nil
	}
	if text == "r" || text == "remaining" {
		return "remaining", nil
	}
	if text == "u" || text == "used" {
		return "used", nil
	}

	// Invalid input falls back to default choice.
	return defaultMode, nil
}

func setupTokens(tools []string) error {
	fmt.Println("\n--- Provider Setup ---")
	var needsClaudeAuth, needsGeminiAuth bool

	for _, tool := range tools {
		switch update.NormalizeToolName(tool) {
		case "claude":
			needsClaudeAuth = true
			fmt.Println("✓ Claude Code: OAuth API with 'claude --print /usage' fallback")
		case "commandcode":
			fmt.Println("✓ Command Code: Remote billing API (run 'commandcode login' or set COMMAND_CODE_API_KEY)")
		case "agy":
			needsGeminiAuth = true
			fmt.Println("✓ Antigravity CLI: Quota parsed from 'agy --print /usage'")
		case "opencode":
			fmt.Println("✓ OpenCode: Local session logs (~/.opencode/sessions or ~/.config/opencode/sessions)")
		case "codex":
			fmt.Println("✓ OpenAI Codex: Local session logs (~/.codex/sessions)")
		case "cursor-agent":
			fmt.Println("✓ Cursor CLI: Official CLI install, with local workspace storage fallback for usage")
		case "copilot":
			isUpdate := false
			existingToken := viper.GetString("github_api_token")
			if existingToken != "" {
				confirmed, err := promptYesNo("GitHub API Token is already registered. Do you want to update it?", false)
				if err != nil {
					return err
				}
				if !confirmed {
					fmt.Println("✓ GitHub Copilot: Using existing token")
					continue
				}
				isUpdate = true
			}

			promptStr := "Enter GitHub API Token\n[Doc] : https://github.com/settings/tokens\n> "
			if isUpdate {
				promptStr = "Enter new GitHub API Token (leave empty to skip)\n> "
			}
			token, err := promptToken(promptStr)
			if err != nil {
				return err
			}
			if token != "" {
				viper.Set("github_api_token", token)
				user, err := promptToken("Enter GitHub Username: ")
				if err != nil {
					return err
				}
				if user != "" {
					viper.Set("github_user", user)
				}
				if err := writeConfig(); err != nil {
					return fmt.Errorf("saving config: %w", err)
				}
				fmt.Println("✓ GitHub Copilot: Token saved")
			} else {
				if existingToken != "" {
					fmt.Println("✓ GitHub Copilot: Kept existing token")
				} else {
					fmt.Println("⚠ GitHub Copilot: No token provided (usage reporting may fail)")
				}
			}
		}
	}

	if needsClaudeAuth || needsGeminiAuth {
		fmt.Println("\n--- Authentication Reminders ---")
		if needsClaudeAuth {
			fmt.Println("Claude Code: Run 'claude auth login' to authenticate the Claude CLI.")
		}
		if needsGeminiAuth {
			fmt.Println("Antigravity CLI: Run 'agy' once to authenticate the CLI; oct parses 'agy --print /usage'.")
		}
	}
	return nil
}

func toolDisplayName(binaryName string) string {
	for _, t := range update.Tools {
		if t.MatchesName(binaryName) {
			return t.Colorize(t.Name)
		}
	}
	return binaryName
}

func printConfigSummary(enabledTools []string, usageMode string) {
	const innerWidth = 55
	fmt.Println()
	printSummaryBorder(innerWidth)
	if len(enabledTools) == 0 {
		printSummaryContent("providers: (none selected)")
	} else {
		colored := make([]string, 0, len(enabledTools))
		for _, tool := range enabledTools {
			colored = append(colored, toolDisplayName(tool))
		}
		printSummaryContent("providers: " + strings.Join(colored, ", "))
	}
	printSummaryContent("usage mode: " + usageMode)
	printSummaryBorder(innerWidth)
}

func printSummaryBorder(innerWidth int) {
	fmt.Printf("--||%s||--\n", strings.Repeat("=", innerWidth+2))
}

// appendStandaloneEntries appends the standalone usage-provider names found in
// raw (comma-separated entries allowed) to dst, skipping names already present.
func appendStandaloneEntries(dst, raw []string) []string {
	for _, entry := range raw {
		for _, part := range strings.Split(entry, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			p, ok := usage.LookupStandalone(part)
			if !ok {
				continue
			}
			dup := false
			for _, existing := range dst {
				if strings.EqualFold(existing, p.Name) {
					dup = true
					break
				}
			}
			if !dup {
				dst = append(dst, p.Name)
			}
		}
	}
	return dst
}

// mergeStandaloneIntoOrder keeps standalone usage providers at their original
// positions in agent_order across an interactive save: the picker only lists
// installable tools, so a plain overwrite would push standalone rows to the
// end and change usage/monitor/menubar ordering. Tool entries the user
// deselected are dropped; newly selected tools are appended after the
// carried-over order.
func mergeStandaloneIntoOrder(newOrder, oldOrder []string) []string {
	if len(oldOrder) == 0 {
		return newOrder
	}
	selected := make(map[string]bool, len(newOrder))
	for _, name := range newOrder {
		selected[update.NormalizeToolName(name)] = true
	}
	merged := make([]string, 0, len(newOrder)+len(oldOrder))
	seen := make(map[string]bool, len(newOrder)+len(oldOrder))
	appendName := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			merged = append(merged, name)
		}
	}
	for _, entry := range oldOrder {
		for _, part := range strings.Split(entry, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if p, ok := usage.LookupStandalone(part); ok {
				appendName(p.Name)
				continue
			}
			normalized := update.NormalizeToolName(part)
			if selected[normalized] {
				appendName(normalized)
			}
		}
	}
	for _, name := range newOrder {
		appendName(update.NormalizeToolName(name))
	}
	return merged
}

func printSummaryContent(content string) {
	fmt.Printf("  %s\n", content)
}

var configCmd = &cobra.Command{
	Use:     "config",
	GroupID: "manage",
	Short:   "Manage configuration (interactive selection if no sub-command)",
	RunE: func(cmd *cobra.Command, args []string) error {
		newEnabledTools, newOrder, usageMode, cancelled, err := runInteractiveConfig()
		if err != nil {
			return fmt.Errorf("configuration prompt failed: %w", err)
		}
		if cancelled {
			fmt.Fprintln(cmd.OutOrStdout(), "Configuration cancelled.")
			return nil
		}
		// The interactive picker only lists installable tools; keep any
		// standalone usage providers the user enabled by name or in agent_order.
		oldOrder := viper.GetStringSlice("agent_order")
		newEnabledTools = appendStandaloneEntries(newEnabledTools, viper.GetStringSlice("enabled_tools"))
		newEnabledTools = appendStandaloneEntries(newEnabledTools, oldOrder)
		newOrder = mergeStandaloneIntoOrder(newOrder, oldOrder)
		viper.Set("enabled_tools", newEnabledTools)
		viper.Set("agent_order", newOrder)
		viper.Set("usage_display_mode", usageMode)
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Config updated successfully.")
		if len(newEnabledTools) > 0 {
			if err := setupTokens(newEnabledTools); err != nil {
				return err
			}
		}
		printConfigSummary(newEnabledTools, usageMode)
		return nil
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
	RunE: func(cmd *cobra.Command, args []string) error {
		tools := strings.Split(args[0], ",")
		var validTools []string
		for _, tool := range tools {
			tool = strings.TrimSpace(tool)
			found := false
			for _, t := range update.Tools {
				if t.MatchesName(tool) {
					validTools = append(validTools, t.BinaryName)
					found = true
					break
				}
			}
			if !found {
				// Standalone usage providers have no update.Tool entry but are
				// valid enabled_tools values (usage-only rows).
				if p, ok := usage.LookupStandalone(tool); ok {
					validTools = append(validTools, p.Name)
					found = true
				}
			}
			if !found {
				return fmt.Errorf("unknown tool: %s", tool)
			}
		}

		viper.Set("enabled_tools", validTools)
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Config updated.")
		return nil
	},
}

var configSetUsageModeCmd = &cobra.Command{
	Use:   "usage-mode <used|remaining>",
	Short: "Set usage display mode",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mode := strings.ToLower(strings.TrimSpace(args[0]))
		if mode != "used" && mode != "remaining" {
			return fmt.Errorf("invalid usage mode %q: use used or remaining", mode)
		}
		viper.Set("usage_display_mode", mode)
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Usage display mode set to %s.\n", mode)
		return nil
	},
}

var configSetMenubarTitleModeCmd = &cobra.Command{
	Use:   "menubar-title-mode <oct|compact>",
	Short: "Set menubar title display mode",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mode := strings.ToLower(strings.TrimSpace(args[0]))
		if mode != "oct" && mode != "compact" {
			return fmt.Errorf("invalid menubar title mode %q: use oct or compact", mode)
		}
		viper.Set("menubar_title_mode", mode)
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Menubar title mode set to %s.\n", mode)
		return nil
	},
}

var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset configuration to defaults",
	RunE: func(cmd *cobra.Command, args []string) error {
		viper.Set("enabled_tools", []string{})
		viper.Set("usage_display_mode", "remaining")
		viper.Set("menubar_title_mode", "oct")
		viper.Set("session_refresh_enabled", false)
		viper.Set("session_refresh_interval", "daily")
		viper.Set("session_refresh_hour", 9)
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Config reset to defaults.")
		return nil
	},
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		if configListJSON {
			encoder := json.NewEncoder(out)
			encoder.SetIndent("", "  ")
			return encoder.Encode(buildConfigSnapshot(configPathForDisplay()))
		}
		fmt.Fprintln(out, "=== one-click-tools config ===")
		fmt.Fprintf(out, "Config file: %s\n\n", viper.ConfigFileUsed())

		enabledTools := viper.GetStringSlice("enabled_tools")
		usageMode := strings.ToLower(strings.TrimSpace(viper.GetString("usage_display_mode")))
		if usageMode != "used" && usageMode != "remaining" {
			usageMode = "remaining"
		}
		titleMode := normalizedMenubarTitleMode(viper.GetString("menubar_title_mode"))
		fmt.Fprintf(out, "Usage display mode: %s\n", usageMode)
		fmt.Fprintf(out, "Menubar title mode: %s\n", titleMode)
		fmt.Fprintf(out, "Session refresh enabled: %v\n", viper.GetBool("session_refresh_enabled"))
		fmt.Fprintf(out, "Session refresh interval: %s\n", viper.GetString("session_refresh_interval"))
		fmt.Fprintf(out, "Session refresh hour: %d\n\n", viper.GetInt("session_refresh_hour"))
		fmt.Fprintln(out, "Enabled tools (agent-update):")

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
				fmt.Fprintf(out, "  ✓ %s\n", t.Colorize(t.Name))
			} else {
				fmt.Fprintf(out, "  ✗ %s\n", t.Colorize(t.Name))
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configResetCmd)
	configSetCmd.AddCommand(configSetToolsCmd)
	configSetCmd.AddCommand(configSetUsageModeCmd)
	configSetCmd.AddCommand(configSetMenubarTitleModeCmd)
}
