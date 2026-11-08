package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/dates"
	"github.com/loharrahulofficial-crypto/mtm/internal/gauges"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

// Course dashboard: a card grid of the user's courses. One card per
// course with the course stripe, a grade gauge with target tick, the
// overall progress bar, a 7-day hours sparkline, the next exam with
// countdown and a grade status line. h/j/k/l move between cards; enter
// opens that course's tasks filtered; esc returns here.

// courseCard bundles everything one card needs.
type courseCard struct {
	project   *core.Project
	stats     *actions.CourseStats
	grades    *core.CourseGrades
	hours     actions.HoursRollup
	weekHours [7]float64
	hasHours  bool
	exams     []courseExam
}

type courseExam struct {
	task     *core.Task
	daysLeft int
}

func collectCourseCards(m Model) ([]courseCard, error) {
	courses, err := actions.ListCourses(m.store)
	if err != nil {
		return nil, err
	}
	now := m.clock.Now()
	y, mo, dd := now.Date()
	todayStart := time.Date(y, mo, dd, 0, 0, 0, 0, now.Location())
	hoursRep, err := actions.CollectHours(m.store, now)
	if err != nil {
		return nil, err
	}
	hoursByCourse := map[string]actions.HoursRollup{}
	for _, r := range hoursRep.Courses {
		hoursByCourse[r.Course] = r
	}
	isDone := func(p, st string) bool { return m.store.IsDoneStatus(p, st) }
	live := m.store.Tasks()
	var cards []courseCard
	for _, p := range courses {
		card := courseCard{project: p}
		stats, err := actions.CollectCourseStats(m.store, p.Name)
		if err != nil {
			return nil, err
		}
		card.stats = stats
		g, _, err := actions.CollectCourseGrades(m.store, p.Name, "")
		if err != nil {
			return nil, err
		}
		card.grades = g
		if r, ok := hoursByCourse[p.Name]; ok {
			card.hours = r
			card.hasHours = true
		}
		// Per-course 7-day hours, attributed like the Home weekly chart.
		for _, t := range live {
			if !strings.EqualFold(t.Project, p.Name) || t.TimeLoggedMinutes <= 0 {
				continue
			}
			stamp := t.Completed
			if stamp == "" && isDone(t.Project, t.Status) {
				stamp = t.Updated
			}
			if stamp == "" {
				continue
			}
			ts, err := time.ParseInLocation(core.TimestampFormat, stamp, now.Location())
			if err != nil {
				continue
			}
			for i := 0; i < 7; i++ {
				day := todayStart.AddDate(0, 0, i-6)
				if sameDay(ts, day) {
					card.weekHours[i] += float64(t.TimeLoggedMinutes) / 60.0
				}
			}
		}
		for _, t := range live {
			if !strings.EqualFold(t.Project, p.Name) || !strings.EqualFold(t.Kind, core.KindExam) {
				continue
			}
			if isDone(t.Project, t.Status) {
				continue
			}
			dueDay := parseDayStart(t.Due, now.Location())
			if dueDay.IsZero() {
				continue
			}
			diff := int(dueDay.Sub(todayStart).Hours() / 24)
			if diff < 0 {
				continue
			}
			card.exams = append(card.exams, courseExam{task: t, daysLeft: diff})
		}
		sortExamsByDays(card.exams)
		cards = append(cards, card)
	}
	return cards, nil
}

func sortExamsByDays(exams []courseExam) {
	for i := 1; i < len(exams); i++ {
		for j := i; j > 0 && exams[j].daysLeft < exams[j-1].daysLeft; j-- {
			exams[j], exams[j-1] = exams[j-1], exams[j]
		}
	}
}

// cardCols picks the columns for the grid from the content width:
// 1 col below 64, 2 below 100, 3 above.
func cardCols(w int) int {
	switch {
	case w >= 100:
		return 3
	case w >= 64:
		return 2
	default:
		return 1
	}
}

const cardGap = 1
const cardRowGap = 1

// renderCoursesBody renders the course dashboard at exactly w×h.
func renderCoursesBody(m Model, w, h int) string {
	cards, err := collectCourseCards(m)
	hint := ""
	if err == nil {
		hint = fmt.Sprintf("%d active", len(cards))
	}
	panel := func(body []string) string {
		return renderPanel(m, "COURSES", w, h, m.focus == focusMain, body, hint)
	}
	if err != nil {
		return panel([]string{"", " " + m.styles.muted.Render("courses: "+err.Error())})
	}
	if len(cards) == 0 {
		return panel([]string{"", " " + m.styles.muted.Render("no courses yet — see `mtm course add -h`")})
	}
	body := renderCardGrid(m, w-2, h-2, cards)
	return panel(body)
}

// renderCardGrid lays the cards out in a 1–3 column grid. Every card is
// nine rows tall (leading blank + seven content rows + trailing pad for
// unselected cards; accent rounded border for the selected card).
func renderCardGrid(m Model, w, h int, cards []courseCard) []string {
	surface := lipgloss.Color(m.palette.Surface)
	cols := cardCols(w)
	cardW := (w - (cols-1)*cardGap) / cols
	if cardW < 20 {
		cardW = 20
	}

	lines := []string{}
	for row := 0; row < len(cards); row += cols {
		end := row + cols
		if end > len(cards) {
			end = len(cards)
		}
		rendered := [][]string{}
		for idx := row; idx < end; idx++ {
			rendered = append(rendered, renderCourseCard(m, idx, cards[idx], cardW))
		}
		slotH := 0
		for _, cl := range rendered {
			if len(cl) > slotH {
				slotH = len(cl)
			}
		}
		fill := func(n int) string {
			return lipgloss.NewStyle().Background(lipgloss.Color(m.palette.SurfaceRaised)).Render(strings.Repeat(" ", n))
		}
		for lineIdx := 0; lineIdx < slotH; lineIdx++ {
			parts := []string{}
			for _, cl := range rendered {
				if lineIdx < len(cl) {
					parts = append(parts, cl[lineIdx])
				} else {
					parts = append(parts, fill(cardW))
				}
			}
			line := parts[0]
			for j := 1; j < len(parts); j++ {
				line += strings.Repeat(" ", cardGap) + parts[j]
			}
			if pad := w - lipgloss.Width(line); pad > 0 {
				line += lipgloss.NewStyle().Background(surface).Render(strings.Repeat(" ", pad))
			}
			lines = append(lines, line)
		}
		// Surface gap between grid rows.
		if end < len(cards) {
			for i := 0; i < cardRowGap; i++ {
				lines = append(lines, lipgloss.NewStyle().Background(surface).Render(strings.Repeat(" ", w)))
			}
		}
	}
	return lines
}

// renderCourseCard renders one card. Selected cards wrap the content in
// the accent rounded border (replacing the blank fill rows).
func renderCourseCard(m Model, idx int, card courseCard, w int) []string {
	selected := idx == m.courseSel
	p := card.project
	course := p.Course
	bg := lipgloss.Color(m.palette.SurfaceRaised)
	raised := bg
	courseColor := theme.CourseColor(m.palette, p.Name, projectColorOverride(m, p.Name))

	name := leafName(p.Name)
	if course != nil && strings.TrimSpace(course.Code) != "" {
		name = course.Code
		if strings.TrimSpace(course.Name) != "" && !strings.EqualFold(course.Code, course.Name) {
			name = course.Code + " — " + course.Name
		}
	}
	bright := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.BrightText)).Background(raised).Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(raised)
	mutedRaw := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(raised)
	fillStyle := lipgloss.NewStyle().Background(raised)

	stripe := lipgloss.NewStyle().Foreground(lipgloss.Color(courseColor)).Background(raised).Render("▌")
	nameBudget := w - 3
	nameTxt := bright.Render(truncatePlain(name, nameBudget))
	row0 := stripe + " " + nameTxt

	// Meta row: semester · credits · target (verbatim Phase 4 format).
	meta := []string{}
	if course != nil {
		if course.Semester != "" {
			meta = append(meta, course.Semester)
		}
		if course.Credits > 0 {
			meta = append(meta, fmt.Sprintf("%d cr", course.Credits))
		}
		if strings.TrimSpace(course.TargetGrade) != "" {
			if pct, err := core.GradeToPercent(course.TargetGrade); err == nil {
				meta = append(meta, fmt.Sprintf("target %s (%.0f%%)", course.TargetGrade, pct))
			} else {
				meta = append(meta, "target "+course.TargetGrade)
			}
		}
	}
	row1 := "  " + muted.Render(truncatePlain(strings.Join(meta, " · "), w-3))

	// Grade gauge with a target tick.
	g := card.grades
	gaugeRow := ""
	if g == nil || len(g.Items) == 0 {
		gaugeRow = "  " + m.styles.muted.Render("grade  nothing graded yet")
	} else if g.Unweighted {
		gaugeRow = "  " + muted.Render(fmt.Sprintf("grade  avg %.1f%% · no weights set", g.UnweightedPct))
	} else {
		const gaugeW = 12
		frac := clampFloat(g.StandingPct/100, 0, 1)
		fillCells := int(frac*float64(gaugeW) + 0.5)
		tickAt := -1
		if g.HasTarget && g.TargetPct > 0 {
			tickAt = int(g.TargetPct/100*float64(gaugeW) + 0.5)
			if tickAt >= gaugeW {
				tickAt = gaugeW - 1
			}
			if tickAt < 0 {
				tickAt = 0
			}
		}
		var gb strings.Builder
		for i := 0; i < gaugeW; i++ {
			switch {
			case i == tickAt:
				gb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.BrightText)).Background(raised).Render("│"))
			case i < fillCells:
				gb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(courseColor)).Background(raised).Render("█"))
			default:
				gb.WriteString(mutedRaw.Render("░"))
			}
		}
		pct := fmt.Sprintf("%3.0f%%", g.StandingPct)
		gaugeRow = "  " + muted.Render("grade  ") + gb.String() + " " + bright.Render(pct)
	}

	// Overall progress bar.
	overall := card.stats.Overall
	fillBar, track := gauges.Bar(easedFrac(m, overall.Fraction), 8)
	bar := lipgloss.NewStyle().Foreground(lipgloss.Color(courseColor)).Background(raised).Render(fillBar) +
		mutedRaw.Render(track)
	fracTxt := fmt.Sprintf("%3.0f%%", overall.Fraction*100)
	doneTxt := fmt.Sprintf("%d/%d", overall.Done, overall.Total)
	doneRow := "  " + muted.Render("done   ") + bar + " " + bright.Render(fracTxt) + " " + muted.Render(doneTxt)

	// 7-day hours sparkline.
	spark := gauges.Sparkline(card.weekHours[:])
	weekTotal := 0.0
	for _, v := range card.weekHours {
		weekTotal += v
	}
	hoursRow := "  " + muted.Render("hours  ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(raised).Render(spark) +
		" " + bright.Render(fmt.Sprintf("%.1fh", weekTotal)) +
		muted.Render(" this week")

	// All-time total, kept short so the row fits narrow cards.
	if card.hasHours && card.hours.LoggedMinutes > 0 {
		tot := card.hours.LoggedMinutes / 60
		hoursRow += muted.Render(fmt.Sprintf(" · %dh", tot))
	}

	// Next exam chip with countdown.
	examRow := "  " + muted.Render("exam   ") + muted.Render("nothing scheduled")
	if len(card.exams) > 0 {
		e := card.exams[0]
		countdown := "today"
		switch {
		case e.daysLeft == 1:
			countdown = "tomorrow"
		case e.daysLeft > 1:
			countdown = fmt.Sprintf("in %d days", e.daysLeft)
		}
		title := truncatePlain(e.task.Title, w-20-lipgloss.Width(countdown))
		star := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(raised).Bold(true).Render("✦")
		examRow = "  " + star + " " + bright.Render(title) + " " + muted.Render("· "+countdown)
	}

	// Grade status line (wording unchanged from the Phase 4 dashboard).
	statusRow := "  " + muted.Render(truncatePlain(gradeStatusLine(g), w-3))

	content := []string{row0, row1, gaugeRow, doneRow, hoursRow, examRow, statusRow}

	if !selected {
		out := make([]string, 0, 9)
		out = append(out, fillStyle.Render(strings.Repeat(" ", w)))
		for _, l := range content {
			out = append(out, padRightCourseCard(l, w, raised))
		}
		out = append(out, fillStyle.Render(strings.Repeat(" ", w)))
		return out
	}
	// Selected card: accent rounded border replacing the blank fill rows.
	border := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(raised)
	top := border.Render("╭" + strings.Repeat("─", w-2) + "╮")
	bot := border.Render("╰" + strings.Repeat("─", w-2) + "╯")
	out := []string{top}
	for _, l := range content {
		out = append(out, border.Render("│")+padRightCourseCard(l, w-2, raised)+border.Render("│"))
	}
	out = append(out, bot)
	return out
}

func padRightCourseCard(line string, w int, bg lipgloss.Color) string {
	if lipgloss.Width(line) > w {
		return clipStyled(line, w)
	}
	if pad := w - lipgloss.Width(line); pad > 0 {
		line += lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", pad))
	}
	return line
}

func clampFloat(f, lo, hi float64) float64 {
	if f < lo {
		return lo
	}
	if f > hi {
		return hi
	}
	return f
}

// gradeStatusLine renders the one-line grade status. Wording is identical
// to the Phase 4 dashboard continuation lines.
func gradeStatusLine(g *core.CourseGrades) string {
	if g == nil || len(g.Items) == 0 {
		return "no assignments or exams yet"
	}
	if g.Unweighted {
		return fmt.Sprintf("avg %.1f%% · no weights set", g.UnweightedPct)
	}
	switch {
	case !g.HasTarget:
		return "no target grade — set one with `mtm course edit`"
	case g.RemainingWeight == 0:
		if g.StandingPct >= g.TargetPct {
			return fmt.Sprintf("done — target met (%.1f%%)", g.StandingPct)
		}
		return fmt.Sprintf("done — final %.1f%%, target %.0f%% not met", g.StandingPct, g.TargetPct)
	case g.NeededSecured:
		return "target secured — nothing more required"
	case g.NeededFeasible:
		return fmt.Sprintf("need %.1f%% on remaining %s", g.NeededOnRemaining, trimFloat(g.RemainingWeight))
	default:
		best := 0.0
		if g.TotalWeight > 0 {
			best = (g.StandingPct*g.TotalWeight + 100*g.RemainingWeight) / g.TotalWeight
		}
		return fmt.Sprintf("out of reach — even 100%% on the rest yields %.1f%%", best)
	}
}

func trimFloat(f float64) string {
	return fmt.Sprintf("%g", f)
}

var _ = dates.HumanDay

// updateCoursesKey moves the card selection with h/j/k/l, opens the
// selected course's task list with enter, and returns nothing else.
func (m Model) updateCoursesKey(act string) (tea.Model, tea.Cmd) {
	cards, err := collectCourseCards(m)
	n := 0
	if err == nil {
		n = len(cards)
	}
	cols := cardCols(m.width - 2 - 2) // content minus panel + padding
	switch act {
	case aUp:
		m.courseSel -= cols
	case aDown:
		m.courseSel += cols
	case aLeft:
		m.courseSel--
	case aRight:
		m.courseSel++
	case aTop:
		m.courseSel = 0
	case aBottom:
		m.courseSel = n - 1
	case aEnter:
		return m.openCourseDetail(cards, err)
	}
	if m.courseSel < 0 {
		m.courseSel = 0
	}
	if m.courseSel >= n && n > 0 {
		m.courseSel = n - 1
	}
	return m, nil
}

// openCourseDetail switches to the list view with the selected course's
// tasks filtered. esc returns here (handled in updateNormalKey).
func (m Model) openCourseDetail(cards []courseCard, err error) (tea.Model, tea.Cmd) {
	if err != nil || m.courseSel < 0 || m.courseSel >= len(cards) {
		return m, nil
	}
	name := cards[m.courseSel].project.Name
	for i, it := range m.sidebarItems {
		if it.Kind == "project" && strings.EqualFold(it.Key, name) {
			m.sidebarSel = i
			break
		}
	}
	m.coursesDrillIn = name
	m.view = viewList
	m.refreshTasks()
	m.cursor = 0
	m.clampCursor()
	return m, nil
}
