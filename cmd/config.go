package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/suho-han/one-click-ai-tools/internal/update"
	"github.com/suho-han/one-click-ai-tools/internal/usage"
)

type toolItem struct {
	tool   update.Tool
	check  bool
	cursor bool
	// choose all/none control row (not a real tool)
	isToggleControl bool
	// confirm control row (not a real tool)
	isConfirmControl bool
	// add-codex-account action row (not a real tool); Enter quits the picker
	// into the account prompt flow instead of toggling anything.
	isAddAccountControl bool
	// registered codex account row (not a real tool); Enter expands/collapses
	// its reconnect/disconnect action rows.
	isAccountRow    bool
	accountAlias    string
	accountHome     string
	accountExpanded bool
	// reconnect/disconnect action row of the expanded account above it
	isAccountActionRow bool
	accountActionKind  string // "reconnect" | "disconnect"
	// collapsible "Manage existing accounts" group row; Enter expands into
	// the account rows below it.
	isManageAccountsRow bool
	manageCount         int
}

// pendingAccountAction is the account picker action a row requested; the
// caller runs it after the picker quits.
type pendingAccountAction struct {
	kind  string // "add" | "reconnect" | "disconnect"
	alias string
	home  string
}

// isControlItem reports whether an item is chrome (control/action rows)
// rather than a selectable tool; chrome rows never take part in tool
// collection, toggle-all, or checkbox rendering.
func isControlItem(it toolItem) bool {
	return it.isToggleControl || it.isConfirmControl || it.isAddAccountControl ||
		it.isAccountRow || it.isAccountActionRow || it.isManageAccountsRow
}

// accountActionItem builds one expanded-account action row.
func accountActionItem(alias, home, kind string) toolItem {
	return toolItem{
		isAccountActionRow: true,
		accountAlias:       alias,
		accountHome:        home,
		accountActionKind:  kind,
	}
}

type configModel struct {
	items     []toolItem
	cancelled bool
	done      bool
	// accountAction is set when the user pressed Enter on an account picker
	// row ("Add codex account…", reconnect, disconnect); the caller runs it
	// after the picker quits instead of saving the tool selection.
	accountAction *pendingAccountAction
	// manageExpanded tracks whether the "Manage existing accounts" group is
	// open; accountExpanded remembers per-account expansion across the
	// rebuilds that syncAccountsSection performs.
	manageExpanded  bool
	accountExpanded map[string]bool
	// Terminal-fit windowing: bubbletea paints the whole View() into the
	// alt screen, and the alt screen has no scrollback -- an oversized view
	// loses its top. The model therefore renders a sliding window of items
	// sized from tea.WindowSizeMsg. viewHeight == 0 (no size known yet, e.g.
	// unit tests) renders everything.
	viewHeight  int
	offset      int // first visible item index
	visibleRows int // visible item count (one line per item)
}

// viewChrome is the number of lines around the item list: 1 header + 1 blank
// + 4 help lines. Scroll indicators are carved out of the item budget below.
const viewChrome = 6

// layoutForHeight resolves the number of visible item rows for a terminal
// height. Every item renders as a single line, so the budget is all that is
// left after the chrome and up to 2 scroll-indicator rows.
func layoutForHeight(height int) int {
	itemBudget := height - viewChrome - 2 // reserve up to 2 scroll-indicator rows
	if itemBudget < 1 {
		return 1
	}
	return itemBudget
}

// applyWindowSize records the terminal height and keeps the cursor inside
// the visible window.
func (m *configModel) applyWindowSize(height int) {
	if height <= 0 {
		return
	}
	m.viewHeight = height
	m.visibleRows = layoutForHeight(height)
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
				// Split comma-joined entries to match how usage consumes
				// enabled_tools, so the checkboxes agree with fetched rows.
				for _, part := range strings.Split(et, ",") {
					if update.NormalizeToolName(part) == update.NormalizeToolName(t.BinaryName) {
						enabled = true
						break
					}
				}
			}
		}
		items = append(items, toolItem{
			tool:   t,
			check:  enabled,
			cursor: i == 0,
		})
	}

	// The codex group sits under the codex tool row: the add-account action
	// row, then a "Manage existing accounts" group (only when accounts are
	// registered) that expands into the per-account rows.
	for i, it := range items {
		if update.NormalizeToolName(it.tool.BinaryName) != "codex" {
			continue
		}
		accounts := usage.AccountsForProvider("codex")
		extras := []toolItem{{
			tool: update.Tool{
				Name:       "Add codex account…",
				BinaryName: "__add_codex_account__",
				Icon:       "➕",
				HexColor:   "#38BDF8",
			},
			isAddAccountControl: true,
		}}
		if len(accounts) > 0 {
			extras = append(extras, toolItem{
				isManageAccountsRow: true,
				manageCount:         len(accounts),
			})
		}
		items = append(items, make([]toolItem, len(extras))...)
		copy(items[i+1+len(extras):], items[i+1:])
		copy(items[i+1:], extras)
		break
	}

	items = append(items, toolItem{
		tool: update.Tool{
			Name:       "Choose all / Choose none",
			BinaryName: "__toggle_all_none__",
			Icon:       "⇄",
			HexColor:   "#9CA3AF",
		},
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
		check:            false,
		cursor:           false,
		isConfirmControl: true,
	})
	return configModel{items: items, accountExpanded: map[string]bool{}}
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
				if m.items[i].isAddAccountControl {
					m.accountAction = &pendingAccountAction{kind: "add"}
					m.done = true
					return m, tea.Quit
				}
				if m.items[i].isAccountRow {
					alias := m.items[i].accountAlias
					m.accountExpanded[alias] = !m.accountExpanded[alias]
					m.syncAccountsSection()
					return m, nil
				}
				if m.items[i].isManageAccountsRow {
					m.manageExpanded = !m.manageExpanded
					m.syncAccountsSection()
					return m, nil
				}
				if m.items[i].isAccountActionRow {
					m.accountAction = &pendingAccountAction{
						kind:  m.items[i].accountActionKind,
						alias: m.items[i].accountAlias,
						home:  m.items[i].accountHome,
					}
					m.done = true
					return m, tea.Quit
				}
				if m.items[i].isToggleControl {
					allChecked := true
					for j := range m.items {
						if isControlItem(m.items[j]) {
							continue
						}
						if !m.items[j].check {
							allChecked = false
							break
						}
					}
					for j := range m.items {
						if isControlItem(m.items[j]) {
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

// syncAccountsSection rebuilds the items list from the manage group's state:
// account rows (and their reconnect/disconnect actions) exist only while the
// "Manage existing accounts" group is expanded, and an account's actions
// exist only while that account row is expanded. The cursor follows its row
// through the rebuild and lands on the manage row when its row disappeared.
func (m *configModel) syncAccountsSection() {
	cursor := m.index()
	var restore string
	if cursor >= 0 {
		it := m.items[cursor]
		switch {
		case it.isAccountRow:
			restore = "row:" + it.accountAlias
			m.items[cursor].cursor = false
		case it.isAccountActionRow:
			restore = "action:" + it.accountActionKind + ":" + it.accountAlias
			m.items[cursor].cursor = false
		}
	}

	var out []toolItem
	for _, it := range m.items {
		if it.isAccountRow || it.isAccountActionRow {
			continue
		}
		out = append(out, it)
		if it.isManageAccountsRow && m.manageExpanded {
			for _, account := range usage.AccountsForProvider("codex") {
				row := toolItem{
					isAccountRow: true,
					accountAlias: account.Name,
					accountHome:  account.Home,
				}
				out = append(out, row)
				if m.accountExpanded[account.Name] {
					out = append(out,
						accountActionItem(account.Name, account.Home, "reconnect"),
						accountActionItem(account.Name, account.Home, "disconnect"),
					)
				}
			}
		}
	}
	m.items = out

	apply := func(key string) bool {
		for i := range m.items {
			it := m.items[i]
			switch {
			case it.isAccountRow:
				if key == "row:"+it.accountAlias {
					m.items[i].cursor = true
					return true
				}
			case it.isAccountActionRow:
				if key == "action:"+it.accountActionKind+":"+it.accountAlias {
					m.items[i].cursor = true
					return true
				}
			}
		}
		return false
	}
	if restore != "" && apply(restore) {
		return
	}
	// The cursor's row was hidden by the collapse: park it on the manage row.
	if restore != "" {
		for i := range m.items {
			if m.items[i].isManageAccountsRow {
				m.items[i].cursor = true
				return
			}
		}
	}
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

// renderItemLines renders one item as a single name row (no icon art).
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
			if isControlItem(x) {
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

	if it.isAddAccountControl {
		// An action, not a checkbox: render with the ➕ icon instead of a
		// toggle mark so it doesn't read as a selectable tool. The indent is
		// the mark's width ("[x]" plus its trailing space), so ➕ starts in
		// the same column as the tool names ("OpenAI Codex" et al) — the same
		// alignment a tab character would land on, but terminal-independent.
		return []string{fmt.Sprintf("%s    ➕ %s", cursor, name)}
	}
	if it.isManageAccountsRow {
		marker := "▸"
		if m.manageExpanded {
			marker = "▾"
		}
		label := fmt.Sprintf("Manage existing accounts (%d)", it.manageCount)
		if it.cursor {
			label = it.tool.ColorizeWithBackgroundBlackText(label)
		} else {
			label = it.tool.Colorize(label)
		}
		return []string{fmt.Sprintf("%s%s %s", cursor, marker, label)}
	}
	if it.isAccountRow {
		// Registered account: label only, with a hint when its home directory
		// is gone and an expand marker while its actions are visible.
		marker := "▸"
		if it.accountExpanded {
			marker = "▾"
		}
		state := ""
		if _, err := os.Stat(it.accountHome); err != nil {
			state = " (missing)"
		}
		return []string{fmt.Sprintf("%s%s codex:%s%s", cursor, marker, it.tool.Colorize(it.accountAlias), state)}
	}
	if it.isAccountActionRow {
		icon := "🔄"
		if it.accountActionKind == "disconnect" {
			icon = "❌"
		}
		label := "Reconnect"
		if it.accountActionKind == "disconnect" {
			label = "Disconnect"
		}
		return []string{fmt.Sprintf("%s    %s %s", cursor, icon, it.tool.Colorize(label))}
	}
	return []string{fmt.Sprintf("%s%s %s", cursor, mark, name)}
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

func runInteractiveConfig() ([]string, []string, bool, *pendingAccountAction, error) {
	enabledTools := viper.GetStringSlice("enabled_tools")
	agentOrder := viper.GetStringSlice("agent_order")
	model := newConfigModel(enabledTools, agentOrder)
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, nil, false, nil, err
	}
	m, ok := finalModel.(configModel)
	if !ok {
		return nil, nil, false, nil, fmt.Errorf("unexpected model type")
	}
	if m.cancelled {
		return nil, nil, true, nil, nil
	}
	if m.accountAction != nil {
		return nil, nil, false, m.accountAction, nil
	}
	var selected []string
	var order []string
	for _, it := range m.items {
		if isControlItem(it) {
			continue
		}
		if it.check {
			selected = append(selected, it.tool.BinaryName)
		}
		order = append(order, it.tool.BinaryName)
	}
	return selected, order, false, nil, nil
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
	// Match oct's other confirm prompts (oct update, agent installs): the
	// default choice is the uppercase letter.
	options := "[y/N]"
	if defaultYes {
		options = "[Y/n]"
	}
	fmt.Printf("%s %s: ", prompt, options)
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

func printConfigSummary(enabledTools []string) {
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
	Short:   "⚙️ Manage configuration (interactive selection if no sub-command)",
	RunE: func(cmd *cobra.Command, args []string) error {
		newEnabledTools, newOrder, cancelled, action, err := runInteractiveConfig()
		if err != nil {
			return fmt.Errorf("configuration prompt failed: %w", err)
		}
		if cancelled {
			fmt.Fprintln(cmd.OutOrStdout(), "Configuration cancelled.")
			return nil
		}
		if action != nil {
			// The picker was left via an account row; run that account action
			// only, leaving the tool selection untouched.
			switch action.kind {
			case "reconnect":
				return runReconnectCodexAccountFlow(cmd, action.alias, action.home)
			case "disconnect":
				return runDisconnectCodexAccountFlow(cmd, action.alias)
			default:
				return runAddCodexAccountFlow(cmd)
			}
		}
		// The interactive picker only lists installable tools; keep any
		// standalone usage providers the user enabled by name or in agent_order.
		oldOrder := viper.GetStringSlice("agent_order")
		newEnabledTools = appendStandaloneEntries(newEnabledTools, viper.GetStringSlice("enabled_tools"))
		newEnabledTools = appendStandaloneEntries(newEnabledTools, oldOrder)
		newOrder = mergeStandaloneIntoOrder(newOrder, oldOrder)
		viper.Set("enabled_tools", newEnabledTools)
		viper.Set("agent_order", newOrder)
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Config updated successfully.")
		if len(newEnabledTools) > 0 {
			if err := setupTokens(newEnabledTools); err != nil {
				return err
			}
		}
		printConfigSummary(newEnabledTools)
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
			if tool == "" {
				// Tolerate empty parts ("a,,b", trailing comma) like every
				// other enabled_tools consumer.
				continue
			}
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
		if len(validTools) == 0 {
			// An all-empty argument must not silently write enabled_tools:[],
			// which means "every installable tool enabled".
			return fmt.Errorf("no tool provided (enabled_tools must include at least one provider)")
		}

		viper.Set("enabled_tools", validTools)
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Config updated.")
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

// accountEntriesForConfig converts account entries back into the documented
// provider-keyed accounts shape for viper to persist. The reserved token-kind
// fields round-trip too (only when set, to keep hand-maintained config files
// free of empty keys) — dropping them here would silently erase key/env
// entries on every account management command.
func accountEntriesForConfig(entries []usage.AccountEntry) map[string][]map[string]string {
	out := make(map[string][]map[string]string)
	for _, e := range entries {
		entry := map[string]string{"name": e.Name, "home": e.Home}
		if e.Key != "" {
			entry["key"] = e.Key
		}
		if e.Env != "" {
			entry["env"] = e.Env
		}
		out[e.Provider] = append(out[e.Provider], entry)
	}
	return out
}

// upsertAccount validates and writes one account entry into the in-memory
// config; the caller persists it via writeConfig. replaced reports whether an
// entry with the same provider and name already existed and was updated.
func upsertAccount(provider, name, home string) (bool, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if !usage.ProviderSupportsAccounts(provider) {
		return false, fmt.Errorf("provider %s does not support multiple accounts (supported: codex, kimi, qwen, grok)", provider)
	}
	name = strings.ToLower(strings.TrimSpace(name))
	if err := usage.ValidateAccountAlias(name); err != nil {
		return false, err
	}
	home = strings.TrimSpace(home)
	if home == "" {
		return false, fmt.Errorf("home directory path must not be empty")
	}
	entries := usage.AccountConfigEntries()
	for i, entry := range entries {
		if entry.Provider == provider && entry.Name == name {
			entries[i].Home = home
			viper.Set("accounts", accountEntriesForConfig(entries))
			return true, nil
		}
	}
	entries = append(entries, usage.AccountEntry{Provider: provider, Name: name, Home: home})
	viper.Set("accounts", accountEntriesForConfig(entries))
	return false, nil
}

// runInteractiveLogin launches `codex login` with CODEX_HOME pointed at the
// account home, handing the terminal over so the browser OAuth flow works
// normally. Package-level var so tests can stub the spawn.
var runInteractiveLogin = func(home string) error {
	bin, err := exec.LookPath("codex")
	if err != nil {
		return err
	}
	login := exec.Command(bin, "login")
	login.Env = append(os.Environ(), "CODEX_HOME="+home)
	login.Stdin = os.Stdin
	login.Stdout = os.Stdout
	login.Stderr = os.Stderr
	return login.Run()
}

// runReconnectCodexAccountFlow re-runs `codex login` for one registered
// account, refreshing its saved tokens in place.
func runReconnectCodexAccountFlow(cmd *cobra.Command, alias, home string) error {
	out := cmd.OutOrStdout()
	label := usage.AccountProviderName("codex", alias)
	if err := os.MkdirAll(home, 0o700); err != nil {
		return fmt.Errorf("create account home: %w", err)
	}
	fmt.Fprintf(out, "Running 'codex login' for %s — complete it in the browser...\n", label)
	if err := runInteractiveLogin(home); err != nil {
		fmt.Fprintf(out, "Login did not complete (%v).\nRetry later with: CODEX_HOME=%s codex login\n", err, home)
		return nil
	}
	if _, err := os.Stat(filepath.Join(home, "auth.json")); err == nil {
		fmt.Fprintf(out, "✓ Reconnected %s.\n", label)
	} else {
		fmt.Fprintf(out, "Login finished, but %s was not found.\n", filepath.Join(home, "auth.json"))
	}
	return nil
}

// runDisconnectCodexAccountFlow removes one account's config entry after a
// confirmation. The home directory with its saved credentials stays on disk.
func runDisconnectCodexAccountFlow(cmd *cobra.Command, alias string) error {
	out := cmd.OutOrStdout()
	label := usage.AccountProviderName("codex", alias)
	confirm, err := promptYesNo(fmt.Sprintf("Disconnect %s? (config entry only; saved credentials stay on disk)", label), false)
	if err != nil {
		return err
	}
	if !confirm {
		fmt.Fprintln(out, "Cancelled.")
		return nil
	}
	entries := usage.AccountConfigEntries()
	kept := make([]usage.AccountEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Provider == "codex" && entry.Name == alias {
			continue
		}
		kept = append(kept, entry)
	}
	viper.Set("accounts", accountEntriesForConfig(kept))
	if err := writeConfig(); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	fmt.Fprintf(out, "Account %s disconnected. To remove its credentials too, delete the home directory shown in 'oct config account list'.\n", label)
	return nil
}

// maskEmailPrefix turns an email prefix into a privacy-safe account alias:
// "hansuho36" -> "han***o36" (7+ chars keep first/last 3, 4-6 keep
// first/last 1, shorter is fully masked). Non-alphanumerics are stripped
// before masking so dots and plus tags never leak into the result.
func maskEmailPrefix(prefix string) string {
	p := strings.ToLower(strings.TrimSpace(prefix))
	p = regexp.MustCompile(`[^a-z0-9]`).ReplaceAllString(p, "")
	r := []rune(p)
	switch {
	case len(r) >= 7:
		return string(r[:3]) + "***" + string(r[len(r)-3:])
	case len(r) >= 4:
		return string(r[:1]) + "***" + string(r[len(r)-1:])
	default:
		return "***"
	}
}

// sanitizeHomeSegment makes a path segment out of an email prefix: lowercase,
// only [a-z0-9._-], no glob characters ('*') and no leading/trailing noise.
func sanitizeHomeSegment(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-.")
	if out == "" {
		out = "account"
	}
	return out
}

// runAddCodexAccountFlow runs the picker's "Add codex account…" action with
// no typing: list the registered accounts, launch `codex login` in a fresh
// temporary home (pre-created — the CLI refuses CODEX_HOME paths that do not
// exist), then read the account email from the login JWT and register the
// account under the masked prefix (hansuho36@gmail.com -> codex:han***o36).
// The user only completes the browser login.
func runAddCodexAccountFlow(cmd *cobra.Command) error {
	out := cmd.OutOrStdout()

	fmt.Fprintln(out, "Codex accounts:")
	accounts := usage.AccountsForProvider("codex")
	if len(accounts) == 0 {
		fmt.Fprintln(out, "  (none registered yet)")
	}
	for _, account := range accounts {
		state := ""
		if _, err := os.Stat(account.Home); err != nil {
			state = " (missing)"
		}
		fmt.Fprintf(out, "  %s -> %s%s\n", usage.AccountProviderName(account.Provider, account.Name), account.Home, state)
	}

	userHome, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	// The temporary home lives under $HOME so the post-login rename onto the
	// final ~/.codex-<name> stays on one filesystem. Pre-creating it also
	// fixes `codex login` refusing CODEX_HOME paths that do not exist.
	tmpHome, err := os.MkdirTemp(userHome, ".codex-tmp-login-")
	if err != nil {
		return fmt.Errorf("create temporary codex home: %w", err)
	}

	fmt.Fprintln(out, "Running 'codex login' — complete it in the browser...")
	if err := runInteractiveLogin(tmpHome); err != nil {
		os.RemoveAll(tmpHome)
		fmt.Fprintf(out, "Login did not complete (%v).\nRetry from the picker, or run: oct config account add codex <name> <dir>\n", err)
		return nil
	}

	email := usage.CodexLoginEmail(tmpHome)
	alias, home := "", ""
	if email != "" {
		prefix, _, _ := strings.Cut(email, "@")
		alias = maskEmailPrefix(prefix)
		home = filepath.Join(userHome, ".codex-"+sanitizeHomeSegment(prefix))
		fmt.Fprintf(out, "✓ Logged in as %s\n", email)
	} else {
		// Token without an email claim: fall back to asking for a name.
		alias, err = promptToken("Account email unavailable; enter an account name (empty = cancel): ")
		if err != nil {
			return err
		}
		alias = strings.ToLower(strings.TrimSpace(alias))
		if alias == "" {
			os.RemoveAll(tmpHome)
			fmt.Fprintln(out, "No account name entered; cancelled.")
			return nil
		}
		home = filepath.Join(userHome, ".codex-"+sanitizeHomeSegment(alias))
	}

	// Logging into an account that is already registered is a no-op.
	for _, account := range accounts {
		if account.Name == alias {
			os.RemoveAll(tmpHome)
			fmt.Fprintf(out, "%s is already registered — nothing to add.\n", usage.AccountProviderName("codex", alias))
			return nil
		}
	}

	if home != tmpHome {
		if err := os.Rename(tmpHome, home); err != nil {
			home = tmpHome // keep the temporary path rather than failing
		}
	}
	if _, err := upsertAccount("codex", alias, home); err != nil {
		return err
	}
	if err := writeConfig(); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	fmt.Fprintf(out, "Registered %s (home: %s). Run 'oct usage' to see its row.\n", usage.AccountProviderName("codex", alias), home)
	return nil
}

// persistAccount upserts one account, writes the config, and prints the
// result including a hint when the home directory does not exist yet.
func persistAccount(cmd *cobra.Command, provider, name, home string) error {
	replaced, err := upsertAccount(provider, name, home)
	if err != nil {
		return err
	}
	if err := writeConfig(); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	label := usage.AccountProviderName(provider, name)
	resolved := usage.ResolveAccountHomePath(home)
	if _, err := os.Stat(resolved); err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "note: %s does not exist yet; log this account in there (e.g. CODEX_HOME=%s codex login)\n", resolved, resolved)
	}
	if replaced {
		fmt.Fprintf(cmd.OutOrStdout(), "Account %s updated (home: %s).\n", label, resolved)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Account %s added (home: %s). Run 'oct usage' to see its row.\n", label, resolved)
	}
	return nil
}

var configAccountCmd = &cobra.Command{
	Use:     "account",
	Aliases: []string{"accounts"},
	Short:   "Manage extra usage accounts (codex, kimi, qwen, grok)",
}

var configAccountAddCmd = &cobra.Command{
	Use:   "add <provider> <name> [home-dir]",
	Short: "Register an extra usage account (own credential directory)",
	Long: `Register an extra usage account for 'oct usage'.

Supported providers: codex, kimi, qwen, grok. Each account reads its own
credential directory instead of the default one, the same way the default
row reads ~/.codex, ~/.kimi-code, ~/.qwen, or ~/.grok.

With two arguments the home directory is derived as ~/.<provider>-<name>
(e.g. 'oct config account add codex han' -> ~/.codex-han); pass an explicit
path as the third argument to override.

The account gets its own usage row (<provider>:<name>) right below the
default provider row in 'oct usage' and every statusline/menubar surface.

Example:
  CODEX_HOME=~/.codex-work codex login
  oct config account add codex work ~/.codex-work`,
	Args: cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		provider := strings.ToLower(strings.TrimSpace(args[0]))
		name := strings.ToLower(strings.TrimSpace(args[1]))
		home := ""
		if len(args) == 3 {
			home = strings.TrimSpace(args[2])
		}
		if home == "" {
			home = fmt.Sprintf("~/.%s-%s", provider, sanitizeHomeSegment(name))
		}
		return persistAccount(cmd, provider, name, home)
	},
}

var configAccountRemoveCmd = &cobra.Command{
	Use:   "remove <provider> <name>",
	Short: "Unregister an extra usage account",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		provider := strings.ToLower(strings.TrimSpace(args[0]))
		name := strings.ToLower(strings.TrimSpace(args[1]))
		entries := usage.AccountConfigEntries()
		kept := make([]usage.AccountEntry, 0, len(entries))
		removed := false
		for _, entry := range entries {
			if entry.Provider == provider && entry.Name == name {
				removed = true
				continue
			}
			kept = append(kept, entry)
		}
		if !removed {
			return fmt.Errorf("no %s account named %s (see 'oct config account list')", provider, name)
		}
		viper.Set("accounts", accountEntriesForConfig(kept))
		if err := writeConfig(); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Account %s removed.\n", usage.AccountProviderName(provider, name))
		return nil
	},
}

var configAccountListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered extra usage accounts",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		entries := usage.AccountConfigEntries()
		if len(entries) == 0 {
			fmt.Fprintln(out, "No extra usage accounts configured.")
			fmt.Fprintln(out, "Add one with: oct config account add <provider> <name> <home-dir>")
			fmt.Fprintln(out, "Supported providers: codex, kimi, qwen, grok")
			return nil
		}
		for _, entry := range entries {
			resolved := usage.ResolveAccountHomePath(entry.Home)
			state := ""
			if _, err := os.Stat(resolved); err != nil {
				state = " (missing)"
			}
			fmt.Fprintf(out, "%s -> %s%s\n", usage.AccountProviderName(entry.Provider, entry.Name), resolved, state)
		}
		return nil
	},
}

var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset configuration to defaults",
	RunE: func(cmd *cobra.Command, args []string) error {
		viper.Set("enabled_tools", []string{})
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
		titleMode := normalizedMenubarTitleMode(viper.GetString("menubar_title_mode"))
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
					// Comma-joined entries are legal config values; split them
					// like usage does so this report matches actual fetching.
					for _, part := range strings.Split(et, ",") {
						if strings.EqualFold(strings.TrimSpace(part), t.BinaryName) {
							enabled = true
							break
						}
					}
				}
			}

			if enabled {
				fmt.Fprintf(out, "  ✓ %s\n", t.Colorize(t.Name))
			} else {
				fmt.Fprintf(out, "  ✗ %s\n", t.Colorize(t.Name))
			}
		}

		// Standalone usage providers ride in enabled_tools but are not
		// agent-update targets; report them so this text view covers every
		// provider the JSON snapshot (and the menubar) shows.
		fmt.Fprintln(out, "\nStandalone usage providers (usage-only):")
		shownStandalone := map[string]bool{}
		foundStandalone := false
		for _, et := range enabledTools {
			for _, part := range strings.Split(et, ",") {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				p, ok := usage.LookupStandalone(part)
				if !ok || shownStandalone[p.Name] {
					continue
				}
				shownStandalone[p.Name] = true
				foundStandalone = true
				fmt.Fprintf(out, "  ✓ %s\n", p.Name)
			}
		}
		if !foundStandalone {
			fmt.Fprintln(out, "  (none)")
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
	configSetCmd.AddCommand(configSetMenubarTitleModeCmd)
	configCmd.AddCommand(configAccountCmd)
	configAccountCmd.AddCommand(configAccountAddCmd)
	configAccountCmd.AddCommand(configAccountRemoveCmd)
	configAccountCmd.AddCommand(configAccountListCmd)
}
