package cmd

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestConfigModel_HasControlRowsAtEnd(t *testing.T) {
	m := newConfigModel([]string{"codex"}, []string{"codex", "gemini"})
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
	m := newConfigModel([]string{"gemini"}, []string{"codex", "gemini"})
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
	m := newConfigModel([]string{"codex"}, []string{"codex", "gemini"})
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
	m := newConfigModel([]string{}, []string{"codex", "gemini"})
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
	m := newConfigModel([]string{}, []string{"codex", "gemini"})
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
