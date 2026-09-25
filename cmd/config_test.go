package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/viper"
	"github.com/suho-han/one-click-ai-tools/internal/update"
)

func TestConfigModel_HasControlRowsAtEnd(t *testing.T) {
	m := newConfigModel([]string{"codex"}, []string{"codex", "antigravity"})
	if len(m.items) < 2 {
		t.Fatalf("expected items")
	}
	if !m.items[len(m.items)-2].isToggleControl {
		t.Fatalf("expected penultimate item to be toggle control")
	}
	if !m.items[len(m.items)-1].isConfirmControl {
		t.Fatalf("expected last item to be confirm control")
	}
}

// TestConfigModel_CommaJoinedEnabledTools pins checkbox agreement with usage:
// a comma-joined enabled_tools entry is split by usage when fetching, so the
// picker must check those rows too.
func TestConfigModel_CommaJoinedEnabledTools(t *testing.T) {
	m := newConfigModel([]string{"codex,agy"}, []string{"codex", "agy"})
	for _, it := range m.items {
		if it.isToggleControl || it.isConfirmControl {
			continue
		}
		want := it.tool.BinaryName == "codex" || it.tool.BinaryName == "agy"
		if it.check != want {
			t.Fatalf("tool %s check = %v, want %v", it.tool.BinaryName, it.check, want)
		}
	}
}

func TestConfigModel_EnterTogglesSingleItem(t *testing.T) {
	m := newConfigModel([]string{"antigravity"}, []string{"codex", "antigravity"})
	for i := range m.items {
		m.items[i].cursor = i == 0
	}
	m.items[0].check = false

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(configModel)
	if !mm.items[0].check {
		t.Fatalf("expected enter to enable current item")
	}

	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm = updated.(configModel)
	if mm.items[0].check {
		t.Fatalf("expected second enter to disable current item")
	}
}

func TestConfigModel_ToggleControl_AllNoneByEnter(t *testing.T) {
	m := newConfigModel([]string{"codex"}, []string{"codex", "antigravity"})
	toggleIdx := -1
	for i, it := range m.items {
		if it.isToggleControl {
			toggleIdx = i
		}
	}
	for i := range m.items {
		m.items[i].cursor = i == toggleIdx
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(configModel)
	for i := range mm.items {
		if isControlItem(mm.items[i]) {
			continue
		}
		if !mm.items[i].check {
			t.Fatalf("expected all tools checked after enter on toggle control")
		}
	}

	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm = updated.(configModel)
	for i := range mm.items {
		if isControlItem(mm.items[i]) {
			continue
		}
		if mm.items[i].check {
			t.Fatalf("expected all tools unchecked after second enter on toggle control")
		}
	}
}

func TestConfigModel_EnterOnConfirmQuits(t *testing.T) {
	m := newConfigModel([]string{}, []string{"codex", "antigravity"})
	confirmIdx := len(m.items) - 1
	for i := range m.items {
		m.items[i].cursor = i == confirmIdx
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(configModel)
	if !mm.done {
		t.Fatalf("expected enter on confirm to finish")
	}
	if cmd == nil {
		t.Fatalf("expected quit command on confirm")
	}
}

func TestConfigModel_KJDoesNotReorder(t *testing.T) {
	m := newConfigModel([]string{}, []string{"codex", "antigravity"})
	if len(m.items) < 3 {
		t.Fatalf("expected at least two tools + control")
	}
	first := m.items[0].tool.BinaryName
	second := m.items[1].tool.BinaryName

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	mm := updated.(configModel)
	if mm.items[0].tool.BinaryName != first || mm.items[1].tool.BinaryName != second {
		t.Fatalf("expected K to not reorder items")
	}

	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	mm = updated.(configModel)
	if mm.items[0].tool.BinaryName != first || mm.items[1].tool.BinaryName != second {
		t.Fatalf("expected J to not reorder items")
	}
}

func TestConfigPromptsShareOneReader(t *testing.T) {
	orig := configPromptReader
	defer func() { configPromptReader = orig }()
	configPromptReader = bufio.NewReader(strings.NewReader("y\ntok-123\nocto-user\n"))

	yes, err := promptYesNo("update token?", false)
	if err != nil {
		t.Fatalf("promptYesNo: %v", err)
	}
	if !yes {
		t.Fatal("expected yes")
	}
	token, err := promptToken("> ")
	if err != nil {
		t.Fatalf("promptToken: %v", err)
	}
	if token != "tok-123" {
		t.Fatalf("expected tok-123, got %q", token)
	}
	user, err := promptToken("> ")
	if err != nil {
		t.Fatalf("promptToken: %v", err)
	}
	if user != "octo-user" {
		t.Fatalf("expected octo-user, got %q", user)
	}
}

func TestConfigPromptsTreatEOFAsDefaults(t *testing.T) {
	orig := configPromptReader
	defer func() { configPromptReader = orig }()
	configPromptReader = bufio.NewReader(strings.NewReader(""))

	yes, err := promptYesNo("update token?", true)
	if err != nil {
		t.Fatalf("promptYesNo: %v", err)
	}
	if !yes {
		t.Fatal("expected default yes on EOF")
	}
	token, err := promptToken("> ")
	if err != nil {
		t.Fatalf("promptToken: %v", err)
	}
	if token != "" {
		t.Fatalf("expected empty token on EOF, got %q", token)
	}
}

func TestConfigLayoutForHeight(t *testing.T) {
	tests := []struct {
		height         int
		wantVisibleMin int
	}{
		{height: 24, wantVisibleMin: 16},
		{height: 20, wantVisibleMin: 12},
		{height: 14, wantVisibleMin: 6},
		{height: 13, wantVisibleMin: 5},
		{height: 12, wantVisibleMin: 4},
		{height: 8, wantVisibleMin: 1},
		{height: 4, wantVisibleMin: 1},
	}
	for _, tt := range tests {
		visible := layoutForHeight(tt.height)
		if visible < tt.wantVisibleMin {
			t.Errorf("layoutForHeight(%d) visible = %d, want >= %d", tt.height, visible, tt.wantVisibleMin)
		}
	}
}

// TestConfigModel_WindowFitsTerminalHeight pins the user-facing contract: the
// rendered view never exceeds the terminal height, so the alt screen never
// clips the top of the tool list.
func TestConfigModel_WindowFitsTerminalHeight(t *testing.T) {
	for _, height := range []int{24, 20, 14, 13, 12, 10, 8} {
		m := newConfigModel(nil, nil)
		m.applyWindowSize(height)
		view := m.View()
		if lines := strings.Count(view, "\n"); lines > height {
			t.Errorf("height %d: view has %d lines, want <= %d", height, lines, height)
		}
	}
}

func TestConfigModel_WindowShowsScrollIndicator(t *testing.T) {
	m := newConfigModel(nil, nil)
	if got := len(m.items); got < 6 {
		t.Fatalf("expected a realistic item list, got %d items", got)
	}
	m.applyWindowSize(12) // 4 items visible, one line each

	view := m.View()
	if !strings.Contains(view, m.items[0].tool.Name) {
		t.Error("first item should be visible in the initial window")
	}
	if strings.Contains(view, m.items[4].tool.Name) {
		t.Error("item outside the window should not be rendered")
	}
	if !strings.Contains(view, fmt.Sprintf("↓ %d more", len(m.items)-4)) {
		t.Errorf("expected a scroll-down indicator for %d hidden items", len(m.items)-4)
	}
	if strings.Contains(view, "↑ ") {
		t.Error("no scroll-up indicator expected at the top of the list")
	}
}

func TestConfigModel_WindowFollowsCursorDown(t *testing.T) {
	m := newConfigModel(nil, nil)
	m.applyWindowSize(12)

	for i := 0; i < 4; i++ {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(configModel)
	}

	if m.index() != 4 || m.offset != 1 {
		t.Fatalf("cursor/offset = %d/%d, want 4/1", m.index(), m.offset)
	}
	view := m.View()
	if strings.Contains(view, m.items[0].tool.Name) {
		t.Error("scrolled-past item should be hidden")
	}
	if !strings.Contains(view, m.items[4].tool.Name) {
		t.Error("cursor item should be visible after scrolling")
	}
	if !strings.Contains(view, "↑ 1 more") {
		t.Error("expected scroll-up indicator after scrolling down")
	}
}

func TestConfigModel_WindowFollowsCursorWrapAround(t *testing.T) {
	m := newConfigModel(nil, nil)
	m.applyWindowSize(12)
	last := len(m.items) - 1

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(configModel)

	if m.index() != last {
		t.Fatalf("cursor = %d, want wrap to %d", m.index(), last)
	}
	view := m.View()
	if !strings.Contains(view, m.items[last].tool.Name) {
		t.Error("confirm row should be visible after wrap-around")
	}
	if !strings.Contains(view, fmt.Sprintf("↑ %d more", m.offset)) {
		t.Errorf("expected scroll-up indicator for %d hidden items, view:\n%s", m.offset, view)
	}
}

func TestConfigModel_ConfirmReachableInWindow(t *testing.T) {
	m := newConfigModel(nil, nil)
	m.applyWindowSize(12) // windowed mode, confirm row starts off-screen
	last := len(m.items) - 1

	for m.index() != last {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(configModel)
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(configModel)
	if !mm.done {
		t.Fatal("confirm row should quit even when windowed")
	}
}

func TestConfigModel_NoWindowSizeRendersAll(t *testing.T) {
	m := newConfigModel(nil, nil)
	view := m.View()
	for _, it := range m.items {
		// The toggle row renders as "Choose all"/"Choose none", never its
		// internal name.
		name := it.tool.Name
		if it.isToggleControl {
			name = "Choose all"
		}
		if !strings.Contains(view, name) {
			t.Fatalf("item %q missing without a known window size", name)
		}
	}
	if strings.Contains(view, "more\n") {
		t.Error("no scroll indicators expected without a known window size")
	}
}

// TestConfigSetTools_ToleratesEmptyParts pins CLI ergonomics: extra or
// trailing commas are skipped like every other enabled_tools consumer, while
// an all-empty argument is rejected instead of silently writing
// enabled_tools:[] (which means "all tools enabled").
func TestConfigSetTools_ToleratesEmptyPartsAndRejectsEmptyInput(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()
	viper.SetConfigFile(filepath.Join(t.TempDir(), "config.yaml"))

	if err := configSetToolsCmd.RunE(configSetToolsCmd, []string{"codex,,agy,"}); err != nil {
		t.Fatalf("set tools with empty parts: %v", err)
	}
	if got := viper.GetStringSlice("enabled_tools"); !slices.Equal(got, []string{"codex", "agy"}) {
		t.Fatalf("enabled_tools = %v, want [codex agy]", got)
	}

	if err := configSetToolsCmd.RunE(configSetToolsCmd, []string{","}); err == nil {
		t.Fatal("expected all-empty input to be rejected")
	}
	if got := viper.GetStringSlice("enabled_tools"); !slices.Equal(got, []string{"codex", "agy"}) {
		t.Fatalf("enabled_tools = %v, want unchanged [codex agy] after rejection", got)
	}
}

// TestConfigListTextShowsStandaloneProviders keeps the plain-text report in
// sync with the JSON snapshot: usage-only providers ride in enabled_tools and
// must be visible there too.
func TestConfigListTextShowsStandaloneProviders(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()
	viper.Set("enabled_tools", []string{"codex", "zai"})

	var buf bytes.Buffer
	configListCmd.SetOut(&buf)
	defer configListCmd.SetOut(nil)
	if err := configListCmd.RunE(configListCmd, nil); err != nil {
		t.Fatalf("config list: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Standalone usage providers") {
		t.Fatalf("missing standalone section in config list output:\n%s", out)
	}
	if !strings.Contains(out, "zai") {
		t.Fatalf("standalone provider zai missing from config list output:\n%s", out)
	}
}

func TestAppendStandaloneEntries(t *testing.T) {
	dst := appendStandaloneEntries([]string{"codex"}, []string{"zai", "codex"})
	if len(dst) != 2 || dst[0] != "codex" || dst[1] != "zai" {
		t.Fatalf("appendStandaloneEntries = %v, want [codex zai]", dst)
	}

	dst = appendStandaloneEntries(dst, []string{"ZAI"})
	if len(dst) != 2 {
		t.Fatalf("duplicate standalone appended: %v", dst)
	}

	// Installable tool names and unknown entries are ignored.
	dst = appendStandaloneEntries(dst, []string{"claude", "not-a-provider"})
	if len(dst) != 2 {
		t.Fatalf("non-standalone entries leaked into result: %v", dst)
	}

	// Comma-separated entries are split like the config values they mirror.
	dst = appendStandaloneEntries(nil, []string{"grok,deepseek"})
	if len(dst) != 2 || dst[0] != "grok" || dst[1] != "deepseek" {
		t.Fatalf("comma-split = %v, want [grok deepseek]", dst)
	}
}

func TestMergeStandaloneIntoOrder(t *testing.T) {
	// A standalone provider keeps its original position ahead of the tool.
	got := mergeStandaloneIntoOrder([]string{"codex", "claude"}, []string{"zai", "codex"})
	if !slices.Equal(got, []string{"zai", "codex", "claude"}) {
		t.Fatalf("merge = %v, want [zai codex claude]", got)
	}

	// Deselected tools drop; newly selected tools append after the carried
	// order.
	got = mergeStandaloneIntoOrder([]string{"claude"}, []string{"codex", "zai"})
	if !slices.Equal(got, []string{"zai", "claude"}) {
		t.Fatalf("merge = %v, want [zai claude]", got)
	}

	// Empty old order passes the picker order through untouched.
	got = mergeStandaloneIntoOrder([]string{"codex"}, nil)
	if !slices.Equal(got, []string{"codex"}) {
		t.Fatalf("merge = %v, want [codex]", got)
	}

	// Legacy tool names canonicalize while merging.
	got = mergeStandaloneIntoOrder([]string{"agy"}, []string{"gemini"})
	if !slices.Equal(got, []string{"agy"}) {
		t.Fatalf("merge = %v, want [agy]", got)
	}
}

// TestConfigModel_WindowFitsTerminalHeightMidList pins the mid-list case: with
// the cursor deep in the list both scroll indicators render, and the view must
// shed chrome instead of overflowing the alt screen.
func TestConfigModel_WindowFitsTerminalHeightMidList(t *testing.T) {
	for _, height := range []int{12, 10, 9, 8, 6, 4} {
		m := newConfigModel(nil, nil)
		m.applyWindowSize(height)
		last := len(m.items) - 1
		for m.index() != last/2 {
			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
			m = updated.(configModel)
		}
		view := m.View()
		if lines := strings.Count(view, "\n"); lines > height {
			t.Errorf("height %d: mid-list view has %d lines, want <= %d", height, lines, height)
		}
	}
}

// TestConfigModel_AddCodexAccountRowUnderCodex pins the action row's
// position: directly after the codex tool row, before the control rows.
func TestConfigModel_AddCodexAccountRowUnderCodex(t *testing.T) {
	m := newConfigModel([]string{"codex"}, []string{"codex", "antigravity", "claude"})
	codexIdx := -1
	for i, it := range m.items {
		if update.NormalizeToolName(it.tool.BinaryName) == "codex" {
			codexIdx = i
		}
	}
	if codexIdx < 0 {
		t.Fatal("codex row missing from picker items")
	}
	next := m.items[codexIdx+1]
	if !next.isAddAccountControl {
		t.Fatalf("item after codex = %+v, want the add-codex-account action row", next)
	}
	// The action row renders as an action, not a checkbox.
	if lines := m.renderItemLines(next); !strings.Contains(lines[0], "➕") || strings.Contains(lines[0], "[ ]") {
		t.Fatalf("action row rendered as %q, want ➕ without a checkbox mark", lines[0])
	}
	// ➕ must start in the tool-name column ("OpenAI Codex" et al), i.e. the
	// row is indented by the mark's width.
	stripANSI := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString
	toolLine := stripANSI(m.renderItemLines(m.items[codexIdx])[0], "")
	actionLine := stripANSI(m.renderItemLines(next)[0], "")
	nameCol := strings.Index(toolLine, "OpenAI Codex")
	emojiCol := strings.Index(actionLine, "➕")
	if nameCol < 0 || emojiCol != nameCol {
		t.Fatalf("➕ column = %d, want the tool-name column %d (tool line %q, action line %q)", emojiCol, nameCol, toolLine, actionLine)
	}
}

func TestConfigModel_EnterOnAddAccountRequestsFlow(t *testing.T) {
	m := newConfigModel([]string{"codex"}, []string{"codex"})
	addIdx := -1
	for i, it := range m.items {
		if it.isAddAccountControl {
			addIdx = i
		}
	}
	for i := range m.items {
		m.items[i].cursor = i == addIdx
	}
	// Tool selection state must not matter: the account flow saves the
	// account only, so toggles stay untouched.
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(configModel)
	if mm.accountAction == nil || mm.accountAction.kind != "add" || !mm.done {
		t.Fatalf("enter on add-account row: action=%+v done=%v, want add action and done", mm.accountAction, mm.done)
	}
	if cmd == nil {
		t.Fatal("expected quit command on add-account row")
	}
}
