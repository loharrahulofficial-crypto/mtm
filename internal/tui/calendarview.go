package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

func (m Model) updateCalendarKey(act string) (tea.Model, tea.Cmd) {
	switch act {
	case aUp:
		m.calAgendaSel -= 1
	case aDown:
		m.calAgendaSel += 1
	case aLeft:
		m.moveCalDay(-1)
	case aRight:
		m.moveCalDay(+1)
	case aJumpLeft:
		m.calMonth = m.calMonth.AddDate(0, -1, 0)
		m.calDay = clampCalDay(m.calMonth, m.calDay)
	case aJumpRight:
		m.calMonth = m.calMonth.AddDate(0, +1, 0)
		m.calDay = clampCalDay(m.calMonth, m.calDay)
	case aTop:
		m.calAgendaSel = 0
	default:
		if m2, cmd, handled := m.updateSimpleTaskAction(act); handled {
			return m2, cmd
		}
	}
	maxSel := len(agendaTasks(m)) - 1
	if m.calAgendaSel > maxSel {
		m.calAgendaSel = maxSel
	}
	if m.calAgendaSel < 0 {
		m.calAgendaSel = 0
	}
	return m, nil
}

func clampCalDay(month, day time.Time) time.Time {
	last := month.AddDate(0, 1, -1).Day()
	d := day.Day()
	if d > last {
		d = last
	}
	return time.Date(month.Year(), month.Month(), d, 0, 0, 0, 0, month.Location())
}

func (m *Model) moveCalDay(delta int) {
	m.calDay = m.calDay.AddDate(0, 0, delta)
	if m.calDay.Year() != m.calMonth.Year() || m.calDay.Month() != m.calMonth.Month() {
		m.calMonth = time.Date(m.calDay.Year(), m.calDay.Month(), 1, 0, 0, 0, 0, m.calDay.Location())
	}
	m.calAgendaSel = 0
}

// agendaTasks returns tasks due on the selected day, sorted; done shown dimmed.
func agendaTasks(m Model) []*core.Task {
	isDone := func(p, st string) bool { return m.store.IsDoneStatus(p, st) }
	dayStr := m.calDay.Format("2006-01-02")
	var out []*core.Task
	for _, t := range m.store.Tasks() {
		if t.Due == "" {
			continue
		}
		d := t.Due
		if len(d) > len("2006-01-02") {
			d = d[:len("2006-01-02")]
		}
		if d != dayStr {
			continue
		}
		if m.needle != "" {
			hay := strings.ToLower(strings.Join([]string{t.Title, t.Body, strings.Join(t.Tags, " ")}, "\n"))
			if !strings.Contains(hay, strings.ToLower(m.needle)) {
				continue
			}
		}
		out = append(out, t)
	}
	core.SortTasks(out, m.cfg.Sort, isDone)
	return out
}

// dotCountForDay reports how many tasks (not done) are due on day.
func dotCountForDay(m Model, day time.Time) (n int, anyOverdue bool) {
	isDone := func(p, st string) bool { return m.store.IsDoneStatus(p, st) }
	now := time.Now()
	for _, t := range m.store.Tasks() {
		if t.Due == "" {
			continue
		}
		if isDone(t.Project, t.Status) {
			continue
		}
		d, err := core.ParseDue(t.Due, day.Location())
		if err != nil {
			continue
		}
		if d.Year() == day.Year() && d.Month() == day.Month() && d.Day() == day.Day() {
			n++
			if d.Before(now) {
				anyOverdue = true
			}
		}
	}
	return n, anyOverdue
}

func weekdayHeaders(weekStart string) []string {
	short := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	ws := strings.ToLower(strings.TrimSpace(weekStart))
	if ws == "sunday" {
		short = []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	}
	return short
}

func renderCalendar(m Model, w, h int) string {
	s := m.styles
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var b strings.Builder

	// Month title.
	title := fmt.Sprintf("  %s", m.calMonth.Format("January 2006"))
	b.WriteString(s.accent.Width(w).Render(truncatePlain(title, w)) + "\n")

	weekday := weekdayHeaders(m.cfg.WeekStart)
	colW := w / 7
	if colW < 4 {
		colW = 4
	}
	for i := range weekday {
		b.WriteString(s.dim.Width(colW).Render(center(weekday[i], colW)))
	}
	b.WriteString("\n")

	// Grid: find first cell.
	first := m.calMonth
	wd := int(first.Weekday())
	offset := wd - 1 // monday-first
	if strings.ToLower(m.cfg.WeekStart) == "sunday" {
		offset = wd
	}
	if offset < 0 {
		offset += 7
	}
	gridStart := first.AddDate(0, 0, -offset)
	last := m.calMonth.AddDate(0, 1, -1)

	cell := func(day time.Time) string {
		isThisMonth := day.Month() == m.calMonth.Month()
		isToday := day.Equal(today)
		isSelected := day.Equal(m.calDay)
		nDots, anyOverdue := dotCountForDay(m, day)

		dayNum := fmt.Sprintf("%2d", day.Day())
		dots := ""
		for i := 0; i < nDots && i < 3; i++ {
			dots += "•"
		}
		if nDots > 3 {
			dots += "+"
		}
		line := dayNum + " " + dots

		var style lipgloss.Style
		switch {
		case isSelected:
			style = s.selected
		case isToday:
			style = s.dueToday
		case !isThisMonth:
			style = s.dim
		case anyOverdue:
			style = s.overdue
		default:
			style = s.base
		}
		_ = last
		return style.Width(colW).Render(line)
	}

	for week := 0; week < 6; week++ {
		weekStart := gridStart.AddDate(0, 0, week*7)
		if week == 5 && weekStart.Month() != m.calMonth.Month() && weekStart.AddDate(0, 0, 6).Month() != m.calMonth.Month() {
			break // nothing of this month in the 6th row
		}
		for d := 0; d < 7; d++ {
			b.WriteString(cell(weekStart.AddDate(0, 0, d)))
		}
		b.WriteString("\n")
	}

	// Agenda.
	b.WriteString(s.dim.Render(strings.Repeat("─", w)) + "\n")
	agendaTitle := fmt.Sprintf(" %s", m.calDay.Format("Monday, 2006-01-02"))
	b.WriteString(s.paneTitle.Width(w).Render(truncatePlain(agendaTitle, w)) + "\n")

	agenda := agendaTasks(m)
	isDone := func(p, st string) bool { return m.store.IsDoneStatus(p, st) }
	agendaH := h - strings.Count(b.String(), "\n") - 2
	if agendaH < 1 {
		agendaH = 1
	}
	for i, t := range agenda {
		if i >= agendaH {
			b.WriteString(s.dim.Render(fmt.Sprintf("  +%d more", len(agenda)-i)) + "\n")
			break
		}
		done := isDone(t.Project, t.Status)
		mark := "○"
		if done {
			mark = "✔"
		}
		prio := priorityPlain(t.Priority)
		line := fmt.Sprintf(" %s %s %s", mark, prio, truncatePlain(t.Title, w-20))
		if i == m.calAgendaSel && m.focus != focusSidebar {
			b.WriteString(s.selected.Width(w).Render(line) + "\n")
			continue
		}
		if done {
			b.WriteString(s.done.Width(w).Render(line) + "\n")
			continue
		}
		b.WriteString(s.base.Width(w).Render(line) + "\n")
	}
	if len(agenda) == 0 {
		b.WriteString(s.dim.Render("  nothing due — a clear day") + "\n")
	}
	for strings.Count(b.String(), "\n") < h-1 {
		b.WriteString(strings.Repeat(" ", w) + "\n")
	}
	b.WriteString(s.dim.Render("  h/l days · H/L months · j/k agenda · x done"))
	return strings.TrimSuffix(b.String(), "\n")
}

// clickCalendar selects the clicked day cell or an agenda line.
func (m *Model) clickCalendar(x, y int) {
	// Calendar body starts after title + weekday rows.
	gridTop := 2
	colW := m.width / 7
	if colW < 4 {
		colW = 4
	}
	if y >= gridTop && y < gridTop+6 {
		col := x / colW
		if col > 6 {
			col = 6
		}
		first := m.calMonth
		wd := int(first.Weekday())
		offset := wd - 1
		if strings.ToLower(m.cfg.WeekStart) == "sunday" {
			offset = wd
		}
		if offset < 0 {
			offset += 7
		}
		gridStart := first.AddDate(0, 0, -offset)
		m.calDay = gridStart.AddDate(0, 0, (y-gridTop)*7+col)
		m.calMonth = time.Date(m.calDay.Year(), m.calDay.Month(), 1, 0, 0, 0, 0, m.calDay.Location())
		m.calAgendaSel = 0
		return
	}
	// Agenda selection.
	agendaTop := gridTop + 8 // approx: grid + separator + agenda title
	idx := y - agendaTop
	if idx >= 0 && idx < len(agendaTasks(*m)) {
		m.calAgendaSel = idx
	}
}
