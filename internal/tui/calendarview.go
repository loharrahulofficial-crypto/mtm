package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
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

// calendarCellStyle picks the style for one day cell. Today keeps its own
// filled-pill style even when the calendar cursor is on it; the selected day
// gets a raised fill with accent-bold numerals (the pill still wins).
func calendarCellStyle(m Model, day time.Time) lipgloss.Style {
	s := m.styles
	today := m.clock.Today()
	d := dayStartT(day)
	isToday := d.Equal(today)
	isSelected := d.Equal(dayStartT(m.calDay))
	isThisMonth := day.Month() == m.calMonth.Month()
	_, _, anyOverdue, _ := dotsForDay(m, day)
	switch {
	case isToday:
		return s.dueToday // today keeps its filled pill even when selected
	case isSelected:
		return s.selected
	case !isThisMonth:
		return s.dim
	case day.Weekday() == time.Saturday || day.Weekday() == time.Sunday:
		return s.weekend
	case anyOverdue:
		return s.overdue
	default:
		return s.base
	}
}

// dotsForDay reports dots info for a day: course-colored dots for open
// tasks due that day, whether any is overdue, and whether an exam is due.
func dotsForDay(m Model, day time.Time) (colors []string, total int, anyOverdue bool, exam bool) {
	isDone := func(p, st string) bool { return m.store.IsDoneStatus(p, st) }
	now := m.clock.Now()
	dayStr := day.Format("2006-01-02")
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
		if isDone(t.Project, t.Status) {
			continue
		}
		total++
		parsed, err := core.ParseDue(t.Due, day.Location())
		if err == nil && parsed.Before(now) {
			anyOverdue = true
		}
		if strings.EqualFold(t.Kind, "exam") {
			exam = true
		}
		if len(colors) < 3 {
			colors = append(colors, theme.CourseColor(m.palette, t.Project, projectColorOverride(m, t.Project)))
		}
	}
	return colors, total, anyOverdue, exam
}

// dayStartT is midnight of t in its own zone.
func dayStartT(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func weekdayHeaders(weekStart string) []string {
	short := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	ws := strings.ToLower(strings.TrimSpace(weekStart))
	if ws == "sunday" {
		short = []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	}
	return short
}

// renderCalendarBody renders the calendar content (inner width × inner height).
func renderCalendarBody(m Model, w, h int) []string {
	s := m.styles
	now := m.clock.Now()
	_ = now
	today := m.clock.Today()

	bgSurface := lipgloss.Color(m.palette.Surface)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bgSurface)

	if w < 28 {
		return []string{s.muted.Render("terminal too narrow")}
	}
	colW := w / 7

	lines := []string{}

	// Title: month year · right-aligned ‹ › hints.
	title := strings.ToUpper(m.calMonth.Format("January 2006"))
	hints := m.icon("chev_left") + " " + m.icon("chev_right")
	hintW := lipglossWidth(hints)
	titleW := w - hintW - 2
	cap_ := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.BrightText)).Background(bgSurface).Bold(true).Render(title)
	titleLine := " " + cap_ + strings.Repeat(" ", max(0, titleW-lipglossWidth(cap_))) + " " + muted.Render(hints)
	lines = append(lines, padToWidth(titleLine, w, bgSurface))

	// Weekday caps.
	weekday := weekdayHeaders(m.cfg.WeekStart)
	weekNames := []string{}
	for _, d := range weekday {
		weekNames = append(weekNames, strings.ToUpper(d))
	}
	if colW < 5 {
		weekNames = []string{"M", "T", "W", "T", "F", "S", "S"}
		if strings.ToLower(m.cfg.WeekStart) == "sunday" {
			weekNames = []string{"S", "M", "T", "W", "T", "F", "S"}
		}
	}
	wkLine := ""
	for _, name := range weekNames {
		wkLine += muted.Width(colW).Render(center(name, colW))
	}
	lines = append(lines, wkLine)

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

	weeks := 6
	// Skip the 6th row when nothing of this month lands in it.
	sixth := gridStart.AddDate(0, 0, 35)
	if sixth.Month() != m.calMonth.Month() && sixth.AddDate(0, 0, 6).Month() != m.calMonth.Month() {
		weeks = 5
	}

	gridTop := len(lines)
	for week := 0; week < weeks; week++ {
		weekStart := gridStart.AddDate(0, 0, week*7)
		cells := make([]string, 0, 7)
		for d := 0; d < 7; d++ {
			day := weekStart.AddDate(0, 0, d)
			cells = append(cells, calendarCell(m, day, colW, today))
		}
		lines = append(lines, strings.Join(cells, ""))
	}

	// Agenda.
	lines = append(lines, "")
	agendaHead := dayStartT(m.calDay).Format("January 2, 2006")
	if dayStartT(m.calDay).Equal(today) {
		agendaHead = "TODAY · " + agendaHead
	}
	agCap := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(bgSurface).Bold(true).Render(strings.ToUpper(agendaHead))
	lines = append(lines, " "+agCap)

	agenda := agendaTasks(m)
	isDone := func(p, st string) bool { return m.store.IsDoneStatus(p, st) }
	agendaH := h - len(lines) - 1
	if agendaH < 1 {
		agendaH = 1
	}
	bgRaised := lipgloss.Color(m.palette.SurfaceRaised)
	for i, t := range agenda {
		if i >= agendaH {
			lines = append(lines, s.muted.Render(fmt.Sprintf("  +%d more", len(agenda)-i)))
			break
		}
		done := isDone(t.Project, t.Status)
		timeStr := ""
		if len(t.Due) > len("2006-01-02") {
			timeStr = t.Due[len("2006-01-02")+1:]
			if len(timeStr) > 5 {
				timeStr = timeStr[:5]
			}
			timeStr += " "
		} else {
			timeStr = "     "
		}
		mark := "○"
		if done {
			mark = "✔"
		}
		color := theme.CourseColor(m.palette, t.Project, projectColorOverride(m, t.Project))
		stripe := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(bgSurface).Render("▌")
		if i == m.calAgendaSel && m.focus != focusSidebar {
			selLine := strings.Repeat(" ", 1) + timeStr + stripe + " " + mark + " " + truncatePlain(t.Title, w-14)
			lines = append(lines, padToWidth(selLine, w, bgRaised))
			continue
		}
		textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(bgSurface)
		if done {
			textStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bgSurface).Strikethrough(true)
		}
		line := " " + muted.Render(timeStr) + stripe + " " + textStyle.Render(mark+" "+truncatePlain(t.Title, w-14))
		lines = append(lines, padToWidth(line, w, bgSurface))
	}
	if len(agenda) == 0 {
		lines = append(lines, s.muted.Render(" "+m.icon("check")+" nothing due — a clear day"))
	}

	_ = gridTop
	return lines
}

// calendarCell renders one fixed-width day cell.
func calendarCell(m Model, day time.Time, colW int, today time.Time) string {
	todayCell := dayStartT(day).Equal(today)
	isSelected := dayStartT(day).Equal(dayStartT(m.calDay))
	isThisMonth := day.Month() == m.calMonth.Month()
	isWeekend := day.Weekday() == time.Saturday || day.Weekday() == time.Sunday

	bg := lipgloss.Color(m.palette.Surface)
	if isSelected {
		bg = lipgloss.Color(m.palette.SurfaceRaised)
	}

	numStr := fmt.Sprintf("%2d", day.Day())
	numStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(bg)
	switch {
	case todayCell:
		numStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Base)).Background(lipgloss.Color(m.palette.Accent)).Bold(true)
	case isSelected:
		numStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(bg).Bold(true)
	case !isThisMonth || isWeekend:
		numStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bg)
	default:
	}

	_, total, anyOverdue, exam := dotsForDay(m, day)
	num := numStyle.Render(numStr)
	if anyOverdue && !todayCell && !isSelected {
		num = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.HeatOverdue)).Background(bg).Bold(true).Render(numStr)
	}

	suffix := ""
	colors, _, _, _ := dotsForDay(m, day)
	for _, c := range colors {
		suffix += lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Background(bg).Render("●")
	}
	if total > 3 {
		suffix += lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bg).Render(fmt.Sprintf("+%d", total-3))
	}
	if exam {
		suffix += lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(bg).Bold(true).Render("✦")
	}

	content := " " + num + " " + suffix
	return lipgloss.NewStyle().Background(bg).Width(colW).Render(content)
}

// clickCalendar selects the clicked day cell or an agenda line.
func (m *Model) clickCalendar(x, y int) {
	// Content starts at body line 2 (title + weekday). Grid lines follow.
	if m.width < 28 {
		return
	}
	colW := (m.width - 2) / 7
	gridTop := 2

	weekday := weekdayHeaders(m.cfg.WeekStart)
	_ = weekday
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
	weeks := 6
	sixth := gridStart.AddDate(0, 0, 35)
	if sixth.Month() != m.calMonth.Month() && sixth.AddDate(0, 0, 6).Month() != m.calMonth.Month() {
		weeks = 5
	}

	// Account for the panel pad row: mouse y is absolute; body starts at y=1.
	const padTop = 1
	gy := y - padTop - gridTop
	if gy >= 0 && gy < weeks {
		col := (x - 1) / colW // body inset 1
		if col > 6 {
			col = 6
		}
		if col < 0 {
			col = 0
		}
		m.calDay = gridStart.AddDate(0, 0, gy*7+col)
		m.calMonth = time.Date(m.calDay.Year(), m.calDay.Month(), 1, 0, 0, 0, 0, m.calDay.Location())
		m.calAgendaSel = 0
		return
	}
	// Agenda selection.
	agendaTop := padTop + gridTop + weeks + 2 // blank + agenda header
	idx := y - agendaTop
	if idx >= 0 && idx < len(agendaTasks(*m)) {
		m.calAgendaSel = idx
	}
}
