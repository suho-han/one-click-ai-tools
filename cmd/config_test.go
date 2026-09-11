package cmd

import (
	"bufio"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
	toggleIdx := len(m.items) - 2
	for i := range m.items {
		m.items[i].cursor = i == toggleIdx
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(configModel)
	for i := 0; i < len(mm.items)-2; i++ {
		if !mm.items[i].check {
			t.Fatalf("expected all tools checked after enter on toggle control")
		}
	}

	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm = updated.(configModel)
	for i := 0; i < len(mm.items)-2; i++ {
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
	configPromptReader = bufio.NewReader(strings.NewReader("y\ntok-123\nocto-user\nu\n"))

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
	mode, err := promptUsageMode("remaining")
	if err != nil {
		t.Fatalf("promptUsageMode: %v", err)
	}
	if mode != "used" {
		t.Fatalf("expected used mode, got %q", mode)
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
	mode, err := promptUsageMode("remaining")
	if err != nil {
		t.Fatalf("promptUsageMode: %v", err)
	}
	if mode != "remaining" {
		t.Fatalf("expected default mode on EOF, got %q", mode)
	}
}

func TestConfigLayoutForHeight(t *testing.T) {
	tests := []struct {
		height         int
		wantRowHeight  int
		wantVisibleMin int
	}{
		{height: 24, wantRowHeight: 3, wantVisibleMin: 5},
		{height: 20, wantRowHeight: 3, wantVisibleMin: 4},
		{height: 14, wantRowHeight: 3, wantVisibleMin: 2},
		{height: 13, wantRowHeight: 3, wantVisibleMin: 1},
		{height: 12, wantRowHeight: 1, wantVisibleMin: 4},
		{height: 8, wantRowHeight: 1, wantVisibleMin: 1},
	}
	for _, tt := range tests {
		rowHeight, visible := layoutForHeight(tt.height)
		if rowHeight != tt.wantRowHeight {
			t.Errorf("layoutForHeight(%d) rowHeight = %d, want %d", tt.height, rowHeight, tt.wantRowHeight)
		}
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
	m.applyWindowSize(20) // 4 items visible at 3 rows each

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
	m.applyWindowSize(20)

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
	m.applyWindowSize(20)
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
	m.applyWindowSize(12) // compact mode, few rows
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
