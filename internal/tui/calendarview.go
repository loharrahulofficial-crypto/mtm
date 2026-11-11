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
	case aMonthPrev:
		m.calMonth = m.calMonth.AddDate(0, -1, 0)
		m.calDay = clampCalDay(m.calMonth, m.calDay)
	case aMonthNext:
		m.calMonth = m.calMonth.AddDate(0, +1, 0)
		m.calDay = clampCalDay(m.calMonth, m.calDay)
	case aTop:
		m.calAgendaSel = 0
	case aToggleCalendarDone:
		m.calShowDone = !m.calShowDone
		if m.calShowDone {
			m.statusMsg = "calendar: showing completed tasks"
		} else {
			m.statusMsg = "calendar: hiding completed tasks"
		}
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
// day's dot overflow land on the bottom-left; tall-enough cells show up to
// five truncated task titles, top-aligned, with "+N more" on the last line
// when more tasks exist. A mostly-empty cell keeps a subtle bottom edge
// line so the grid still reads as cells.
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
		if fastWidth(row) > cw {
			return clipStyled(row, cw)
		}
		if pad := cw - fastWidth(row); pad > 0 {
			row += fill.Render(strings.Repeat(" ", pad))
		}
		return row
	}
	titlesShown := cellH >= 5 && cw >= 12

	// Row 0: day number (pill for today, bold accent when cursor is on it).
	// Dots sit on the SAME line, to the right of the number. When the cell
	// is tall enough to show titles, the dots are dropped entirely.
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

	// Dots (open tasks due on the day). Cells too narrow for titles
	// (< 12 columns, spec 0b.4) show a count chip with the total in
	// addition to as many course-colored dots as fit.
	dotsRow := ""
	if !titlesShown {
		colors, total, _, exam := dotsForDay(m, day)
		any := false
		if cw < 12 && total > 0 {
			chip := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(lipgloss.Color(m.palette.SurfaceRaised)).Bold(true).Render(fmt.Sprintf("%d", total))
			chipW := fastWidth(chip)
			numW := fastWidth(numStr)
			shown := 0
			room := cw - 3 - numW - chipW // row fill + num + gap before chip
			if exam {
				room--
			}
			for shown < len(colors) {
				if shown >= room {
					break
				}
				dotsRow += lipgloss.NewStyle().Foreground(lipgloss.Color(colors[shown])).Background(bg).Render("●")
				shown++
			}
			if exam {
				dotsRow += lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(bg).Bold(true).Render("✦")
			}
			dotsRow += " " + chip
			any = true
		} else {
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
		}
		if any {
			dotsRow = fill.Render(" ") + dotsRow
		}
	}
	row0 := fill.Render(" ") + numStr + dotsRow
	out := []string{gitRun(row0)}
	for i := 1; i < cellH; i++ {
		out = append(out, gitRun(""))
	}

	// Task titles (only when the cell can breathe). Top-aligned; as many
	// as fit, capped at five; "+N more" takes the last line when the tail
	// does not fit. Cells with fewer titles than height keep a subtle
	// bottom edge line.
	rowsAvail := cellH - 1
	tasks := tasksDueOn(m, day)
	if titlesShown {
		maxTitles := 5
		if rowsAvail < maxTitles {
			maxTitles = rowsAvail
		}
		show := len(tasks)
		if show > maxTitles {
			show = maxTitles
		}
		overflow := len(tasks) - show
		if overflow > 0 {
			show = maxTitles - 1
			if show < 0 {
				show = 0
			}
			overflow = len(tasks) - show
		}
		for i := 0; i < show; i++ {
			t := tasks[i]
			done := m.store.IsDoneStatus(t.Project, t.Status)
			ts := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(bg)
			timePrefix := ""
			if len(t.Due) > len("2006-01-02") {
				timeStr := t.Due[len("2006-01-02")+1:]
				if len(timeStr) > 5 {
					timeStr = timeStr[:5]
				}
				timePrefix = timeStr
			}
			prefixW := 0
			if timePrefix != "" {
				prefixW = 6 // "16:00" + one space
			}
			title := truncatePlain(t.Title, cw-2-prefixW)
			if done {
				text := fill.Render(" ")
				if timePrefix != "" {
					text += subtleTimePrefix(m, bg, timePrefix) + " "
				}
				text += strikeTitle(lipgloss.Color(m.palette.Muted), bg, title)
				out[1+i] = gitRun(text)
				continue
			}
			text := fill.Render(" ")
			if timePrefix != "" {
				text += subtleTimePrefix(m, bg, timePrefix) + " "
			}
			text += ts.Render(title)
			out[1+i] = gitRun(text)
		}
		if overflow > 0 {
			more := fmt.Sprintf(" +%d more", overflow)
			out[cellH-1] = gitRun(fill.Render(" ") + lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bg).Render(more))
		}
	}
	// Mostly-empty cell: a quiet bottom edge so the grid still reads as
	// cells. Full cells (titles or "+N more") keep their own last line;
	// dots mode cells are always mostly-empty.
	cellFull := titlesShown && out[cellH-1] != gitRun("")
	if !isSelected && !cellFull && (!titlesShown || len(tasks) < rowsAvail) {
		out[cellH-1] = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Border)).Background(bg).Render(strings.Repeat("─", cw))
	}

	_ = fill
	// Bottom row: selected → accent underline rule (but never clobber a
	// populated last line, e.g. "+N more"). Dots live on row 0.
	last := cellH - 1
	if isSelected && out[last] == gitRun("") {
		out[last] = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(bg).Render(strings.Repeat("─", cw))
	}
	return out
}

// tasksDueOn returns tasks due on day, sorted like the agenda.
// tasksDueOn feeds the day cells; completed tasks are hidden unless the
// `d` toggle is on.
func tasksDueOn(m Model, day time.Time) []*core.Task {
	saved := m.calDay
	m.calDay = day
	out := agendaTasks(m)
	m.calDay = saved
	if m.calShowDone {
		return out
	}
	isDone := func(p, st string) bool { return m.store.IsDoneStatus(p, st) }
	kept := out[:0]
	for _, t := range out {
		if !isDone(t.Project, t.Status) {
			kept = append(kept, t)
		}
	}
	return kept
}

// subtleTimePrefix renders a "16:00" cell prefix for timed tasks, muted
// against the cell background.
func subtleTimePrefix(m Model, bg lipgloss.Color, timeStr string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bg).Render(timeStr)
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
	sd := fastWidth(s)
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

// calendarColWidths distributes w cells across the 7 columns plus six
// 1-cell internal gutters and one trailing 1-cell right gutter, so the
// Sunday column never touches the panel border and every column shares
// the same gap. The first columns absorb the remainder.
func calendarColWidths(w int) []int {
	const gap = 1
	const nCols = 7
	inner := w - (nCols-1+1)*gap // six internal gaps + one right gutter
	base := inner / nCols
	if base < 2 {
		base = 2
	}
	rem := inner - base*nCols
	cols := make([]int, nCols)
	for i := range cols {
		cols[i] = base
		if i < rem {
			cols[i]++
		}
	}
	return cols
}

// calendarLayout computes the per-column widths and grid height for the
// current content box: colWidths[i] is the width of column i (shared by
// weekday headers and day cells); cells are separated by 1-cell base
// gutters, with a matching 1-cell gutter at the right edge. The agenda
// always keeps a 5-row minimum (title line + 4 entries), cells may grow
// to 8 rows and absorb leftover height before the agenda area expands;
// the agenda shows up to 8 entries of the selected day.
func calendarLayout(m Model, w, h int) (colWidths []int, cellH, weeks int, gridStart time.Time) {
	colWidths = calendarColWidths(w)
	gridStart, weeks = calendarGrid(m)

	const minAgenda = 5 // title line + 4 entries
	const maxCellH = 8
	maxGrid := h - 3 - minAgenda // title + weekday header + blank + agenda
	cellH = maxGrid / weeks
	if cellH > maxCellH {
		cellH = maxCellH
	}
	if cellH < 2 {
		cellH = 2
	}
	return colWidths, cellH, weeks, gridStart
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

	// No in-body month heading — the panel title carries the month
	// ("C A L E N D A R · Nov 2026"); this row is the weekday header.
	colWidths, cellH, weeks, gridStart := calendarLayout(m, w, h)
	_ = cellH
	baseGutter := lipgloss.NewStyle().Background(lipgloss.Color(m.palette.Base)).Render(strings.Repeat(" ", gap))

	// Weekday caps, left-aligned with the day numbers (leading space,
	// same per-column widths).
	weekday := weekdayHeaders(m.cfg.WeekStart)
	weekNames := []string{}
	for _, d := range weekday {
		weekNames = append(weekNames, strings.ToUpper(d))
	}
	for i, cw := range colWidths {
		if cw < 5 {
			weekNames[i] = strings.ToUpper(weekNames[i][:1])
		}
	}
	headCells := make([]string, 0, 7)
	for i, name := range weekNames {
		headCells = append(headCells, muted.Width(colWidths[i]).Render(" "+name))
	}
	lines := []string{joinGridRow(baseBGcolor, baseGutter, headCells, w)}

	// Grid: cellH rows per week, gaps showing base.
	for week := 0; week < weeks; week++ {
		weekStart := gridStart.AddDate(0, 0, week*7)
		cells := make([][]string, 0, 7)
		for d := 0; d < 7; d++ {
			day := weekStart.AddDate(0, 0, d)
			cells = append(cells, calendarCell(m, day, colWidths[d], cellH, today))
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
	const maxAgendaEntries = 8
	agendaCap := agendaH
	if agendaCap > maxAgendaEntries {
		agendaCap = maxAgendaEntries
	}
	bgRaised := lipgloss.Color(m.palette.SurfaceRaised)
	for i, t := range agenda {
		if i >= agendaCap {
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
	if pad := w - fastWidth(line); pad > 0 {
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
	colWidths, cellH, weeks, gridStart := calendarLayout(*m, innerW, contentH)
	_ = gridStart
	const padTop = 1
	gridTop := 1 // weekday header occupies the first content row

	gy := y - padTop - gridTop
	week := gy / cellH
	if gy >= 0 && week < weeks {
		local := x - 1 // body inset 1
		off := 0
		cell := -1
		for i, cw := range colWidths {
			if local >= off && local < off+cw {
				cell = i
				break
			}
			off += cw + 1
		}
		if cell < 0 {
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
