package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type menuItem struct {
	Label       string
	Description string
}

type menuModel struct {
	Title  string
	Items  []menuItem
	Cursor int
}

func (m *menuModel) move(delta int) {
	if len(m.Items) == 0 {
		m.Cursor = 0
		return
	}
	m.Cursor += delta
	if m.Cursor < 0 {
		m.Cursor = len(m.Items) - 1
	}
	if m.Cursor >= len(m.Items) {
		m.Cursor = 0
	}
}

func (m menuModel) selectedIndex() int {
	if len(m.Items) == 0 {
		return -1
	}
	if m.Cursor < 0 {
		return 0
	}
	if m.Cursor >= len(m.Items) {
		return len(m.Items) - 1
	}
	return m.Cursor
}

func (m menuModel) View() string {
	var b strings.Builder
	if strings.TrimSpace(m.Title) != "" {
		b.WriteString(m.Title)
		b.WriteString("\n")
	}
	for i, item := range m.Items {
		cursor := " "
		if i == m.selectedIndex() {
			cursor = ">"
		}
		if strings.TrimSpace(item.Description) == "" {
			b.WriteString(fmt.Sprintf("%s %s\n", cursor, item.Label))
			continue
		}
		b.WriteString(fmt.Sprintf("%s %s - %s\n", cursor, item.Label, item.Description))
	}
	return strings.TrimRight(b.String(), "\n")
}

type formField struct {
	Key         string
	Label       string
	Value       string
	Placeholder string
}

type formModel struct {
	Title       string
	Fields      []formField
	Cursor      int
	SubmitLabel string
}

func (f *formModel) move(delta int) {
	if len(f.Fields) == 0 {
		f.Cursor = 0
		return
	}
	f.Cursor += delta
	if f.Cursor < 0 {
		f.Cursor = len(f.Fields) - 1
	}
	if f.Cursor >= len(f.Fields) {
		f.Cursor = 0
	}
}

func (f *formModel) currentField() *formField {
	if len(f.Fields) == 0 {
		return nil
	}
	if f.Cursor < 0 {
		f.Cursor = 0
	}
	if f.Cursor >= len(f.Fields) {
		f.Cursor = len(f.Fields) - 1
	}
	return &f.Fields[f.Cursor]
}

func (f *formModel) values() map[string]string {
	out := make(map[string]string, len(f.Fields))
	for _, field := range f.Fields {
		out[field.Key] = strings.TrimSpace(field.Value)
	}
	return out
}

func (f *formModel) Update(msg tea.KeyMsg) (submitted bool, canceled bool) {
	switch msg.String() {
	case "esc":
		return false, true
	case "up", "shift+tab":
		f.move(-1)
		return false, false
	case "down", "tab":
		f.move(1)
		return false, false
	case "enter":
		if len(f.Fields) == 0 {
			return false, false
		}
		if f.Cursor == len(f.Fields)-1 {
			return true, false
		}
		f.move(1)
		return false, false
	case "backspace", "ctrl+h":
		field := f.currentField()
		if field == nil || field.Value == "" {
			return false, false
		}
		r := []rune(field.Value)
		field.Value = string(r[:len(r)-1])
		return false, false
	default:
		field := f.currentField()
		if field == nil {
			return false, false
		}
		if len(msg.Runes) > 0 {
			field.Value += string(msg.Runes)
		}
		return false, false
	}
}

func (f formModel) View() string {
	var b strings.Builder
	if strings.TrimSpace(f.Title) != "" {
		b.WriteString(f.Title)
		b.WriteString("\n")
	}
	for i, field := range f.Fields {
		cursor := " "
		if i == f.Cursor {
			cursor = ">"
		}
		value := field.Value
		if strings.TrimSpace(value) == "" {
			value = field.Placeholder
		}
		b.WriteString(fmt.Sprintf("%s %-20s %s\n", cursor, field.Label+":", value))
	}
	submit := f.SubmitLabel
	if strings.TrimSpace(submit) == "" {
		submit = "Enter on the last field to submit"
	}
	b.WriteString("\n")
	b.WriteString(submit)
	b.WriteString(" | Esc cancel | Tab/Shift+Tab move | Type to edit")
	return strings.TrimRight(b.String(), "\n")
}

func isQuitKey(msg tea.KeyMsg) bool {
	return msg.Type == tea.KeyCtrlC || msg.String() == "q"
}
