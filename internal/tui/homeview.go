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

// heatAbsorbCap bounds how tall UP NEXT may grow absorbing leftover rows:
// content rows plus enough headroom to keep the panel's blank ratio under
// ~0.35 (the frame invariant), never below the 4-row empty state.
func heatAbsorbCap(contentRows int) int {
	if contentRows < 1 {
		return 4
	}
	if m := int(float64(contentRows) / 0.65); m > 4 {
		return m
	}
	return 4
}

// homeRect is one Home panel's frame, relative to the body (x, y).
type homeRect struct {
	title      string
	x, y, w, h int
}

// homeLayout computes the band rects. Band order (both modes): TODAY,
// EXAMS (own band), COURSES, THIS WEEK, UP NEXT, HEATMAP (last band; it
// takes only its natural height — month row + 7 data rows + legend, plus
// the panel's title and spacer rows). Below 110 cells every band is full
// width; at w >= 110 COURSES (left) and THIS WEEK (right, content-sized
// chart panel including the totals line) share one band side by side.
// Leftover rows go to UP NEXT, capped so its blank ratio stays under the
// frame invariant; rows beyond that are unassigned surface padding.
// When the height is too small the panels shrink in order: THIS WEEK →
// EXAMS → COURSES → row1 → UP NEXT drops out → the heatmap degrades.
func homeLayout(m Model, w, h int) [][]homeRect {
	d := m.homeDataCached()
	gutter := 1
	sparse := len(m.store.Tasks()) <= 3
	stripRows := 0
	if sparse {
		stripRows = 1
	}
	avail := h - 3 - stripRows // date line + blank + greeting
	contentRows := len(todayBody(m, w-2, 1<<30, d))

	// Natural (unclipped) panel contents.
	coursesNat := len(coursesBody(m, w-2, 1<<30, d))
	coursesH := clampInt(coursesNat+2, 6, 12)
	upIdeal := len(d.upNext) + 2
	if upIdeal < 4 {
		upIdeal = 4
	}
	// Show up to 8 tasks at tall terminals: 8 task rows + panel chrome.
	if upIdeal > 10 {
		upIdeal = 10
	}
	heatBand, heatMin := 11, 9 // 9 content rows + title + spacer
	chartNat := 13             // value row + 8 bar rows + axis + totals (+2)
	chartMin := 6              // value row + 1 bar row + axis + totals (+2)

	// Row 1: TODAY, content-sized (its 60% fill bound keeps the frame
	// invariant), never stretching past 40% of the body.
	maxRow1 := (avail * 40) / 100
	fillLimit := int(float64(contentRows) / 0.6)
	if contentRows > 0 {
		fillLimit++ // +1: the rounded-up height keeps fill ≥ 60%
	}
	fillLimit += 2
	if fillLimit < 6 {
		fillLimit = 6
	}
	row1H := maxInt(contentRows+2, 6)
	if row1H > fillLimit {
		row1H = fillLimit
	}
	if row1H > maxRow1 {
		row1H = maxRow1
	}

	examsH := 0
	if len(d.exams) > 0 {
		examsH = examsContentRows(d.exams, w-2) + 2
		if examsH < 3 {
			examsH = 3
		}
		if examsH > 10 {
			examsH = 10
		}
	}

	sideBySide := w >= 110

	weekH := chartNat
	if sideBySide {
		// Both panels share one band. The chart keeps ≥4 interior rows
		// (1 bar row); the band is capped so the courses panel's blank
		// ratio stays under the invariant bound:
		// (h - coursesNat - 1)/h ≤ 0.35 → h ≤ (coursesNat+1)/0.65.
		weekH = maxInt(coursesH, 7)
		if capRows := int(float64(coursesNat+1) / 0.65); weekH > capRows {
			weekH = capRows
		}
		if weekH < 6 {
			weekH = 6
		}
	}
	heatH := heatBand
	total := func() int {
		if sideBySide {
			mid := coursesH
			if weekH > mid {
				mid = weekH
			}
			return row1H + examsH + mid + heatH
		}
		return row1H + examsH + coursesH + weekH + heatH
	}
	// Starvation: shrink THIS WEEK, then EXAMS, then COURSES, then row1;
	// the heatmap degrades to its floor last (its content drops the
	// month/legend rows), then everything clips at the frame edge.
	sideBySide = w >= 110
	for total() > avail {
		switch {
		case weekH > chartMin:
			weekH--
		case examsH > 3:
			examsH--
		case coursesH > 6:
			coursesH--
		case row1H > 5:
			row1H--
		case heatH > heatMin:
			heatH--
		case examsH > 0:
			examsH = 0
		case heatH > 4:
			heatH--
		default:
			heatH--
		}
	}

	// Leftover rows go to UP NEXT (show up to 8 tasks' worth of
	// height; the panel absorbs the slack). Rows beyond the blank-ratio
	// bound are unassigned surface padding.
	midH := weekH
	if w < 110 {
		midH = coursesH + weekH
	} else if coursesH > weekH {
		midH = coursesH
	}
	upH := avail - row1H - examsH - midH - heatH
	if upH > upIdeal {
		upH = upIdeal
	}
	if maxUp := heatAbsorbCap(len(d.upNext)); upH > maxUp {
		upH = maxUp
	}
	if upH < 0 {
		upH = 0
	}

	b := [][]homeRect{}
	y := 0
	band := func(title string, hh int) {
		if hh <= 0 {
			return
		}
		b = append(b, []homeRect{{title, 0, y, w, hh}})
		y += hh
	}
	band("TODAY", row1H)
	band("EXAMS", examsH)
	if w >= 110 {
		const chartW = 42
		coursesW := w - gutter - chartW
		if coursesW < 44 {
			coursesW = 44
		}
		b = append(b, []homeRect{{"COURSES", 0, y, coursesW, midH}, {"THIS WEEK", coursesW + gutter, y, chartW, midH}})
		y += midH
	} else {
		band("COURSES", coursesH)
		band("THIS WEEK", weekH)
	}
	band("UP NEXT", upH)
	band("HEATMAP", heatH)
	return b
}

// sparseHint is the dense-key hint line shown under Home on tiny datasets;
// keys are looked up in the keymap so config overrides stay accurate.
func sparseHint(m Model) string {
	return fmt.Sprintf(" %s add · %s focus · %s palette · %s help",
		m.keymap.Seq(aAdd), m.keymap.Seq(aFocus), m.keymap.Seq(aPalette), m.keymap.Seq(aHelp))
}

// renderHomeBody renders the Home dashboard (inner width × inner height).
func renderHomeBody(m Model, w, h int) []string {
	d := m.homeDataCached()

	bgBase := lipgloss.Color(m.palette.Base)
	base := lipgloss.NewStyle().Background(bgBase)

	sparse := len(m.store.Tasks()) <= 3

	stripRows := 0
	if sparse {
		stripRows = 1
	}
	lines := []string{}
	lines = append(lines, homeHeaderLine(m, w, d))
	lines = append(lines, "")
	lines = append(lines, homeGreetingLine(m, d, w))
	if h-len(lines)-stripRows < 8 {
		if sparse {
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bgBase).Render(sparseHint(m)))
		}
		return lines
	}

	gutter := 1
	bands := homeLayout(m, w, h)
	for _, band := range bands {
		rendered := make([][]string, 0, len(band))
		for _, r := range band {
			var body []string
			switch r.title {
			case "TODAY":
				body = todayBody(m, r.w-2, r.h-2, d)
			case "EXAMS":
				body = examsBody(m, r.w-2, r.h-2, d)
			case "COURSES":
				body = coursesBody(m, r.w-2, r.h-2, d)
			case "UP NEXT":
				body = upNextBody(m, r.w-2, r.h-2, d)
			case "THIS WEEK":
				body = thisWeekBody(m, r.w-2, r.h-2, d)
			case "HEATMAP":
				body = heatmapBody(m, r.w-2, r.h-2, d)
			}
			rendered = append(rendered, splitPane(m, r.title, r.w, r.h, m.focus == focusMain && r.title == "TODAY", body))
		}
		bandLines := rendered[0]
		if len(rendered) > 1 {
			bandLines = hstack(rendered, gutter, base)
		}
		lines = append(lines, bandLines...)
	}
	if sparse {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bgBase).Render(sparseHint(m)))
	}
	return lines
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
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
			muted.Render(fmt.Sprintf("   press %s to add something for later", m.keymap.Seq(aAdd))),
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

// examsContentRows is what the panel needs at interior width w: the
// nearest exam as the countdown card, plus one compact line per other exam.
func examsContentRows(exams []examCard, w int) int {
	if len(exams) == 0 {
		return 0
	}
	need := examCardHeight(exams[0])
	if len(exams) > 1 {
		need += len(exams) - 1 // compact one-liners beneath the card
	}
	return need
}

// examsBody renders the nearest exam as the block-digit countdown card,
// with the other exams as compact one-line rows beneath it. When the
// interior height is too small for the card, every exam falls back to
// compact rows so each exam stays visible.
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
	inText := func(daysLeft int) string {
		switch {
		case daysLeft == 0:
			return "today"
		case daysLeft == 1:
			return "tomorrow"
		default:
			return fmt.Sprintf("in %d days", daysLeft)
		}
	}
	if h < examsContentRows(d.exams, w) {
		// Compact fallback: two lines per exam (name + countdown, then
		// weight/topics) when they fit; one line each otherwise, and a
		// trailing "+N more" so no exam disappears silently.
		metaLine := func(e examCard) string {
			meta := ""
			if e.weight >= 0 {
				meta += fmt.Sprintf("weight %.0f%%", e.weight)
			}
			if e.subTotal > 0 {
				if meta != "" {
					meta += "  ·  "
				}
				meta += fmt.Sprintf("topics %d/%d", e.subDone, e.subTotal)
			}
			return meta
		}
		lines := []string{}
		placed := 0
		for _, e := range d.exams {
			meta := metaLine(e)
			need := 1
			if meta != "" {
				need = 2
			}
			if len(lines)+need > h {
				break
			}
			stripe := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.CourseColor(m.palette, e.task.Project, projectColorOverride(m, e.task.Project)))).Background(bgSurface).Render("▌")
			txt := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(bgSurface)
			name := truncatePlain(e.task.Title, w-16)
			lines = append(lines, " "+stripe+" "+txt.Render(name)+muted.Render(" — "+inText(e.daysLeft)))
			if meta != "" {
				lines = append(lines, "   "+muted.Render(meta))
			}
			placed++
		}
		if extra := len(d.exams) - placed; extra > 0 && len(lines) < h {
			lines = append(lines, muted.Render(fmt.Sprintf(" +%d more", extra)))
		}
		return lines
	}
	// Nearest exam as the countdown card; the rest as compact one-line
	// rows beneath it.
	lines := []string{}
	lines = append(lines, singleExamCard(m, d.exams[0], w, bgSurface)...)
	shown := 1
	for _, e := range d.exams[1:] {
		if len(lines) >= h {
			break
		}
		stripe := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.CourseColor(m.palette, e.task.Project, projectColorOverride(m, e.task.Project)))).Background(bgSurface).Render("▌")
		txt := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(bgSurface)
		nameBudget := w - 5 - lipgloss.Width(" — "+inText(e.daysLeft))
		if e.weight >= 0 && nameBudget > 12 {
			nameBudget -= lipgloss.Width(fmt.Sprintf(" · %.0f%%", e.weight))
		}
		if nameBudget < 8 {
			nameBudget = 8
		}
		name := truncatePlain(e.task.Title, nameBudget)
		line := " " + stripe + " " + txt.Render(name) + muted.Render(" — "+inText(e.daysLeft))
		if e.weight >= 0 && lipgloss.Width(line)+lipgloss.Width(fmt.Sprintf(" · %.0f%%", e.weight)) <= w {
			line += muted.Render(fmt.Sprintf(" · %.0f%%", e.weight))
		}
		lines = append(lines, padToWidth(line, w, bgSurface))
		shown++
	}
	if extra := len(d.exams) - shown; extra > 0 && len(lines) < h {
		lines = append(lines, muted.Render(fmt.Sprintf(" +%d more", extra)))
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
		lines = append(lines, muted.Render("  "+meta))
	}
	if e.subTotal > 0 {
		fill, track := gauges.Bar(float64(e.subDone)/float64(e.subTotal), clampInt(w/3, 8, 12))
		bar := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(bgSurface).Render(fill) +
			muted.Render(track)
		lines = append(lines, "  "+bar)
	}
	for i, l := range lines {
		if pad := w - lipgloss.Width(l); pad > 0 {
			lines[i] = l + lipgloss.NewStyle().Background(bgSurface).Render(strings.Repeat(" ", pad))
		}
	}
	return lines
}

// coursesBody renders one row per course: stripe, leaf name (padded to the
// widest shown name, clamped), a fixed-width thin bar (lower-half-block
// glyphs so rows never merge into slabs), percent, the next-exam countdown
// chip and the hours logged this week. Leaf names (`dbms`, not
// `college/dbms`) so segments line up in a column. At most h rows are used;
// the panel shrinks to its content.
func coursesBody(m Model, w, h int, d homeData) []string {
	if h < 1 {
		h = 1
	}
	bgSurface := lipgloss.Color(m.palette.Surface)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(bgSurface)
	subtle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.SurfaceStrong)).Background(bgSurface)

	if len(d.courses) == 0 {
		return []string{
			"",
			muted.Render(" ✦ no courses yet"),
			muted.Render("   mtm project add <name>"),
		}
	}
	shown := d.courses
	overflow := 0
	if len(shown) > h {
		overflow = len(shown) - (h - 1)
		shown = shown[:h-1]
	}
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
		fill, track := gauges.ThinBar(easedFrac(m, c.frac), barW)
		bar := lipgloss.NewStyle().Foreground(lipgloss.Color(c.color)).Background(bgSurface).Render(fill) +
			subtle.Render(track)
		pct := fmt.Sprintf("%3.0f%%", c.frac*100)
		// Next-exam chip: "✦6d" / "✦today"; blank slot when no exam is open.
		chip := "     "
		if c.nextExam != nil {
			t := ""
			switch {
			case c.examDays == 0:
				t = "today"
			case c.examDays > 0:
				t = fmt.Sprintf("%dd", c.examDays)
			}
			if len(t) > 4 {
				t = t[:4]
			}
			chip = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(bgSurface).Bold(true).
				Render(padRightPlain("✦"+t, 5))
		}
		hours := muted.Render("     ")
		if c.weekHours > 0 {
			h := fmt.Sprintf("%.1fh", c.weekHours)
			if len(h) > 5 {
				h = h[:5]
			}
			hours = muted.Render(padRightPlain(h, 5))
		}
		lines = append(lines, " "+stripe+" "+name+" "+bar+" "+muted.Render(pct)+" "+chip+" "+hours)
	}
	if overflow > 0 {
		lines = append(lines, muted.Render(fmt.Sprintf(" +%d more", overflow)))
	}
	return lines
}

// upNextBody renders the next open tasks by due date then priority:
// course stripe, title, relative due (heat-colored when hot).
func upNextBody(m Model, w, h int, d homeData) []string {
	if h < 1 {
		h = 1
	}
	bgSurface := lipgloss.Color(m.palette.Surface)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(bgSurface)

	if len(d.upNext) == 0 {
		return []string{
			"",
			muted.Render(" ✦ nothing open — a clear runway"),
		}
	}
	now := d.now
	shown := d.upNext
	overflow := 0
	if len(shown) > h {
		overflow = len(shown) - (h - 1)
		shown = shown[:h-1]
	}
	lines := []string{}
	for _, t := range shown {
		color := theme.CourseColor(m.palette, t.Project, projectColorOverride(m, t.Project))
		stripe := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(bgSurface).Render("▌")
		label, heat := "", gauges.HeatCalm
		if t.Due != "" {
			label, heat = gauges.DueLabel(t.Due, now)
		}
		dueCell := ""
		if label != "" {
			dc := lipgloss.NewStyle().Background(bgSurface)
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
			dueCell = dc.Render(label)
		}
		dueW := lipgloss.Width(dueCell)
		examMark := ""
		if strings.EqualFold(t.Kind, core.KindExam) {
			examMark = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(bgSurface).Bold(true).Render("✦ ")
		}
		markW := lipgloss.Width(examMark)
		budget := w - 4 - markW - dueW - 2
		if budget < 8 {
			budget = 8
		}
		title := truncatePlain(t.Title, budget)
		titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(bgSurface)
		line := " " + stripe + " " + examMark + titleStyle.Render(title)
		// Right-align the due cell on the panel edge.
		if dueCell != "" {
			pad := w - lipgloss.Width(line) - dueW - 1
			if pad < 1 {
				line = clipStyled(line, w-dueW-2)
				pad = 1
			}
			line += lipgloss.NewStyle().Background(bgSurface).Render(strings.Repeat(" ", pad)) + dueCell + " "
		}
		lines = append(lines, padToWidth(line, w, bgSurface))
	}
	if overflow > 0 {
		lines = append(lines, muted.Render(fmt.Sprintf(" +%d more", overflow)))
	}
	return lines
}

// Heatmap geometry: weekday label column, then one cell per week with a
// 1-cell gap. The grid never stretches — it shows the configured number of
// weeks (default 26) and leaves the rest of the panel blank (or a short
// stats strip when the panel is wide enough).
const (
	heatLabelW = 5 // "Mon" + two spaces
	heatColW   = 2 // cell + 1-cell gap
)

func heatGridWidth(weeks int) int { return heatLabelW + weeks*heatColW - 1 }

type monthPlacement struct {
	x    int
	text string
}

// heatmapMonthPlacements decides where month labels land in the month row.
// A label starts on the column whose week opens a new month; it is dropped
// (never clipped) when it would overlap the previous label or run past w.
func heatmapMonthPlacements(now time.Time, weeks, w int) []monthPlacement {
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	out := []monthPlacement{}
	prevMonth := time.Month(0)
	prevEnd := 0
	for ci := 0; ci < weeks; ci++ {
		ref := todayStart.AddDate(0, 0, -7*(weeks-1-ci))
		weekStart := ref.AddDate(0, 0, -int(ref.Weekday()+6)%7)
		mth := weekStart.Month()
		if ci != 0 && mth == prevMonth {
			continue
		}
		prevMonth = mth
		text := weekStart.Format("Jan")
		x := heatLabelW + ci*heatColW
		if ci != 0 && x < prevEnd+1 {
			continue // would touch the previous label — drop it
		}
		if x+len(text) > w {
			continue // would run past the panel — drop it
		}
		out = append(out, monthPlacement{x, text})
		prevEnd = x + len(text)
	}
	return out
}

// heatStats summarizes the visible window: best single day, current streak
// (consecutive days with ≥1 completion, today or yesterday anchor) and
// completions in the period.
type heatStats struct {
	bestDay   string
	bestCount int
	streak    int
	total     int
}

func heatStatsFor(d homeData, weeks int) heatStats {
	st := heatStats{}
	now := d.now
	for i := 0; i < weeks*7 && i < len(d.dayCounts); i++ {
		v := d.dayCounts[i]
		st.total += v
		if v > st.bestCount {
			st.bestCount = v
			day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -i)
			st.bestDay = day.Format("Mon 2 Jan")
		}
	}
	i := 0
	if st.total > 0 && d.dayCounts[0] == 0 {
		i = 1 // today is blank — the streak ending yesterday still counts
	}
	for ; i < len(d.dayCounts); i++ {
		if d.dayCounts[i] == 0 {
			break
		}
		st.streak++
	}
	return st
}

// heatmapBody renders the completion heatmap (Mon..Sun rows) as exactly
// seven single-cell-high data rows with a month-label row above and a
// legend row below. The grid shows the last cfg.Heatmap.Weeks weeks
// (default 26), left-aligned after the weekday labels; when the panel is
// wide a short stats strip sits to the right of the grid.
func heatmapBody(m Model, w, h int, d homeData) []string {
	bgSurface := lipgloss.Color(m.palette.Surface)

	weeks := clampInt(m.cfg.Heatmap.Weeks, 4, 52)
	gridW := heatGridWidth(weeks)
	firstCol := 52 - weeks

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
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Heatmap[level(v)])).Background(bgSurface).Render("▄")
	}
	cellGap := lipgloss.NewStyle().Background(bgSurface).Render(" ")
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bgSurface)

	// Month labels above the grid; each sits on the column whose week
	// opens its month. Labels are dropped (never clipped).
	monthRow := ""
	{
		for _, p := range heatmapMonthPlacements(d.now, weeks, w) {
			if pad := p.x - lipgloss.Width(monthRow); pad > 0 {
				monthRow += lipgloss.NewStyle().Background(bgSurface).Render(strings.Repeat(" ", pad))
			}
			monthRow += muted.Render(p.text)
		}
		if pad := w - lipgloss.Width(monthRow); pad > 0 {
			monthRow += lipgloss.NewStyle().Background(bgSurface).Render(strings.Repeat(" ", pad))
		}
	}

	rowLabel := func(i int) string {
		names := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
		label := ""
		if i%2 == 0 {
			label = names[i]
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bgSurface).Width(heatLabelW).Render(label + strings.Repeat(" ", heatLabelW-lipgloss.Width(label)))
	}

	// Stats strip to the right of the grid when the panel is wide enough.
	stats := heatStatsFor(d, weeks)
	statsX := gridW + 4
	strip := []string{}
	if w >= statsX+22 && stats.total >= 0 {
		bright := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.BrightText)).Background(bgSurface).Bold(true)
		lbl := func(s string) string { return muted.Render(s) }
		if stats.bestCount > 0 {
			strip = append(strip, lbl("best day  ")+bright.Render(fmt.Sprintf("%s (%d)", stats.bestDay, stats.bestCount)))
		}
		strip = append(strip, lbl("streak    ")+bright.Render(fmt.Sprintf("%d days", stats.streak)))
		strip = append(strip, lbl("total     ")+bright.Render(fmt.Sprintf("%d this period", stats.total)))
	}

	// If the band is shorter than the natural 9 rows, drop the month
	// row first, then the legend — the 7 data rows always win.
	withMonth, withLegend := true, true
	switch {
	case h < 8:
		withMonth = false
	case h < 9:
		withLegend = false
	}
	lines := []string{}
	if withMonth {
		lines = append(lines, monthRow)
	}
	for row := 0; row < 7; row++ {
		line := rowLabel(row)
		for ci := 0; ci < weeks; ci++ {
			line += cell(d.heatmap[row*52+firstCol+ci])
			if ci != weeks-1 {
				line += cellGap
			}
		}
		// Stats strip: three lines centered on the data rows.
		stripLine := -1
		switch row {
		case 1:
			stripLine = 0
		case 3:
			stripLine = 1
		case 5:
			stripLine = 2
		}
		if stripLine >= 0 && stripLine < len(strip) {
			if pad := statsX - lipgloss.Width(line); pad > 0 {
				line += lipgloss.NewStyle().Background(bgSurface).Render(strings.Repeat(" ", pad))
			}
			line += strip[stripLine]
		}
		if pad := w - lipgloss.Width(line); pad > 0 {
			line += lipgloss.NewStyle().Background(bgSurface).Render(strings.Repeat(" ", pad))
		}
		lines = append(lines, line)
	}

	// Legend: less → more.
	legend := strings.Repeat(" ", heatLabelW) + muted.Render("less ")
	legendCell := func(i int) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Heatmap[i])).Background(bgSurface).Render("▪") + " "
	}
	legend += legendCell(1) + legendCell(2) + legendCell(3) + legendCell(4) + muted.Render("more")
	if pad := w - lipgloss.Width(legend); pad > 0 {
		legend += lipgloss.NewStyle().Background(bgSurface).Render(strings.Repeat(" ", pad))
	}
	if withLegend {
		lines = append(lines, legend)
	}

	// If the panel is taller than the content, center it vertically so
	// the gap above/below reads as intentional padding.
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

// thisWeekBody renders the shared 7-day study-hours bar chart (value
// labels, bars, day letters, totals line) capped at 8 bar rows.
func thisWeekBody(m Model, w, h int, d homeData) []string {
	bgSurface := lipgloss.Color(m.palette.Surface)
	surfaceBG := lipgloss.NewStyle().Background(bgSurface)
	muted := func(s string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(bgSurface).Render(s)
	}
	bright := func(s string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.BrightText)).Background(bgSurface).Bold(true).Render(s)
	}
	accent := func(s string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(bgSurface).Render(s)
	}
	bg := func(s string) string { return surfaceBG.Render(s) }

	// Today's column index, counting back six days from today.
	todayIdx := 6
	letters := []string{"M", "T", "W", "T", "F", "S", "S"}
	weekdayLetters := func() []string {
		// The trailing column is always today; walk the weekday back.
		now := d.now
		letters[todayIdx] = strings.ToUpper(now.Format("Mon")[:1])
		for i := todayIdx - 1; i >= 0; i-- {
			day := now.AddDate(0, 0, i-todayIdx)
			letters[i] = strings.ToUpper(day.Format("Mon")[:1])
		}
		return letters
	}()

	cfg := gauges.BarChartConfig{
		Values:      d.weekHours[:],
		Labels:      weekdayLetters,
		Width:       w,
		Height:      h,
		MaxBarRows:  8,
		BarWidth:    3,
		Today:       todayIdx,
		ShowTotal:   true,
		ShowAverage: true,
		Accent:      accent,
		Bar:         accent,
		Muted:       muted,
		Bright:      bright,
		Bg:          bg,
	}
	return gauges.RenderBarChart(cfg)
}
