package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/dates"
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

// calendarCell renders one day cell as cellH lines of exactly cw cells.
// Block on surface with the day number top-left; out-of-month/weekend days
// are dimmed; today wears an accent pill on an accent-tinted cell; the
// selected day sits on raised with an accent underline. Task dots and the
// day's dot overflow land on the bottom-left; wide-enough/tall-enough cells
// also show up to two truncated task titles.
func calendarCell(m Model, day time.Time, cw, cellH int, today time.Time) []string {
	todayCell := dayStartT(day).Equal(today)
	isSelected := dayStartT(day).Equal(dayStartT(m.calDay))

	bg := lipgloss.Color(m.palette.Surface)
	if todayCell {
		// Faint accent tint (12% of accent over surface).
		tint := theme.Blend(m.palette.Surface, m.palette.Accent, 0.12)
		bg = lipgloss.Color(tint)
	}
	if isSelected {
		bg = lipgloss.Color(m.palette.SurfaceRaised)
	}
	fill := lipgloss.NewStyle().Background(bg)
	gitRun := func(row string) string {
		if pad := cw - lipgloss.Width(row); pad > 0 {
			row += fill.Render(strings.Repeat(" ", pad))
		}
		// A row wider than cw is clipped by the panel.
		return row
	}

	// Row 0: day number (pill for today, bold accent when cursor is on it).
	numStr := fmt.Sprintf("%d", day.Day())
	st := calendarCellStyle(m, day)
	switch {
	case todayCell:
		numStr = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Base)).Background(lipgloss.Color(m.palette.Accent)).Bold(true).Render(" " + numStr + " ")
	case isSelected:
		numStr = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(bg).Bold(true).Render(numStr)
	default:
		numStr = lipgloss.NewStyle().Foreground(st.GetForeground()).Background(bg).Bold(st.GetBold()).Render(numStr)
	}

	row0 := fill.Render(" ") + numStr
	out := []string{gitRun(row0)}
	for i := 1; i < cellH; i++ {
		out = append(out, gitRun(""))
	}

	// Task titles (only when the cell can breathe).
	if cellH >= 5 && cw >= 14 {
		tasks := tasksDueOn(m, day)
		maxTitles := 2
		if cellH-2 < maxTitles {
			maxTitles = cellH - 2
		}
		for i := 0; i < len(tasks) && i < maxTitles; i++ {
			t := tasks[i]
			done := m.store.IsDoneStatus(t.Project, t.Status)
			ts := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(bg)
			title := truncatePlain(t.Title, cw-2)
			if done {
				out[1+i] = gitRun(fill.Render(" ") + strikeTitle(lipgloss.Color(m.palette.Muted), bg, title))
				continue
			}
			out[1+i] = gitRun(fill.Render(" ") + ts.Render(title))
		}
	}

	// Dots row (bottom-left), with +N overflow and an exam marker.
	colors, total, _, exam := dotsForDay(m, day)
	dotsRow := fill.Render(" ")
	any := false
	for _, c := range colors {
		dotsRow += lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Background(bg).Render("●")
		any = true
	}
	if total > len(colors) {
		dotsRow += lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bg).Render(fmt.Sprintf("+%d", total-len(colors)))
		any = true
	}
	if exam {
		dotsRow += lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(bg).Bold(true).Render("✦")
		any = true
	}
	dotsLine := any

	// Bottom row: selected → accent underline rule; else dots.
	last := cellH - 1
	if isSelected {
		out[last] = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(bg).Render(strings.Repeat("─", cw))
		if dotsLine && last-1 >= 1 {
			out[last-1] = gitRun(dotsRow)
		}
		return out
	}
	if dotsLine {
		out[last] = gitRun(dotsRow)
	}
	return out
}

// tasksDueOn returns tasks due on day, sorted like the agenda.
func tasksDueOn(m Model, day time.Time) []*core.Task {
	saved := m.calDay
	m.calDay = day
	out := agendaTasks(m)
	m.calDay = saved
	return out
}

// calendarCellStyle picks the base style for one day's number. Today keeps
// its own filled-pill style even when the cursor is on it; the selected day
// gets the accent treatment (the pill still wins). Used for the dimmed/hot
// precedence: out-of-month → weekend → overdue → plain.
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

func center(s string, w int) string {
	sd := lipgloss.Width(s)
	if sd >= w {
		return clipStyled(s, w)
	}
	left := (w - sd) / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", w-left-sd)
}

// calendarGrid computes the grid start day and the number of weeks to show.
func calendarGrid(m Model) (gridStart time.Time, weeks int) {
	first := m.calMonth
	wd := int(first.Weekday())
	offset := wd - 1 // monday-first
	if strings.ToLower(m.cfg.WeekStart) == "sunday" {
		offset = wd
	}
	if offset < 0 {
		offset += 7
	}
	gridStart = first.AddDate(0, 0, -offset)

	weeks = 6
	// Skip the 6th row when nothing of this month lands in it.
	sixth := gridStart.AddDate(0, 0, 35)
	if sixth.Month() != m.calMonth.Month() && sixth.AddDate(0, 0, 6).Month() != m.calMonth.Month() {
		weeks = 5
	}
	return gridStart, weeks
}

// calendarLayout computes the cell width and grid height for the current
// content box: cw is the single cell width shared by weekday headers and
// day cells; cells are separated by 1-cell base gutters.
func calendarLayout(m Model, w, h int) (cw, cellH, weeks int, gridStart time.Time) {
	const gap = 1
	cw = (w - 6*gap) / 7
	if cw < 4 {
		cw = 4
	}
	gridStart, weeks = calendarGrid(m)

	agendaRows := 2 // header + at least one line (empty state)
	if n := len(agendaTasks(m)); n > 0 {
		agendaRows = 1 + n
	}
	if maxAgenda := h / 2; agendaRows > maxAgenda {
		agendaRows = maxAgenda
	}
	// Reserve: title(1) + weekday(1) + blank before agenda(1) + agenda rows.
	gridAvail := h - 3 - agendaRows
	cellH = gridAvail / weeks
	if cellH < 3 {
		cellH = 3
	}
	return cw, cellH, weeks, gridStart
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
	today := m.clock.Today()

	bgSurface := lipgloss.Color(m.palette.Surface)
	baseBGcolor := lipgloss.Color(m.palette.Base)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bgSurface)

	if w < 28 {
		return []string{s.muted.Render("terminal too narrow")}
	}
	const gap = 1

	// Title: month year · right-aligned ‹ › hints.
	title := strings.ToUpper(m.calMonth.Format("January 2006"))
	hints := m.icon("chev_left") + " " + m.icon("chev_right")
	hintW := lipglossWidth(hints)
	titleW := w - hintW - 2
	cap_ := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.BrightText)).Background(bgSurface).Bold(true).Render(title)
	titleLine := " " + cap_ + strings.Repeat(" ", max(0, titleW-lipglossWidth(cap_))) + " " + muted.Render(hints)
	lines := []string{padToWidth(titleLine, w, bgSurface)}

	cw, cellH, weeks, gridStart := calendarLayout(m, w, h)
	_ = cellH
	baseGutter := lipgloss.NewStyle().Background(lipgloss.Color(m.palette.Base)).Render(strings.Repeat(" ", gap))

	// Weekday caps, centered over their columns (same cw + gaps).
	weekday := weekdayHeaders(m.cfg.WeekStart)
	weekNames := []string{}
	for _, d := range weekday {
		weekNames = append(weekNames, strings.ToUpper(d))
	}
	if cw < 5 {
		weekNames = []string{"M", "T", "W", "T", "F", "S", "S"}
		if strings.ToLower(m.cfg.WeekStart) == "sunday" {
			weekNames = []string{"S", "M", "T", "W", "T", "F", "S"}
		}
	}
	headCells := make([]string, 0, 7)
	for _, name := range weekNames {
		headCells = append(headCells, muted.Width(cw).Render(center(name, cw)))
	}
	lines = append(lines, joinGridRow(baseBGcolor, baseGutter, headCells, w))

	// Grid: cellH rows per week, gaps showing base.
	for week := 0; week < weeks; week++ {
		weekStart := gridStart.AddDate(0, 0, week*7)
		cells := make([][]string, 0, 7)
		for d := 0; d < 7; d++ {
			day := weekStart.AddDate(0, 0, d)
			cells = append(cells, calendarCell(m, day, cw, cellH, today))
		}
		for row := 0; row < cellH; row++ {
			rowCells := make([]string, 0, 7)
			for d := 0; d < 7; d++ {
				rowCells = append(rowCells, cells[d][row])
			}
			lines = append(lines, joinGridRow(baseBGcolor, baseGutter, rowCells, w))
		}
	}

	// Agenda.
	lines = append(lines, padToWidth("", w, bgSurface))
	agendaHead := dates.HumanDay(dayStartT(m.calDay))
	if dayStartT(m.calDay).Equal(today) {
		agendaHead = "TODAY · " + agendaHead
	}
	agCap := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(bgSurface).Bold(true).Render(strings.ToUpper(agendaHead))
	lines = append(lines, padToWidth(" "+agCap, w, bgSurface))

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
		// Exams carry a ✦ next to the title, not only the calendar dot.
		exam := strings.EqualFold(t.Kind, core.KindExam)
		examMark := ""
		if exam {
			examMark = "✦ "
		}
		color := theme.CourseColor(m.palette, t.Project, projectColorOverride(m, t.Project))
		stripe := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(bgSurface).Render("▌")
		boardTitle := func(bg lipgloss.Color, style lipgloss.Style, budget int) string {
			whole := truncatePlain(t.Title, budget)
			if done {
				return strikeTitle(lipgloss.Color(m.palette.Muted), bg, whole)
			}
			return style.Render(whole)
		}
		titleBudget := w - 14 - runewidth.StringWidth(examMark)
		if i == m.calAgendaSel && m.focus != focusSidebar {
			selBG := lipgloss.NewStyle().Background(bgRaised)
			selMut := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bgRaised)
			selTxt := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(bgRaised).Bold(true)
			selStripe := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(bgRaised).Render("▌")
			selLine := selBG.Render(" ") + selMut.Render(timeStr) + selStripe + selMut.Render(" "+mark+" ")
			if exam {
				selLine += selTxt.Render(examMark)
			}
			selLine += boardTitle(bgRaised, selTxt, titleBudget)
			lines = append(lines, padToWidth(selLine, w, bgRaised))
			continue
		}
		textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(bgSurface)
		line := " " + muted.Render(timeStr) + stripe + " "
		if exam {
			line += textStyle.Render(examMark)
		}
		line += boardTitle(bgSurface, textStyle, titleBudget)
		lines = append(lines, padToWidth(line, w, bgSurface))
	}
	if len(agenda) == 0 {
		lines = append(lines, padToWidth(" "+s.muted.Render(m.icon("check")+" nothing due — a clear day"), w, bgSurface))
	}

	return lines
}

// joinGridRow joins grid cells with the base gutter and pads with base to w.
func joinGridRow(baseBG lipgloss.Color, gutter string, cells []string, w int) string {
	line := strings.Join(cells, gutter)
	if pad := w - lipgloss.Width(line); pad > 0 {
		line += lipgloss.NewStyle().Background(baseBG).Render(strings.Repeat(" ", pad))
	}
	return line
}

// clickCalendar selects the clicked day cell or an agenda line.
func (m *Model) clickCalendar(x, y int) {
	if m.width < 28 {
		return
	}
	innerW := m.width - 2
	contentH := m.height - 2
	cw, cellH, weeks, gridStart := calendarLayout(*m, innerW, contentH)
	_ = gridStart
	const padTop = 1
	gridTop := 2

	gy := y - padTop - gridTop
	week := gy / cellH
	if gy >= 0 && week < weeks {
		col := x - 1 // body inset 1
		cell := col / (cw + 1)
		within := col % (cw + 1)
		if cell > 6 {
			cell = 6
		}
		if cell < 0 || within == cw {
			return // clicked a base gutter
		}
		m.calDay = gridStart.AddDate(0, 0, week*7+cell)
		m.calMonth = time.Date(m.calDay.Year(), m.calDay.Month(), 1, 0, 0, 0, 0, m.calDay.Location())
		m.calAgendaSel = 0
		return
	}
	// Agenda selection.
	agendaTop := padTop + gridTop + weeks*cellH + 2 // blank + agenda header
	idx := y - agendaTop
	if idx >= 0 && idx < len(agendaTasks(*m)) {
		m.calAgendaSel = idx
	}
}
