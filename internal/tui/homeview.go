package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/dates"
	"github.com/loharrahulofficial-crypto/mtm/internal/gauges"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

// splitPane renders a panel and splits it into lines for hstack.
func splitPane(m Model, title string, w, h int, focused bool, body []string) []string {
	return strings.Split(renderPanel(m, title, w, h, focused, body), "\n")
}

// renderHomeBody renders the Home dashboard (inner width × inner height).
// Reflows 3 columns (≥150) → 2 columns (≥100) → stacked.
func renderHomeBody(m Model, w, h int) []string {
	d := m.homeDataCached()

	bgBase := lipgloss.Color(m.palette.Base)
	base := lipgloss.NewStyle().Background(bgBase)

	sparse := len(m.store.Tasks()) <= 3

	lines := []string{}
	lines = append(lines, homeHeaderLine(m, w, d))
	lines = append(lines, "")
	lines = append(lines, homeGreetingLine(m, d, w))
	stripRows := 0
	if sparse {
		stripRows = 1
	}
	avail := h - len(lines) - stripRows
	if avail < 8 {
		// Extremely short: just header + greeting.
		if sparse {
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bgBase).Render(" a add · F focus · ctrl+p palette · ? help"))
		}
		return lines
	}
	// Content-aware band heights: TODAY is clamped to [6, 40%] and
	// grows only until the exam cards fit (so the countdown digits are
	// visible by default); the freed rows go to the lower bands.
	contentRows := len(todayBody(m, w-2, 1<<30, d))
	examsPanelW := 0
	switch {
	case w >= 150:
		examsPanelW = (w * 24) / 100
	case w >= 100:
		examsPanelW = w - (w*46+99)/100 - 1
	default:
		examsPanelW = w
	}
	row1H := contentRows + 2
	if len(d.exams) > 0 {
		if need := examsHeight(d.exams, examsPanelW-2) + 2; need > row1H {
			row1H = need
		}
	}
	if maxH := (avail * 40) / 100; row1H > maxH {
		row1H = maxH
	}
	if row1H < 6 {
		row1H = 6
	}
	remaining := avail - row1H
	row3H := (remaining * 40) / 100
	if row3H < 9 {
		row3H = 9
	}
	if row3H > 11 {
		row3H = 11
	}
	row2H := remaining - row3H
	if row2H < 6 {
		row2H = 6
		row3H = remaining - row2H
		if row3H < 4 {
			row3H = 4
		}
	}

	gutter := 1
	switch {
	case w >= 150:
		todayW := (w*44 + 99) / 100
		examsW := (w * 24) / 100
		coursesW := w - todayW - examsW - 2*gutter
		row1 := hstack([][]string{
			splitPane(m, "TODAY", todayW, row1H, m.focus == focusMain, todayBody(m, todayW-2, row1H-2, d)),
			splitPane(m, "EXAMS", examsW, row1H, false, examsBody(m, examsW-2, row1H-2, d)),
			splitPane(m, "COURSES", coursesW, row1H, false, coursesBody(m, coursesW-2, row1H-2, d)),
		}, gutter, base)
		row2 := hstack([][]string{
			splitPane(m, "THIS WEEK", w, row2H, false, thisWeekBody(m, w-2, row2H-2, d)),
		}, gutter, base)
		row3 := hstack([][]string{
			splitPane(m, "HEATMAP", w, row3H, false, heatmapBody(m, w-2, row3H-2, d)),
		}, gutter, base)
		lines = append(lines, row1...)
		lines = append(lines, row2...)
		lines = append(lines, row3...)
	case w >= 100:
		todayW := (w*46 + 99) / 100
		examsW := w - todayW - gutter
		coursesW := (w*52 + 99) / 100
		weekW := w - coursesW - gutter
		row1 := hstack([][]string{
			splitPane(m, "TODAY", todayW, row1H, m.focus == focusMain, todayBody(m, todayW-2, row1H-2, d)),
			splitPane(m, "EXAMS", examsW, row1H, false, examsBody(m, examsW-2, row1H-2, d)),
		}, gutter, base)
		row2 := hstack([][]string{
			splitPane(m, "COURSES", coursesW, row2H, false, coursesBody(m, coursesW-2, row2H-2, d)),
			splitPane(m, "THIS WEEK", weekW, row2H, false, thisWeekBody(m, weekW-2, row2H-2, d)),
		}, gutter, base)
		row3 := hstack([][]string{
			splitPane(m, "HEATMAP", w, row3H, false, heatmapBody(m, w-2, row3H-2, d)),
		}, gutter, base)
		lines = append(lines, row1...)
		lines = append(lines, row2...)
		lines = append(lines, row3...)
	default:
		// Stacked: TODAY is sized to its content; the remaining
		// panels split the leftover proportionally with per-panel
		// minimums, HEATMAP flexing to absorb the slack.
		left := avail - row1H
		weekH := left / 5
		coursesH := left / 5
		examsH := left * 3 / 10
		heatH := left - weekH - coursesH - examsH
		floors := []struct {
			p   *int
			min int
		}{
			{&weekH, 6},
			{&coursesH, 4},
			{&examsH, 5},
			{&heatH, 7},
		}
		if avail < 25 {
			floors[0].min, floors[1].min, floors[2].min, floors[3].min = 3, 2, 3, 5
		}
		for _, f := range floors {
			if *f.p < f.min {
				*f.p = f.min
			}
		}
		for total := weekH + coursesH + examsH + heatH; total > left; {
			decremented := false
			for _, f := range floors {
				if *f.p > f.min {
					*f.p--
					decremented = true
					break
				}
			}
			if !decremented {
				for _, f := range floors {
					if *f.p > 2 {
						*f.p--
						decremented = true
						break
					}
				}
				if !decremented {
					break // tiny terminal; paint clips the overflow
				}
			}
			total--
		}
		if weekH < 1 {
			weekH = 1
		}
		if coursesH < 1 {
			coursesH = 1
		}
		if examsH < 1 {
			examsH = 1
		}
		if heatH < 1 {
			heatH = 1
		}
		blocks := []struct {
			title  string
			height int
			body   func(iw, ih int) []string
		}{
			{"TODAY", row1H, func(iw, ih int) []string { return todayBody(m, iw, ih, d) }},
			{"THIS WEEK", weekH, func(iw, ih int) []string { return thisWeekBody(m, iw, ih, d) }},
			{"COURSES", coursesH, func(iw, ih int) []string { return coursesBody(m, iw, ih, d) }},
			{"EXAMS", examsH, func(iw, ih int) []string { return examsBody(m, iw, ih, d) }},
			{"HEATMAP", heatH, func(iw, ih int) []string { return heatmapBody(m, iw, ih, d) }},
		}
		focused := m.focus == focusMain
		for i, b := range blocks {
			body := b.body(w-2, b.height-2)
			lines = append(lines, splitPane(m, b.title, w, b.height, focused && i == 0, body)...)
		}
	}
	if sparse {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bgBase).Render(" a add · F focus · ctrl+p palette · ? help"))
	}
	return lines
}

// homeHeaderLine renders the quiet date line: "Sat 7 Nov" on the left (plus
// "· Week N of M" only when a semester is configured). No clock, no sync
// pill — the statusline owns both.
func homeHeaderLine(m Model, w int, d homeData) string {
	left := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.BrightText)).Background(lipgloss.Color(m.palette.Base)).Bold(true).
		Render(" " + d.dateLine)
	if d.weekLine != "" {
		left += lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(lipgloss.Color(m.palette.Base)).
			Render(" · " + d.weekLine)
	}
	return left
}

// homeGreetingLine renders the time-aware greeting with due/overdue counts.
func homeGreetingLine(m Model, d homeData, width int) string {
	hour := d.now.Hour()
	greet := "Good morning"
	switch {
	case hour >= 12 && hour < 17:
		greet = "Good afternoon"
	case hour >= 17 && hour < 22:
		greet = "Good evening"
	case hour >= 22 || hour < 5:
		greet = "Working late"
	}

	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(lipgloss.Color(m.palette.Base))
	bright := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.BrightText)).Background(lipgloss.Color(m.palette.Base)).Bold(true)
	hot := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.HeatHot)).Background(lipgloss.Color(m.palette.Base)).Bold(true)
	danger := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.HeatOverdue)).Background(lipgloss.Color(m.palette.Base)).Bold(true)
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(lipgloss.Color(m.palette.Base)).Bold(true)

	examBit := ""
	if d.nextExam != nil && d.nextExamDays >= 0 {
		switch d.nextExamDays {
		case 0:
			examBit = " · exam today!"
		case 1:
			examBit = " · exam tomorrow"
		default:
			examBit = fmt.Sprintf(" · next exam in %d days", d.nextExamDays)
		}
	}

	build := func(includeExam, compact bool) string {
		line := " " + muted.Render(greet+", ") + bright.Render(d.user+".") + "  "
		if compact {
			line += hot.Render(fmt.Sprintf("%d due today · ", d.dueToday))
			line += danger.Render(fmt.Sprintf("%d overdue", d.overdue))
			return line
		}
		line += hot.Render(fmt.Sprintf("%d due today", d.dueToday)) + muted.Render(" · ")
		line += danger.Render(fmt.Sprintf("%d overdue", d.overdue))
		if includeExam {
			line += accent.Render(examBit)
		}
		return line
	}
	for _, candidate := range []string{build(true, false), build(false, false), build(true, true), build(false, true)} {
		if lipgloss.Width(candidate) <= width {
			return candidate
		}
	}
	return truncatePlain(build(false, true), width)
}

// hstack joins pre-rendered panel blocks horizontally with a base-colored gutter.
func hstack(blocks [][]string, gap int, base lipgloss.Style) []string {
	maxH := 0
	for _, b := range blocks {
		if len(b) > maxH {
			maxH = len(b)
		}
	}
	gutterLine := strings.Repeat(" ", gap)
	gutter := make([]string, 0, maxH)
	for i := 0; i < maxH; i++ {
		gutter = append(gutter, base.Render(gutterLine))
	}
	out := make([]string, maxH)
	for i := range out {
		parts := []string{}
		for j, b := range blocks {
			if i < len(b) {
				parts = append(parts, b[i])
			} else {
				parts = append(parts, base.Render(strings.Repeat(" ", lipgloss.Width(b[0]))))
			}
			if j < len(blocks)-1 {
				parts = append(parts, gutter[i])
			}
		}
		out[i] = strings.Join(parts, "")
	}
	return out
}

// todayBody renders the today timeline body lines.
func todayBody(m Model, w, h int, d homeData) []string {
	bgSurface := lipgloss.Color(m.palette.Surface)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(bgSurface)
	if len(d.todayRows) == 0 {
		return []string{
			"",
			muted.Render(" ✦ nothing due today — a clear day"),
			muted.Render("   press a to add something for later"),
		}
	}

	isDone := func(p, st string) bool { return m.store.IsDoneStatus(p, st) }
	now := d.now
	nowMin := now.Hour()*60 + now.Minute()

	lines := []string{}
	dividerPlaced := false
	for i, t := range d.todayRows {
		if i >= h-1 {
			lines = append(lines, muted.Render(fmt.Sprintf(" +%d more", len(d.todayRows)-i)))
			break
		}
		timeStr := "     "
		timed := false
		if len(t.Due) > len("2006-01-02") {
			timeStr = t.Due[len("2006-01-02")+1:]
			if len(timeStr) > 5 {
				timeStr = timeStr[:5]
			}
			timed = true
		}
		if timed && !dividerPlaced {
			dueMin := int(dueMinutes(t.Due))
			if dueMin > nowMin {
				lines = append(lines, nowDivider(m, w))
				dividerPlaced = true
			}
		}
		done := isDone(t.Project, t.Status)
		color := theme.CourseColor(m.palette, t.Project, projectColorOverride(m, t.Project))
		stripe := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(bgSurface).Render("▌")
		sel := m.homeSel == i && m.focus == focusMain
		bg := bgSurface
		if sel {
			bg = lipgloss.Color(m.palette.SurfaceRaised)
			stripe = lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(bg).Render("▌")
		}
		rowBg := lipgloss.NewStyle().Background(bg)
		mutedTxt := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(bg)

		est := ""
		if t.EstimateMinutes > 0 {
			if t.EstimateMinutes >= 60 {
				est = fmt.Sprintf("~%dh", t.EstimateMinutes/60)
			} else {
				est = fmt.Sprintf("~%dm", t.EstimateMinutes)
			}
		}
		prio := ""
		if !done && (strings.EqualFold(t.Priority, "high") || strings.EqualFold(t.Priority, "urgent")) {
			prio = " ! " + strings.ToUpper(t.Priority[:4])
		}

		label := truncatePlain(t.Title, w-6-9-lipgloss.Width(est)-lipgloss.Width(prio)-3)
		var titleStyle lipgloss.Style
		if done {
			titleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bg)
		} else {
			titleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(bg).Bold(sel)
		}
		mark := "○"
		if done {
			mark = "✔"
		}
		renderedTitle := titleStyle.Render(label)
		if done {
			renderedTitle = strikeTitle(lipgloss.Color(m.palette.Muted), bg, label)
		}
		line := rowBg.Render(" ") + lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(bg).Render(timeStr) +
			rowBg.Render(" ") + stripe + rowBg.Render(mark+" ") + renderedTitle
		rest := rowBg.Render(" ") + mutedTxt.Render(est) + dangerStyleFor(t, m, bg).Render(prio)
		line += rowBg.Render(strings.Repeat(" ", max(0, w-lipglossWidth(line)-lipglossWidth(rest)-1))) + rest + rowBg.Render(" ")
		lines = append(lines, padToWidth(line, w, bg))
	}
	if !dividerPlaced && len(d.todayRows) > 0 {
		lines = append(lines, nowDivider(m, w))
	}
	return lines
}

func nowDivider(m Model, w int) string {
	label := " now "
	rule := w - lipglossWidth(label) - 2
	if rule < 2 {
		rule = 2
	}
	left := rule / 2
	right := rule - left
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(lipgloss.Color(m.palette.Surface))
	return muted.Render("──" + strings.Repeat("─", left) + label + strings.Repeat("─", right))
}

func dangerStyleFor(t *core.Task, m Model, bg lipgloss.Color) lipgloss.Style {
	p := strings.ToLower(t.Priority)
	if p == "urgent" {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Danger)).Background(bg).Bold(true)
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.HeatHot)).Background(bg).Bold(true)
}

func dueMinutes(due string) int {
	sep := -1
	if i := strings.Index(due, "T"); i >= 0 {
		sep = i
	} else if i := strings.Index(due, " "); i >= 0 {
		sep = i
	}
	if sep >= 0 {
		part := due[sep+1:]
		if len(part) >= 5 {
			var hh, mm int
			if _, err := fmt.Sscanf(part, "%02d:%02d", &hh, &mm); err == nil {
				return hh*60 + mm
			}
		}
	}
	return 0
}

// examCardsHeight is the total rows the block-digit cards need.
func examCardsHeight(exams []examCard) int {
	total := 0
	for _, e := range exams {
		total += examCardHeight(e) + 1 // trailing gap
	}
	return total
}

// examCardHeight is the height of one card (name + 3 digit rows + meta + bar).
func examCardHeight(e examCard) int {
	card := 4 // name + 3 digit rows
	if e.weight >= 0 || e.subTotal > 0 {
		card++ // meta line
	}
	if e.subTotal > 0 {
		card++ // topics bar
	}
	return card
}

// examsHeight is what the panel needs to show every exam as a card,
// side-by-side when the width allows it. A half needs >= 34 cells to
// fit digits + date/countdown (5-cell glyphs).
func examsHeight(exams []examCard, w int) int {
	if len(exams) >= 2 && w >= 69 {
		need := 0
		for _, e := range exams {
			if h := examCardHeight(e); h > need {
				need = h
			}
		}
		return need
	}
	return examCardsHeight(exams)
}

// examsBody renders up to two exam countdown cards.
func examsBody(m Model, w, h int, d homeData) []string {
	bgSurface := lipgloss.Color(m.palette.Surface)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(bgSurface)

	if len(d.exams) == 0 {
		return []string{
			"",
			muted.Render(" ✦ no exams yet"),
			muted.Render("   :exam <course> <name> <date>"),
		}
	}
	// Too short for block-digit cards: a single styled line per exam, so
	// every exam stays visible. Cards cost (side-by-side) max(cards) or
	// (stacked) the sum of card heights, + trailing gap.
	if need := examsHeight(d.exams, w); h < need {
		lines := []string{}
		for i, e := range d.exams {
			if i >= h {
				break
			}
			stripe := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.CourseColor(m.palette, e.task.Project, projectColorOverride(m, e.task.Project)))).Background(bgSurface).Render("▌")
			txt := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(bgSurface)
			name := truncatePlain(e.task.Title, w-16)
			inTxt := "today"
			switch {
			case e.daysLeft == 1:
				inTxt = "tomorrow"
			case e.daysLeft > 1:
				inTxt = fmt.Sprintf("in %d days", e.daysLeft)
			}
			lines = append(lines, " "+stripe+" "+txt.Render(name)+muted.Render(" — "+inTxt))
		}
		if extra := len(d.exams) - len(lines); extra > 0 && len(lines) < h {
			lines = append(lines, muted.Render(fmt.Sprintf(" +%d more", extra)))
		}
		return lines
	}
	// Side-by-side when the panel is wide enough for two readable
	// cards; otherwise the stacked layout.
	if len(d.exams) >= 2 && w >= 69 {
		half := (w - 1) / 2
		rightW := w - half - 1
		left := singleExamCard(m, d.exams[0], half, bgSurface)
		right := singleExamCard(m, d.exams[1], rightW, bgSurface)
		lines := []string{}
		for i := 0; i < len(left); i++ {
			r := ""
			if i < len(right) {
				r = right[i]
			}
			lines = append(lines, hstack([][]string{{left[i]}, {r}}, 1, lipgloss.NewStyle().Background(bgSurface))[0])
		}
		return lines
	}
	lines := []string{}
	for _, e := range d.exams {
		card := singleExamCard(m, e, w, bgSurface)
		lines = append(lines, card...)
		lines = append(lines, "")
	}
	return lines
}

// singleExamCard renders one exam countdown card at width w: name, block
// digits with the date and "in N days" beside them, then weight/topics.
func singleExamCard(m Model, e examCard, w int, bgSurface lipgloss.Color) []string {
	color := theme.CourseColor(m.palette, e.task.Project, projectColorOverride(m, e.task.Project))
	name := truncatePlain(e.task.Title, w-1)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(bgSurface)
	lines := []string{m.styles.header.Render(truncatePlain(" "+name, w))}

	dueDay := parseDayStart(e.task.Due, time.Local)
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
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(bgSurface).Bold(true)
	mutedTxt := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(bgSurface).Bold(true)
	digits := gauges.Digits(fmt.Sprintf("%dd", e.daysLeft))
	right := []string{
		mutedTxt.Render(dateTxt),
		accent.Render(inTxt),
		"",
	}
	for row := 0; row < 3; row++ {
		line := "  " + accent.Render(digits[row]) + "  " + right[row]
		lines = append(lines, line)
	}
	// Weight + topics covered.
	meta := ""
	// Weight is a percent of the course (e.g. 30 = 30%).
	if e.weight >= 0 {
		meta += fmt.Sprintf("weight %.0f%%", e.weight)
	}
	if e.subTotal > 0 {
		if meta != "" {
			meta += "  ·  "
		}
		meta += fmt.Sprintf("topics %d/%d", e.subDone, e.subTotal)
	}
	if meta != "" {
		lines = append(lines, muted.Render(" "+meta))
	}
	if e.subTotal > 0 {
		fill, track := gauges.Bar(float64(e.subDone)/float64(e.subTotal), clampInt(w/3, 8, 12))
		bar := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(bgSurface).Render(fill) +
			muted.Render(track)
		lines = append(lines, " "+bar)
	}
	for i, l := range lines {
		if pad := w - lipgloss.Width(l); pad > 0 {
			lines[i] = l + lipgloss.NewStyle().Background(bgSurface).Render(strings.Repeat(" ", pad))
		}
	}
	return lines
}

// coursesBody renders one row per course: stripe, name, bar, percent.
func coursesBody(m Model, w, h int, d homeData) []string {
	if h < 1 {
		h = 1
	}
	bgSurface := lipgloss.Color(m.palette.Surface)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(bgSurface)

	if len(d.courses) == 0 {
		return []string{"", muted.Render(" ✦ no courses yet — add one with `mtm project add`")}
	}
	shown := d.courses
	if len(shown) > h-1 {
		shown = shown[:h-1]
	}
	// Fixed columns: stripe, leaf name (padded to the widest shown
	// name, clamped), bar, percent — leaf names (`dbms`, not
	// `college/dbms`), so segments line up in a column.
	nameW := 6
	for _, c := range shown {
		if n := len([]rune(leafName(c.name))); n > nameW {
			nameW = n
		}
	}
	nameW = clampInt(nameW, 6, 14)
	const barW = 8
	lines := []string{}
	for _, c := range shown {
		name := padRightPlain(truncatePlain(leafName(c.name), nameW), nameW)
		stripe := lipgloss.NewStyle().Foreground(lipgloss.Color(c.color)).Background(bgSurface).Render("▌")
		fill, track := gauges.Bar(easedFrac(m, c.frac), barW)
		bar := lipgloss.NewStyle().Foreground(lipgloss.Color(c.color)).Background(bgSurface).Render(fill) +
			muted.Render(track)
		pct := fmt.Sprintf("%3.0f%%", c.frac*100)
		lines = append(lines, " "+stripe+" "+name+" "+bar+" "+muted.Render(pct))
	}
	if len(d.courses) > len(shown) {
		lines = append(lines, muted.Render(fmt.Sprintf(" +%d more", len(d.courses)-len(shown))))
	}
	return lines
}

// heatmapBody renders the completion heatmap (Mon..Sun rows) at
// half-block granularity: 2-cell × 1-row cells (▀ on surface, so rows
// don't blur together), month labels above, weekday labels on the
// left, a `less ▪▪▪▪ more` legend — scaled to the width, up to 52
// weeks.
func heatmapBody(m Model, w, h int, d homeData) []string {
	bgSurface := lipgloss.Color(m.palette.Surface)

	maxV := 0
	for _, v := range d.heatmap {
		if v > maxV {
			maxV = v
		}
	}
	level := func(v int) int {
		if v <= 0 || maxV == 0 {
			return 0
		}
		switch {
		case v == 1:
			return 1
		case v == 2:
			return 2
		case v == 3:
			return 3
		default:
			return 4
		}
	}
	cell := func(v int) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Heatmap[level(v)])).Background(bgSurface).Render("▀▀")
	}
	cellGap := lipgloss.NewStyle().Background(bgSurface).Render(" ")

	const labelW = 5
	const colW = 3 // 2-cell cell + 1-cell gap
	weeks := (w - labelW + 1) / colW
	if weeks > 52 {
		weeks = 52
	}
	if weeks < 4 {
		weeks = 4
	}

	// Month labels above the grid. Column hc covers the 7 days ending
	// (51-hc) weeks before today; label a column when its span opens a
	// new month.
	now := d.now
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	monthAt := func(hc int) string {
		start := todayStart.AddDate(0, 0, -(51-(hc))*7-6)
		return start.Format("Jan")
	}
	monthRow := ""
	{
		prev := ""
		x := labelW
		for hc := 52 - weeks; hc < 52; hc++ {
			mth := monthAt(hc)
			if mth != prev {
				pad := x - lipgloss.Width(monthRow)
				if pad > 0 {
					monthRow += lipgloss.NewStyle().Background(bgSurface).Render(strings.Repeat(" ", pad))
				}
				monthRow += lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bgSurface).Render(mth)
				prev = mth
			}
			x += colW
		}
		pad := w - lipgloss.Width(monthRow)
		if pad > 0 {
			monthRow += lipgloss.NewStyle().Background(bgSurface).Render(strings.Repeat(" ", pad))
		}
	}

	rowLabel := func(i int) string {
		names := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
		label := ""
		if i%2 == 0 {
			label = names[i]
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bgSurface).Width(4).Render(label)
	}

	lines := []string{}
	if h >= 9 {
		lines = append(lines, monthRow)
	}
	for row := 0; row < 7; row++ {
		line := " " + rowLabel(row) + " "
		for hc := 52 - weeks; hc < 52; hc++ {
			line += cell(d.heatmap[row*52+hc])
			if hc != 51 {
				line += cellGap
			}
		}
		if pad := w - lipgloss.Width(line); pad > 0 {
			line += lipgloss.NewStyle().Background(bgSurface).Render(strings.Repeat(" ", pad))
		}
		lines = append(lines, line)
	}
	// Legend: less → more.
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bgSurface)
	legendCell := func(i int) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Heatmap[i])).Background(bgSurface).Render("▪") + " "
	}
	legend := strings.Repeat(" ", labelW) + muted.Render("less ") + legendCell(1) + legendCell(2) + legendCell(3) + legendCell(4) + muted.Render("more")
	lines = append(lines, legend)
	// If the panel is taller than the content, center it vertically so
	// the slab below/above reads as intentional padding.
	if h > len(lines) {
		top := (h - len(lines)) / 2
		padded := make([]string, 0, h)
		for i := 0; i < top; i++ {
			padded = append(padded, lipgloss.NewStyle().Background(bgSurface).Render(strings.Repeat(" ", w)))
		}
		padded = append(padded, lines...)
		lines = padded
	}
	return lines
}

// thisWeekBody renders the 7-day study-hours bar chart: hours above
// each bar, day letters (M T W T F S S) below, and the total — scaled
// to the available height.
func thisWeekBody(m Model, w, h int, d homeData) []string {
	bgSurface := lipgloss.Color(m.palette.Surface)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(bgSurface)
	bright := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.BrightText)).Background(bgSurface).Bold(true)

	col := w / 7
	if col < 3 {
		col = 3
	}
	barRows := h - 4
	if barRows < 1 {
		barRows = 1
	}
	maxV := 0.0
	for _, v := range d.weekHours {
		if v > maxV {
			maxV = v
		}
	}
	heightOf := func(v float64) int {
		if maxV <= 0 || v <= 0 {
			return 0
		}
		r := int(v / maxV * float64(barRows))
		if r < 1 && v > 0 {
			r = 1
		}
		return r
	}
	hoursLabel := func(v float64) string {
		if v <= 0 {
			return "0"
		}
		if v < 1 {
			return fmt.Sprintf("%dm", int(v*60))
		}
		s := fmt.Sprintf("%.1f", v)
		s = strings.TrimSuffix(s, ".0")
		return s + "h"
	}
	center := func(s string, width int) string {
		if len(s) >= width {
			return s[:width]
		}
		return strings.Repeat(" ", (width-len(s))/2) + s
	}
	// Value row (hours above each bar).
	valueLine := " "
	for i := 0; i < 7; i++ {
		valueLine += muted.Render(center(hoursLabel(d.weekHours[i]), col))
	}

	// Bar rows, top → down. Bar glyphs are 2 cells wide, centered.
	accent := lipgloss.Color(m.palette.Accent)
	barGlyph := lipgloss.NewStyle().Foreground(accent).Background(bgSurface).Render("██")
	barLine := func(r int) string {
		line := " "
		for i := 0; i < 7; i++ {
			pad := (col - 2) / 2
			fill := strings.Repeat(" ", col)
			if heightOf(d.weekHours[i]) >= r {
				fill = strings.Repeat(" ", pad) + barGlyph + strings.Repeat(" ", col-pad-2)
			}
			line += lipgloss.NewStyle().Background(bgSurface).Render(fill)
		}
		return line
	}
	// Day letters.
	letters := []string{"M", "T", "W", "T", "F", "S", "S"}
	letterLine := " "
	for i := 0; i < 7; i++ {
		letterLine += muted.Render(center(letters[i], col))
	}
	// Total row.
	total := fmt.Sprintf("%.1f h", d.weekTotal)
	totalLine := " " + bright.Render(total) + " " + muted.Render("study hours · last 7 days")

	lines := []string{valueLine}
	for r := barRows; r >= 1; r-- {
		lines = append(lines, barLine(r))
	}
	lines = append(lines, letterLine)
	lines = append(lines, totalLine)
	return lines
}
