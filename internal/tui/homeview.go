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

// homeRect is one Home panel's frame, relative to the body (x, y).
type homeRect struct {
	title      string
	x, y, w, h int
}

// homeLayout computes the band rects: content-first heights, never a panel
// stretched beyond its content (the heatmap and THIS WEEK absorb the
// leftover). Stacked below 100 cells; side-by-side bands above.
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
	coursesBodyNat := coursesBody(m, w-2, 1<<30, d)
	coursesH := len(coursesBodyNat) + 2
	if coursesH < 4 {
		coursesH = 4
	}
	if coursesH > 12 {
		coursesH = 12
	}
	upNat := len(d.upNext) + 2
	if upNat < 4 {
		upNat = 4
	}
	if upNat > 8 {
		upNat = 8
	}
	examsNat := 5
	if len(d.exams) > 0 {
		examsNat = examsHeight(d.exams, w-2) + 2
		if examsNat < 5 {
			examsNat = 5
		}
		if examsNat > avail-6 {
			examsNat = avail - 6
		}
		if examsNat < 3 {
			examsNat = 3
		}
	}

	if w >= 100 {
		// Band 1: TODAY | EXAMS.
		var todayW int
		if w >= 150 {
			todayW = (w*56 + 99) / 100
		} else {
			todayW = (w*46 + 99) / 100
		}
		examsW := w - todayW - gutter
		maxRow1 := (avail * 40) / 100
		row1H := maxInt(contentRows+2, 6)
		if row1H > maxRow1 {
			row1H = maxRow1
		}
		if row1H < 6 {
			row1H = 6
		}
		// Exams may grow the band, but never past TODAY's fill target
		// (~60% of the interior) — below that they degrade to compact rows.
		if len(d.exams) > 0 {
			maxFill := int(float64(contentRows)/0.6) + 2
			if need := examsHeight(d.exams, examsW-2) + 2; need > row1H {
				target := minInt(need, minInt(maxRow1, maxFill))
				if target > row1H {
					row1H = target
				}
			}
		}

		// Bands 3+: THIS WEEK at its natural capped height, UP NEXT as
		// its own content-sized band (or dropped when the height does not
		// allow it), and the heatmap absorbing the rest.
		const heatMin = 8
		rem := avail - row1H - coursesH
		upH := upNat
		if len(d.upNext) == 0 {
			upH = 4 // empty-state band
		}
		weekH := 13
		if rem-weekH < heatMin+upH {
			weekH = rem - heatMin - upH
		}
		if weekH < 7 {
			// Not enough room: drop UP NEXT, keep the heatmap readable.
			upH = 0
			weekH = rem - heatMin
			if weekH > 13 {
				weekH = 13
			}
			if weekH < 7 {
				weekH = 7
			}
		}
		heatH := rem - weekH - upH
		if heatH < 4 {
			heatH = 4
		}

		y := row1H + coursesH
		b := [][]homeRect{
			{{"TODAY", 0, 0, todayW, row1H}, {"EXAMS", todayW + gutter, 0, examsW, row1H}},
			{{"COURSES", 0, row1H, w, coursesH}},
			{{"THIS WEEK", 0, y, w, weekH}},
		}
		y += weekH
		if upH > 0 {
			b = append(b, []homeRect{{"UP NEXT", 0, y, w, upH}})
			y += upH
		}
		b = append(b, []homeRect{{"HEATMAP", 0, y, w, heatH}})
		return b
	}

	// Stacked: each panel gets its content height; the heatmap flexes.
	row1H := maxInt(contentRows+2, 6)
	if maxRow1 := (avail * 40) / 100; row1H > maxRow1 {
		row1H = maxRow1
	}
	if row1H < 6 {
		row1H = 6
	}
	left := avail - row1H
	rects := []homeRect{{"TODAY", 0, 0, w, row1H}}
	y := row1H

	// Content-first wishes, shrinkable against a heatmap floor.
	coursesH = clampInt(coursesH, 4, 12)
	upH := upNat
	if upH > 8 || len(d.upNext) == 0 {
		upH = 4
	}
	if len(d.upNext) > 0 && upH > left-8 {
		upH = left - 8
	}
	examsH := 0
	if len(d.exams) > 0 {
		examsH = clampInt(examsNat, 5, 10)
	}
	thisH := 13
	heatH := left - coursesH - upH - examsH - thisH
	for heatH < 8 {
		switch {
		case thisH > 7:
			thisH--
		case examsH > 5:
			examsH--
		case upH > 4:
			upH--
		case upH > 0:
			upH = 0 // hide UP NEXT entirely
		case coursesH > 4:
			coursesH--
		default:
			heatH = 4
			goto doneSteal
		}
		heatH = left - coursesH - upH - examsH - thisH
	}
doneSteal:
	appendRect := func(title string, hh int) {
		if hh <= 0 {
			return
		}
		rects = append(rects, homeRect{title, 0, y, w, hh})
		y += hh
	}
	appendRect("COURSES", coursesH)
	appendRect("UP NEXT", upH)
	appendRect("EXAMS", examsH)
	appendRect("THIS WEEK", thisH)
	appendRect("HEATMAP", heatH)
	// Stacked bands render strictly one after another (never hstacked).
	stacked := make([][]homeRect, 0, len(rects))
	for _, r := range rects {
		stacked = append(stacked, []homeRect{r})
	}
	return stacked
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
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bgBase).Render(" a add · F focus · ctrl+p palette · ? help"))
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
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bgBase).Render(" a add · F focus · ctrl+p palette · ? help"))
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
	// Too short for block-digit cards: compact rows, two lines per exam
	// (stripe + name + countdown, then weight/topics) when they fit; one
	// line each otherwise, so every exam stays visible.
	if need := examsHeight(d.exams, w); h < need {
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
			inTxt := "today"
			switch {
			case e.daysLeft == 1:
				inTxt = "tomorrow"
			case e.daysLeft > 1:
				inTxt = fmt.Sprintf("in %d days", e.daysLeft)
			}
			lines = append(lines, " "+stripe+" "+txt.Render(name)+muted.Render(" — "+inTxt))
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
	// Tall panels grow each weekday cell from the half-block row (1 cell
	// high) up to 2 half-blocks stacked with surface gaps (never full rows —
	// those merge into a slab).
	cellH := 1
	if h > 9 {
		cellH = (h - 2) / 7 // month row + legend row reserved
		if cellH > 2 {
			cellH = 2
		}
		if cellH < 1 {
			cellH = 1
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
	gridRows := 7 * cellH
	// Fit month row and legend to the allocated interior: drop the month
	// row first, then the legend, before anything else clips.
	withMonth := h >= gridRows+2
	withLegend := h >= gridRows+1
	if withMonth {
		lines = append(lines, monthRow)
	}
	for row := 0; row < 7; row++ {
		for j := 0; j < cellH; j++ {
			line := " "
			if j == 0 {
				line += rowLabel(row) + " "
			} else {
				line += strings.Repeat(" ", labelW)
			}
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
	}
	// Legend: less → more.
	if withLegend {
		muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bgSurface)
		legendCell := func(i int) string {
			return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Heatmap[i])).Background(bgSurface).Render("▪") + " "
		}
		legend := strings.Repeat(" ", labelW) + muted.Render("less ") + legendCell(1) + legendCell(2) + legendCell(3) + legendCell(4) + muted.Render("more")
		lines = append(lines, legend)
	}
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
