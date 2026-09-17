package cmd

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/viper"
)

type alertSettingsDraft struct {
	values map[string]string
}

type alertSettingsItem struct {
	key     string
	label   string
	value   string
	cursor  bool
	confirm bool
}

type alertSettingsModel struct {
	items           []alertSettingsItem
	cancelled       bool
	done            bool
	editing         bool
	editingValue    string
	validationError string
	viewHeight      int
	offset          int
	visibleRows     int
}

var alertSettingsRows = []struct{ key, label string }{
	{"enabled", "Enabled"},
	{"threshold_percent", "Threshold percent"},
	{"critical_percent", "Critical percent"},
	{"cooldown_minutes", "Cooldown minutes"},
	{"quiet_hours", "Quiet hours"},
	{"timezone", "Timezone"},
	{"threshold.default", "Threshold default"},
	{"threshold.5h", "Threshold 5h"},
	{"threshold.7d", "Threshold 7d"},
}

const alertSettingsViewChrome = 6

func alertSettingsDraftFromViper() alertSettingsDraft {
	cfg := buildAlertConfigFromViper(viper.GetBool("usage_alert_enabled"))
	threshold := cfg.ThresholdPct
	if threshold <= 0 {
		threshold = 80
	}
	// Runtime evaluation inherits 5h/7d windows from the global default
	// (thresholdFor: window -> default -> legacy percent), so the draft must
	// show the same inherited value instead of the legacy percent.
	effectiveDefault := alertThresholdOrDefault(cfg.GlobalThresholds, "default", threshold)
	values := map[string]string{
		"enabled":           strconv.FormatBool(cfg.Enabled),
		"threshold_percent": formatAlertSettingNumber(threshold),
		"critical_percent":  formatAlertSettingNumber(cfg.CriticalPct),
		"cooldown_minutes":  strconv.Itoa(cfg.CooldownMinutes),
		"quiet_hours":       cfg.QuietHours,
		"timezone":          cfg.Timezone,
		"threshold.default": formatAlertSettingNumber(effectiveDefault),
		"threshold.5h":      formatAlertSettingNumber(alertThresholdOrDefault(cfg.GlobalThresholds, "5h", effectiveDefault)),
		"threshold.7d":      formatAlertSettingNumber(alertThresholdOrDefault(cfg.GlobalThresholds, "7d", effectiveDefault)),
	}
	if cfg.CriticalPct <= 0 {
		values["critical_percent"] = "98"
	}
	if cfg.CooldownMinutes <= 0 {
		values["cooldown_minutes"] = "360"
	}
	return alertSettingsDraft{values: values}
}

func alertThresholdOrDefault(thresholds map[string]float64, key string, fallback float64) float64 {
	if value := thresholds[key]; value > 0 {
		return value
	}
	return fallback
}

func formatAlertSettingNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func newAlertSettingsModel(draft alertSettingsDraft) alertSettingsModel {
	items := make([]alertSettingsItem, 0, len(alertSettingsRows)+1)
	for i, row := range alertSettingsRows {
		items = append(items, alertSettingsItem{key: row.key, label: row.label, value: draft.values[row.key], cursor: i == 0})
	}
	items = append(items, alertSettingsItem{key: "confirm", label: "Confirm", cursor: false, confirm: true})
	return alertSettingsModel{items: items}
}

func (m alertSettingsModel) Init() tea.Cmd { return nil }

func (m alertSettingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.applyWindowSize(msg.Height)
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "ctrl+q" {
			m.cancelled = true
			m.done = true
			return m, tea.Quit
		}
		if m.editing {
			// Plain "q" must reach updateEdit so values like timezones can
			// contain it; cancellation while editing stays on the ctrl keys.
			return m.updateEdit(msg)
		}
		if msg.String() == "q" {
			m.cancelled = true
			m.done = true
			return m, tea.Quit
		}
		switch msg.String() {
		case "up":
			m.move(-1)
		case "down":
			m.move(1)
		case "enter", " ":
			item := &m.items[m.index()]
			if item.confirm {
				m.done = true
				return m, tea.Quit
			}
			if item.key == "enabled" {
				item.value = strconv.FormatBool(item.value != "true")
			} else {
				m.editing = true
				m.editingValue = item.value
				m.validationError = ""
			}
		}
	}
	return m, nil
}

func (m alertSettingsModel) updateEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.editing = false
		m.validationError = ""
	case "enter":
		item := &m.items[m.index()]
		if err := validateAlertSettingsValue(item.key, m.editingValue); err != nil {
			m.validationError = err.Error()
			return m, nil
		}
		item.value = strings.TrimSpace(m.editingValue)
		m.editing = false
		m.validationError = ""
	case "backspace", "delete":
		if length := len(m.editingValue); length > 0 {
			m.editingValue = m.editingValue[:length-1]
		}
	default:
		if len(msg.Runes) > 0 {
			m.editingValue += string(msg.Runes)
		}
	}
	return m, nil
}

func (m *alertSettingsModel) move(delta int) {
	index := m.index()
	m.items[index].cursor = false
	next := (index + delta + len(m.items)) % len(m.items)
	m.items[next].cursor = true
	m.clampOffset()
}

func (m alertSettingsModel) index() int {
	for i, item := range m.items {
		if item.cursor {
			return i
		}
	}
	return 0
}

func (m *alertSettingsModel) applyWindowSize(height int) {
	if height <= 0 {
		return
	}
	m.viewHeight = height
	m.visibleRows = max(1, height-alertSettingsViewChrome-2)
	m.clampOffset()
}

func (m *alertSettingsModel) clampOffset() {
	if m.visibleRows == 0 || m.viewHeight == 0 {
		return
	}
	index := m.index()
	if index < m.offset {
		m.offset = index
	}
	if index >= m.offset+m.visibleRows {
		m.offset = index - m.visibleRows + 1
	}
	m.offset = max(0, min(m.offset, len(m.items)-m.visibleRows))
}

func (m alertSettingsModel) View() string {
	from, to := 0, len(m.items)
	if m.viewHeight > 0 {
		from, to = m.offset, min(len(m.items), m.offset+m.visibleRows)
	}
	lines := []string{"Alert settings:"}
	if from > 0 {
		lines = append(lines, fmt.Sprintf("  ^ %d more", from))
	}
	for _, item := range m.items[from:to] {
		marker, value := " ", item.value
		if item.cursor {
			marker = ">"
		}
		if item.confirm {
			lines = append(lines, marker+" Confirm")
			continue
		}
		if item.cursor && m.editing {
			value = m.editingValue + "_"
		}
		lines = append(lines, fmt.Sprintf("%s %s: %s", marker, item.label, value))
	}
	if more := len(m.items) - to; more > 0 {
		lines = append(lines, fmt.Sprintf("  v %d more", more))
	}
	if m.validationError != "" {
		lines = append(lines, "Invalid: "+m.validationError)
	} else if m.editing {
		lines = append(lines, "Enter accepts edit, Esc keeps current value")
	} else {
		lines = append(lines, "Up/Down move, Enter/Space edit, q cancels")
	}
	for m.viewHeight > 0 && len(lines) > m.viewHeight {
		lines = append(lines[:1], lines[2:]...)
	}
	return strings.Join(lines, "\n") + "\n"
}

func validateAlertSettingsValue(key, value string) error {
	switch key {
	case "enabled":
		_, err := parseAlertBool(value)
		return err
	case "cooldown_minutes":
		minutes, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || minutes <= 0 {
			return fmt.Errorf("invalid cooldown_minutes %q: must be a positive integer", value)
		}
		return nil
	case "threshold_percent", "critical_percent", "threshold.default", "threshold.5h", "threshold.7d":
		_, err := parseAlertPercent(strings.ReplaceAll(key, ".", " "), value)
		return err
	case "quiet_hours":
		return validateAlertQuietHours(value)
	case "timezone":
		if strings.TrimSpace(value) == "" {
			return nil
		}
		_, err := time.LoadLocation(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("invalid timezone %q: %w", value, err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported alert settings key %q", key)
	}
}

func applyAlertSettingsDraft(draft alertSettingsDraft) error {
	for _, row := range alertSettingsRows {
		if err := validateAlertSettingsValue(row.key, draft.values[row.key]); err != nil {
			return err
		}
	}
	for _, row := range alertSettingsRows {
		if err := setAlertConfigValue(row.key, draft.values[row.key]); err != nil {
			return fmt.Errorf("apply %s: %w", row.key, err)
		}
	}
	if err := persistViperConfig(); err != nil {
		return fmt.Errorf("persist alert settings: %w", err)
	}
	return nil
}

func runInteractiveAlert(input io.Reader, output io.Writer) error {
	program := tea.NewProgram(newAlertSettingsModel(alertSettingsDraftFromViper()), tea.WithAltScreen(), tea.WithInput(input), tea.WithOutput(output))
	finalModel, err := program.Run()
	if err != nil {
		return err
	}
	model, ok := finalModel.(alertSettingsModel)
	if !ok {
		return fmt.Errorf("unexpected alert settings model type")
	}
	return applyCompletedAlertSettingsModel(model)
}

func applyCompletedAlertSettingsModel(model alertSettingsModel) error {
	if model.cancelled || !model.done {
		return nil
	}
	draft := alertSettingsDraft{values: make(map[string]string, len(alertSettingsRows))}
	for _, item := range model.items {
		if !item.confirm {
			draft.values[item.key] = item.value
		}
	}
	return applyAlertSettingsDraft(draft)
}
