package delete

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Soeky/pomo/internal/db"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	maxRecent     = 50
	visibleWindow = 10
)

// step states
const (
	stepSelectCount = iota
	stepSelectItems
	stepConfirm
)

// item represents a session entry
type item struct {
	ID        int
	Topic     string
	StartTime time.Time
	Duration  int // seconds
}

// model defines Bubble Tea model
type model struct {
	step       int
	countInput string
	items      []item
	cursor     int
	selected   map[int]struct{}
}

// NewModel creates the delete TUI
func NewModel() model {
	return model{
		step:       stepSelectCount,
		countInput: "",
		items:      nil,
		cursor:     0,
		selected:   make(map[int]struct{}),
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

// Update handles messages, with global quit on 'q' or Ctrl+C
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "q" || keyMsg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.step {
		case stepSelectCount:
			if msg.String() == "enter" {
				c, err := strconv.Atoi(m.countInput)
				if err != nil || c <= 0 || c > maxRecent {
					m.countInput = ""
					return m, nil
				}
				items, _ := getRecentSessions(c)
				m.items = items
				if m.cursor >= len(items) {
					m.cursor = len(items) - 1
				}
				m.step = stepSelectItems
				return m, nil
			}
			if strings.ContainsAny(msg.String(), "0123456789") {
				m.countInput += msg.String()
			}

		case stepSelectItems:
			switch msg.String() {
			case "up":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down":
				if m.cursor < len(m.items)-1 {
					m.cursor++
				}
			case " ", "enter": // toggle select
				if _, ok := m.selected[m.cursor]; ok {
					delete(m.selected, m.cursor)
				} else {
					m.selected[m.cursor] = struct{}{}
				}
			case "c": // confirm selection
				if len(m.selected) > 0 {
					m.step = stepConfirm
				}
			}

		case stepConfirm:
			switch msg.String() {
			case "y", "Y":
				for idx := range m.selected {
					deleteSessionByID(m.items[idx].ID)
				}
				return m, tea.Quit
			case "n", "N":
				m.step = stepSelectItems
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	switch m.step {
	case stepSelectCount:
		return fmt.Sprintf("Load up to how many recent sessions (1-%d): %s", maxRecent, m.countInput)
	case stepSelectItems:
		var b strings.Builder
		b.WriteString("Select sessions to delete (space/enter to toggle, 'c' to confirm, 'q' to quit):\n")

		start := max(m.cursor-visibleWindow/2, 0)
		end := start + visibleWindow
		if end > len(m.items) {
			end = len(m.items)
			start = max(end-visibleWindow, 0)
		}

		for i := start; i < end; i++ {
			it := m.items[i]
			cursor := " "
			if m.cursor == i {
				cursor = ">"
			}
			checked := " "
			if _, ok := m.selected[i]; ok {
				checked = "x"
			}
			b.WriteString(fmt.Sprintf("%s [%s] %-12s %6s  %s\n",
				cursor, checked,
				it.Topic,
				formatDuration(it.Duration),
				it.StartTime.Format("2006-01-02 15:04")))
		}
		return b.String()
	case stepConfirm:
		var b strings.Builder
		b.WriteString(fmt.Sprintf("You selected %d sessions to delete:\n", len(m.selected)))
		for idx := range m.selected {
			it := m.items[idx]
			b.WriteString(fmt.Sprintf("- [%3d] %-12s %6s  %s\n", it.ID, it.Topic, formatDuration(it.Duration), it.StartTime.Format("2006-01-02 15:04")))
		}
		b.WriteString("\nConfirm deletion? (y/n, 'q' to cancel)")
		return b.String()
	}
	return ""
}

// StartDelete launches the Bubble Tea program
func StartDelete() {
	db.InitDB()
	program := tea.NewProgram(NewModel())
	_, err := program.Run()
	if err != nil {
		fmt.Printf("Error: %v", err)
	}
}

// formatDuration formats seconds into HH:MM
func formatDuration(sec int) string {
	d := time.Duration(sec) * time.Second
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%02d:%02d", h, m)
}

// getRecentSessions fetches recent sessions from DB
func getRecentSessions(limit int) ([]item, error) {
	rows, err := db.DB.Query(`
		SELECT id, topic, start_time, duration
		FROM sessions
		ORDER BY start_time DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []item
	for rows.Next() {
		var it item
		var durationSec sql.NullInt64
		err := rows.Scan(&it.ID, &it.Topic, &it.StartTime, &durationSec)
		if err != nil {
			continue
		}
		if it.Topic == "" {
			it.Topic = "break"
		}
		if durationSec.Valid {
			it.Duration = int(durationSec.Int64)
		}
		items = append(items, it)
	}
	return items, nil
}

// deleteSessionByID removes a session row by ID
func deleteSessionByID(id int) error {
	_, err := db.DB.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}
