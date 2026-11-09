package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/dates"
	"github.com/loharrahulofficial-crypto/mtm/internal/gauges"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

// Course dashboard detail pane: everything there is to know about one
// course at a glance. Sections render top-down — header, grade gauge with
// target tick, graded items table, hours (shared bar chart), the next exam
// with its topics checklist, and the next five open tasks.
//
// Bars in this view use thin glyphs (lower-half blocks) so adjacent bar
// rows never merge into slabs; sections are separated by a blank spacer.

func renderCourseDetail(m Model, c courseCard, w, h int) []string {
	bg := lipgloss.Color(m.palette.Surface)
	// One cell of padding on each side of the pane so no text touches
	// the border; content rows stay padded to the reduced width and the
	// final lines are wrapped in 1-cell surface gutters.
	w -= 2
	if w < 1 {
		w = 1
	}
	bright := func(s string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.BrightText)).Background(bg).Bold(true).Render(s)
	}
	muted := func(s string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(bg).Render(s)
	}
	subtle := func(s string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bg).Render(s)
	}
	accent := func(s string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(bg).Bold(true).Render(s)
	}
	pad := func(line string) string { return padToWidth(line, w, bg) }

	p := c.project
	color := theme.CourseColor(m.palette, p.Name, projectColorOverride(m, p.Name))
	stripe := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(bg).Render("▌")
	name := courseDisplayName(p)
	compact := w < 100 // stacked: keep spacers out so the task list survives
	lines := []string{
		pad(" " + stripe + " " + bright(truncatePlain(name, w-4))),
		pad("   " + muted(truncatePlain(courseMetaLine(m, c), w-4))),
	}
	if !compact {
		lines = append(lines, pad(""))
	}
	lines = append(lines, courseGradeSection(m, c, w, color, pad, bright, muted, subtle)...)
	if !compact {
		lines = append(lines, pad(""))
	}
	lines = append(lines, courseTableSection(m, c, w, pad, bright, muted, subtle)...)
	if !compact {
		lines = append(lines, pad(""))
	}
	lines = append(lines, courseHoursSection(m, c, w, pad, bright, muted, subtle, accent)...)
	if !compact {
		lines = append(lines, pad(""))
	}
	lines = append(lines, courseExamSection(m, c, w, pad, bright, muted, subtle, accent, color)...)
	if !compact {
		lines = append(lines, pad(""))
	}
	lines = append(lines, courseTasksSection(m, c, w, pad, accent)...)
	_ = h
	gutterCell := lipgloss.NewStyle().Background(lipgloss.Color(m.palette.Surface)).Render(" ")
	for i, l := range lines {
		lines[i] = gutterCell + clipStyled(l, w) + gutterCell
	}
	return lines
}

// courseDisplayName is "CODE — Name" when a course code is set, else the
// course name, else the leaf project name.
func courseDisplayName(p *core.Project) string {
	name := leafName(p.Name)
	if p.Course == nil {
		return name
	}
	code := strings.TrimSpace(p.Course.Code)
	full := strings.TrimSpace(p.Course.Name)
	switch {
	case code != "" && full != "" && !strings.EqualFold(code, full):
		return code + " — " + full
	case code != "":
		return code
	case full != "":
		return full
	default:
		return name
	}
}

// courseMetaLine: "semester · credits · target X (pct%)".
func courseMetaLine(m Model, c courseCard) string {
	p := c.project
	meta := []string{}
	if p.Course != nil {
		if p.Course.Semester != "" {
			meta = append(meta, p.Course.Semester)
		}
		if p.Course.Credits > 0 {
			meta = append(meta, fmt.Sprintf("%d cr", p.Course.Credits))
		}
		if strings.TrimSpace(p.Course.TargetGrade) != "" {
			t := strings.TrimSpace(p.Course.TargetGrade)
			if pct, err := core.GradeToPercent(t); err == nil {
				meta = append(meta, fmt.Sprintf("target %s (%.0f%%)", t, pct))
			} else {
				meta = append(meta, "target "+t)
			}
		}
	}
	return strings.Join(meta, " · ")
}

// sectionLabel renders a muted caps section header with a hairline rule.
func detailSection(m Model, w int, text string) string {
	bg := lipgloss.Color(m.palette.Surface)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bg).Bold(true)
	rule := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Border)).Background(bg)
	label := " " + muted.Render(strings.ToUpper(text)) + " "
	ruleLen := w - lipgloss.Width(label) - 1
	if ruleLen < 1 {
		ruleLen = 1
	}
	return label + rule.Render(strings.Repeat("─", ruleLen))
}

// courseGradeSection: thin-glyph gauge with a bright tick glyph for the
// target, then a single legend line beneath it ("target 90% · need 90.0%
// on remaining 40") that merges the target with the grade status wording
// from Phase 4 (that wording is unchanged). No duplicate target labels,
// no connector glyphs.
func courseGradeSection(m Model, c courseCard, w int, color string, pad func(string) string, bright, muted, subtle func(string) string) []string {
	bg := lipgloss.Color(m.palette.Surface)
	g := c.grades
	if g == nil || len(g.Items) == 0 {
		return []string{pad(" " + muted("grade  nothing graded yet"))}
	}
	const gaugeW = 12
	frac := 0.0
	switch {
	case g.Unweighted:
		frac = g.UnweightedPct / 100
	default:
		frac = g.StandingPct / 100
	}
	frac = clampFloat(frac, 0, 1)
	fill, track := gauges.ThinBar(frac, gaugeW)
	subtleRow := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.SurfaceStrong)).Background(bg)
	fillRow := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(bg)
	barCells := []string{}
	for _, r := range fill {
		barCells = append(barCells, fillRow.Render(string(r)))
	}
	for _, r := range track {
		barCells = append(barCells, subtleRow.Render(string(r)))
	}
	if !g.Unweighted && g.HasTarget && g.TargetPct > 0 {
		tickAt := int(g.TargetPct/100*float64(gaugeW) + 0.5)
		if tickAt >= gaugeW {
			tickAt = gaugeW - 1
		}
		if tickAt < 0 {
			tickAt = 0
		}
		barCells[tickAt] = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.BrightText)).Background(bg).Bold(true).Render("│")
	}
	standing := g.StandingPct
	if g.Unweighted {
		standing = g.UnweightedPct
	}
	pct := fmt.Sprintf("%5.1f%%", clampFloat(standing, 0, 100))
	bar := strings.Join(barCells, "")
	status := gradeStatusLine(g)
	if !g.Unweighted && g.HasTarget && g.TargetPct > 0 {
		status = fmt.Sprintf("target %.0f%% · %s", g.TargetPct, status)
	}
	return []string{
		pad(" " + muted("grade  ") + bar + " " + bright(pct)),
		pad("        " + subtle(truncatePlain(status, w-8))),
	}
}

// courseTableSection: assignment/exam · weight · score · PTS, with a
// total row. PTS is weight × score pct (points earned of assigned
// weight). Only items carrying a weight are listed — unweighted tasks
// live in the Next tasks section. When no weighted item exists, a muted
// hint takes the table's place.
func courseTableSection(m Model, c courseCard, w int, pad func(string) string, bright, muted, subtle func(string) string) []string {
	g := c.grades
	if g == nil || len(g.Items) == 0 {
		return []string{pad(" " + muted("nothing graded yet"))}
	}
	weighted := make([]core.GradeItem, 0, len(g.Items))
	for _, it := range g.Items {
		if it.HasWeight {
			weighted = append(weighted, it)
		}
	}
	if len(weighted) == 0 {
		return []string{
			detailSection(m, w, "Graded items"),
			pad(" " + muted("No weighted items yet. Add one with: mtm grade or :exam")),
		}
	}
	nameW := w - 27 // ' ' + item + ' ' + weight(8) + ' ' + score(9) + ' ' + pts(6) == w
	if nameW < 4 {
		nameW = 4
	}
	r := func(s string, width int) string {
		width -= lipgloss.Width(s)
		if width < 0 {
			width = 0
		}
		return strings.Repeat(" ", width) + s
	}
	l := func(s string, width int) string { return padRightPlain(s, width) }
	lines := []string{detailSection(m, w, "Graded items")}
	header := " " + muted(l("ITEM", nameW)) + " " + muted(r("WEIGHT", 8)) + " " + muted(r("SCORE", 9)) + " " + muted(r("PTS", 6))
	lines = append(lines, pad(header))
	for _, it := range weighted {
		kindMark := ""
		if strings.EqualFold(it.Kind, core.KindExam) {
			kindMark = "✦ "
		}
		name := truncatePlain(kindMark+it.Title, nameW)
		wtxt, stxt, ctxt := "—", "—", "—"
		wtxt = trimFloat(it.Weight)
		if it.Scored {
			stxt = fmt.Sprintf("%g/%g", it.Score, it.ScoreMax)
			if it.ScoreMax > 0 {
				pct := it.Score / it.ScoreMax * 100
				ctxt = fmt.Sprintf("%.1f", it.Weight*pct/100)
			}
		}
		line := " " + l(name, nameW) + " " + r(wtxt, 8) + " " + r(stxt, 9) + " " + r(ctxt, 6)
		lines = append(lines, pad(line))
	}
	// Total row: hairline above it.
	rule := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Border)).Background(lipgloss.Color(m.palette.Surface)).Render(strings.Repeat("─", w))
	lines = append(lines, rule)
	assigned := "—"
	contributed := "—"
	if g.TotalWeight > 0 {
		assigned = fmt.Sprintf("%g", g.TotalWeight)
		contributed = fmt.Sprintf("%.1f", g.StandingPct*g.TotalWeight/100)
	}
	total := " " + bright(l("TOTAL", nameW)) + " " + bright(r(assigned, 8)) + " " + bright(r("", 9)) + " " + bright(r(contributed, 6))
	lines = append(lines, pad(total))
	return lines
}

// courseHoursSection: the shared bar chart of the last 7 days, plus total
// logged vs the estimate accuracy rollup ("N of M tasks estimated"; the
// percentage only with at least 3 estimated tasks).
func courseHoursSection(m Model, c courseCard, w int, pad func(string) string, bright, muted, subtle func(string) string, accent func(string) string) []string {
	bg := lipgloss.Color(m.palette.Surface)
	now := m.clock.Now()
	bgSurface := bg
	surfaceBG := lipgloss.NewStyle().Background(bgSurface)
	mutedFG := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(bgSurface)
	mutedB := func(s string) string { return mutedFG.Render(s) }
	brightB := func(s string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.BrightText)).Background(bgSurface).Bold(true).Render(s)
	}
	accentB := func(s string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(bgSurface).Render(s)
	}
	todayIdx := 6
	letters := []string{"M", "T", "W", "T", "F", "S", "S"}
	for i := 0; i < 7; i++ {
		day := now.AddDate(0, 0, i-6)
		letters[i] = strings.ToUpper(day.Format("Mon")[:1])
	}
	height := 8
	if w < 100 {
		height = 6
	}
	cfg := gauges.BarChartConfig{
		Values:      c.weekHours[:],
		Labels:      letters,
		Width:       w,
		Height:      height,
		MaxBarRows:  6,
		BarWidth:    2,
		Today:       todayIdx,
		ShowTotal:   true,
		ShowAverage: false,
		Accent:      accentB,
		Bar:         accentB,
		Muted:       mutedB,
		Bright:      brightB,
		Bg:          func(s string) string { return surfaceBG.Render(s) },
	}
	lines := []string{detailSection(m, w, "Hours")}
	for _, l := range gauges.RenderBarChart(cfg) {
		lines = append(lines, pad(l))
	}
	total := fmt.Sprintf("total %s logged", actions.FormatMinutes(c.hours.LoggedMinutes))
	if c.hours.Accuracy.Total > 0 {
		total += fmt.Sprintf(" · %s", c.hours.Accuracy.Label())
		if c.hours.Accuracy.Enough {
			total += fmt.Sprintf(" (%.0f%% of estimate)", c.hours.Accuracy.Pct())
		}
	}
	lines = append(lines, pad(" "+muted(total)))
	return lines
}

// courseExamSection: the nearest open exam — header with the date,
// block-digit countdown with "in N days" beside it, weight, topics
// progress bar, per-topic checklist. The date renders once, in the
// header line; topics are marked with Nerd Font check glyphs (Unicode
// fallbacks), muted when done — no strikethrough.
func courseExamSection(m Model, c courseCard, w int, pad func(string) string, bright, muted, subtle func(string) string, accent func(string) string, color string) []string {
	bg := lipgloss.Color(m.palette.Surface)
	subtleRow := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.SurfaceStrong)).Background(bg)
	colorRow := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(bg)

	lines := []string{detailSection(m, w, "Exams")}
	if len(c.exams) == 0 {
		lines = append(lines, pad(" "+muted("nothing scheduled — e adds one")))
		return lines
	}
	e := c.exams[0]
	task := e.task
	isDone := func(p, st string) bool { return m.store.IsDoneStatus(p, st) }
	dueDay := parseDayStart(task.Due, time.Local)
	dateTxt := ""
	if !dueDay.IsZero() {
		dateTxt = dates.HumanDay(dueDay)
	}
	inTxt := "today"
	switch {
	case e.daysLeft == 1:
		inTxt = "tomorrow"
	case e.daysLeft > 1:
		inTxt = fmt.Sprintf("in %d days", e.daysLeft)
	}
	weightTxt := ""
	weight := -1.0
	if task.Weight != nil {
		weight = *task.Weight
		weightTxt = fmt.Sprintf("weight %.0f%%", weight)
	}
	subDone, subTotal, _ := core.SubtaskProgress(m.store.Tasks(), task.ID, isDone)
	meta := weightTxt
	if subTotal > 0 {
		if meta != "" {
			meta += " · "
		}
		meta += fmt.Sprintf("topics %d/%d", subDone, subTotal)
	}

	// Header line: the exam name and the date, exactly once.
	header := " " + accent("✦") + " "
	nameBudget := w - 4
	if dateTxt != "" {
		nameBudget -= lipgloss.Width(dateTxt) + 3 // " · " + date
	}
	if nameBudget < 8 {
		nameBudget = 8
	}
	header += bright(truncatePlain(task.Title, nameBudget))
	if dateTxt != "" {
		header += muted(" · " + dateTxt)
	}
	lines = append(lines, pad(header))
	if meta != "" {
		lines = append(lines, pad("   "+muted(meta)))
	}

	// Countdown digits beside the in-N-days line.
	digits := gauges.Digits(fmt.Sprintf("%dd", e.daysLeft))
	topicsLine := ""
	if subTotal > 0 {
		fill, track := gauges.ThinBar(float64(subDone)/float64(subTotal), 10)
		topicsLine = muted("topics ") + colorRow.Render(fill) + subtleRow.Render(track) + muted(fmt.Sprintf(" %d/%d", subDone, subTotal))
	} else {
		topicsLine = muted("topics not set")
	}
	inStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(bg).Bold(true)
	right := []string{
		inStyle.Render(inTxt),
		"",
		"",
	}
	for row := 0; row < 3; row++ {
		line := "  " + accent(digits[row]) + "  " + right[row]
		lines = append(lines, pad(line))
	}
	lines = append(lines, pad("   "+topicsLine))

	// Per-topic checklist with Nerd Font check glyphs; done topics are
	// muted, never struck through.
	sub := examSubtasks(m, task.ID)
	if len(sub) > 0 {
		shown := 0
		maxChecklist := 4
		if w < 100 {
			maxChecklist = 2
		}
		openIcon := m.icon("circle")
		doneIcon := m.icon("check")
		iconCell := func(ic string) string {
			if ic == "" {
				return "   "
			}
			st := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bg)
			return st.Render(" " + ic + " ")
		}
		for _, st := range sub {
			done := isDone(st.Project, st.Status)
			title := truncatePlain(st.Title, w-8)
			cell := iconCell(openIcon)
			text := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(bg).Render(title)
			if done {
				cell = iconCell(doneIcon)
				text = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bg).Render(title)
			}
			lines = append(lines, pad("   "+cell+text))
			shown++
			if shown >= maxChecklist {
				break
			}
		}
		if len(sub) > shown && w >= 100 {
			lines = append(lines, pad("   "+muted(fmt.Sprintf("+%d more topics", len(sub)-shown))))
		}
	}
	return lines
}

// courseTasksSection: the next five open tasks for the course, due first.
func courseTasksSection(m Model, c courseCard, w int, pad func(string) string, accent func(string) string) []string {
	bg := lipgloss.Color(m.palette.Surface)
	mutedRow := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(bg)
	isDone := func(p, st string) bool { return m.store.IsDoneStatus(p, st) }
	var open []*core.Task
	for _, t := range m.store.Tasks() {
		if !strings.EqualFold(t.Project, c.project.Name) {
			continue
		}
		if isDone(t.Project, t.Status) {
			continue
		}
		if strings.EqualFold(t.Kind, core.KindExam) {
			continue // exams live in the EXAMS section
		}
		open = append(open, t)
	}
	core.SortTasks(open, []string{core.SortDue, core.SortPriority, core.SortTitle}, isDone)
	total := len(open)
	if len(open) > 5 {
		open = open[:5]
	}
	lines := []string{detailSection(m, w, "Next tasks")}
	if total == 0 {
		lines = append(lines, pad(" "+mutedRow.Render("nothing open — a clear runway")))
		return lines
	}
	now := m.clock.Now()
	for _, t := range open {
		color := theme.CourseColor(m.palette, t.Project, projectColorOverride(m, t.Project))
		stripe := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(bg).Render("▌")
		labelText, heat := gauges.DueLabel(t.Due, now)
		due := ""
		if t.Due != "" {
			dc := lipgloss.NewStyle().Background(bg)
			switch heat {
			case gauges.HeatWarm:
				dc = dc.Foreground(lipgloss.Color(m.palette.HeatWarm))
			case gauges.HeatHot:
				dc = dc.Foreground(lipgloss.Color(m.palette.HeatHot)).Bold(true)
			case gauges.HeatOverdue:
				dc = dc.Foreground(lipgloss.Color(m.palette.HeatOverdue)).Bold(true)
			default:
				dc = dc.Foreground(lipgloss.Color(m.palette.MutedSoft))
			}
			due = dc.Render(labelText)
		}
		dueW := lipgloss.Width(due)
		budget := w - 4 - dueW - 2
		if budget < 8 {
			budget = 8
		}
		title := truncatePlain(t.Title, budget)
		txt := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(bg)
		line := " " + stripe + " " + txt.Render(title)
		if due != "" {
			padCells := w - lipgloss.Width(line) - dueW - 1
			if padCells < 1 {
				line = clipStyled(line, w-dueW-2)
				padCells = 1
			}
			line += lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", padCells)) + due + " "
		}
		lines = append(lines, pad(line))
	}
	if extra := total - len(open); extra > 0 {
		lines = append(lines, pad(" "+mutedRow.Render(fmt.Sprintf("+%d more open", extra))))
	}
	return lines
}

// examSubtasks lists the subtasks of an exam task (the syllabus topics).
func examSubtasks(m Model, examID string) []*core.Task {
	var out []*core.Task
	for _, t := range m.store.Tasks() {
		if t.Parent == examID {
			out = append(out, t)
		}
	}
	return out
}
