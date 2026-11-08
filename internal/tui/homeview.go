package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
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
	row1H := (avail * 45) / 100
	row2H := (avail * 25) / 100
	row3H := avail - row1H - row2H
	// Minimums; HEATMAP needs room for its legend row.
	if row3H < 9 {
		row3H = 9
	}
	if row1H < 7 {
		row1H = 7
	}
	if row2H < 6 {
		row2H = 6
	}
	if total := row1H + row2H + row3H; total > avail {
		row1H -= total - avail
		if row1H < 5 {
			row2H -= 5 - row1H
			row1H = 5
		}
	}

	gutter := 1
	switch {
	case w >= 150:
		todayW := (w*44 + 99) / 100
		examsW := (w * 24) / 100
		coursesW := w - todayW - examsW - 2*gutter
		row1 := hstack([][]string{
			splitPane(m, "TODAY", todayW, row1H, m.focus == focusMain, todayBody(m, todayW-2, row1H, d)),
			splitPane(m, "EXAMS", examsW, row1H, false, examsBody(m, examsW-2, row1H, d)),
			splitPane(m, "COURSES", coursesW, row1H, false, coursesBody(m, coursesW-2, row1H, d)),
		}, gutter, base)
		row2 := hstack([][]string{
			splitPane(m, "THIS WEEK", w, row2H, false, thisWeekBody(m, w-2, row2H, d)),
		}, gutter, base)
		row3 := hstack([][]string{
			splitPane(m, "HEATMAP", w, row3H, false, heatmapBody(m, w-2, row3H, d)),
		}, gutter, base)
		lines = append(lines, row1...)
		lines = append(lines, row2...)
		lines = append(lines, row3...)
	case w >= 100:
		todayW := (w*56 + 99) / 100
		examsW := w - todayW - gutter
		coursesW := (w*52 + 99) / 100
		weekW := w - coursesW - gutter
		row1 := hstack([][]string{
			splitPane(m, "TODAY", todayW, row1H, m.focus == focusMain, todayBody(m, todayW-2, row1H, d)),
			splitPane(m, "EXAMS", examsW, row1H, false, examsBody(m, examsW-2, row1H, d)),
		}, gutter, base)
		row2 := hstack([][]string{
			splitPane(m, "COURSES", coursesW, row2H, false, coursesBody(m, coursesW-2, row2H, d)),
			splitPane(m, "THIS WEEK", weekW, row2H, false, thisWeekBody(m, weekW-2, row2H, d)),
		}, gutter, base)
		row3 := hstack([][]string{
			splitPane(m, "HEATMAP", w, row3H, false, heatmapBody(m, w-2, row3H, d)),
		}, gutter, base)
		lines = append(lines, row1...)
		lines = append(lines, row2...)
		lines = append(lines, row3...)
	default:
		// Stacked: each panel gets a proportional share of the available
		// height; TODAY and HEATMAP flex to absorb the leftover.
		blocks := []struct {
			title  string
			weight int
			body   func(iw, ih int) []string
		}{
			{"TODAY", 3, func(iw, ih int) []string { return todayBody(m, iw, ih, d) }},
			{"THIS WEEK", 1, func(iw, ih int) []string { return thisWeekBody(m, iw, ih, d) }},
			{"COURSES", 2, func(iw, ih int) []string { return coursesBody(m, iw, ih, d) }},
			{"EXAMS", 2, func(iw, ih int) []string { return examsBody(m, iw, ih, d) }},
			{"HEATMAP", 3, func(iw, ih int) []string { return heatmapBody(m, iw, ih, d) }},
		}
		totalWeight := 0
		for _, b := range blocks {
			totalWeight += b.weight
		}
		// Minimum panel heights shrink on small terminals so all five fit
		// exactly into the available rows.
		minH := 2
		if avail >= 30 {
			minH = 4
		} else if avail >= 20 {
			minH = 3
		}
		heights := make([]int, len(blocks))
		assigned := 0
		for i, b := range blocks {
			heights[i] = avail * b.weight / totalWeight
			if heights[i] < minH {
				heights[i] = minH
			}
			assigned += heights[i]
		}
		// Distribute the remainder round-robin between the flex panels
		// (TODAY first, HEATMAP second).
		for assigned < avail {
			for _, idx := range []int{0, 4} {
				heights[idx]++
				assigned++
				if assigned == avail {
					break
				}
			}
		}
		for assigned > avail {
			decreased := false
			for i := range heights {
				if heights[i] > minH {
					heights[i]--
					assigned--
					decreased = true
					if assigned == avail {
						break
					}
				}
			}
			if !decreased {
				break
			}
		}
		focused := m.focus == focusMain
		for i, b := range blocks {
			body := b.body(w-2, heights[i]-2)
			lines = append(lines, splitPane(m, b.title, w, heights[i], focused && i == 0, body)...)
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
		if strings.EqualFold(t.Priority, "high") || strings.EqualFold(t.Priority, "urgent") {
			prio = " ! " + strings.ToUpper(t.Priority[:4])
		}

		label := truncatePlain(t.Title, w-6-9-lipgloss.Width(est)-lipgloss.Width(prio)-3)
		var titleStyle lipgloss.Style
		if done {
			titleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bg).Strikethrough(true)
		} else {
			titleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(bg).Bold(sel)
		}
		mark := "○"
		if done {
			mark = "✔"
		}
		line := rowBg.Render(" ") + lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(bg).Render(timeStr) +
			rowBg.Render(" ") + stripe + rowBg.Render(mark+" ") + titleStyle.Render(label)
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
		card := 4 // name + 3 digit rows
		if e.weight >= 0 || e.subTotal > 0 {
			card++ // meta line
		}
		if e.subTotal > 0 {
			card++ // topics bar
		}
		total += card + 1 // trailing gap
	}
	return total
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
	// every exam stays visible. A card costs name+digits (+meta)(+bar)+gap.
	if need := examCardsHeight(d.exams); h < need {
		lines := []string{}
		for i, e := range d.exams {
			if i >= h {
				break
			}
			stripe := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.CourseColor(m.palette, e.task.Project, projectColorOverride(m, e.task.Project)))).Background(bgSurface).Render("▌")
			txt := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(bgSurface)
			name := truncatePlain(e.task.Title, w-10)
			lines = append(lines, " "+stripe+" "+txt.Render(name)+muted.Render(fmt.Sprintf(" — %dd", e.daysLeft)))
		}
		if extra := len(d.exams) - len(lines); extra > 0 && len(lines) < h {
			lines = append(lines, muted.Render(fmt.Sprintf(" +%d more", extra)))
		}
		return lines
	}
	lines := []string{}
	for _, e := range d.exams {
		color := theme.CourseColor(m.palette, e.task.Project, projectColorOverride(m, e.task.Project))
		name := truncatePlain(e.task.Title, w-1)
		lines = append(lines, m.styles.header.Render(" "+name))

		digits := gauges.Digits(fmt.Sprintf("%dd", e.daysLeft))
		for row := 0; row < 3; row++ {
			lines = append(lines, "  "+lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(bgSurface).Render(digits[row]))
		}
		// Weight + topics covered bar.
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
			fill, track := gauges.Bar(float64(e.subDone)/float64(e.subTotal), 12)
			bar := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(bgSurface).Render(fill) +
				muted.Render(track)
			lines = append(lines, " "+bar)
		}
		lines = append(lines, "")
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
	nameW := clampInt(w/3, 6, 14)
	barW := 8
	lines := []string{}
	for _, c := range shown {
		name := truncatePlain(c.name, nameW)
		stripe := lipgloss.NewStyle().Foreground(lipgloss.Color(c.color)).Background(bgSurface).Render("▌")
		fill, track := gauges.Bar(easedFrac(m, c.frac), barW)
		bar := lipgloss.NewStyle().Foreground(lipgloss.Color(c.color)).Background(bgSurface).Render(fill) +
			muted.Render(track)
		pct := fmt.Sprintf("%3.0f%%", c.frac*100)
		lines = append(lines, " "+stripe+" "+padRightPlain(name, nameW)+" "+bar+" "+muted.Render(pct))
	}
	if len(d.courses) > len(shown) {
		lines = append(lines, muted.Render(fmt.Sprintf(" +%d more", len(d.courses)-len(shown))))
	}
	return lines
}

// heatmapBody renders the completion heatmap (Mon..Sun rows) at 2-cell
// granularity: as many week-columns as fit (max 26), a visible subtle track
// for empty days, and a `less ▪▪▪▪ more` legend.
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
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Heatmap[level(v)])).Background(bgSurface).Render("██") + " "
	}
	rowLabel := func(i int) string {
		names := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
		label := ""
		if i%2 == 0 {
			label = names[i]
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bgSurface).Width(4).Render(label)
	}

	const labelW = 5
	weeks := (w - labelW) / 3
	if weeks > 26 {
		weeks = 26
	}
	if weeks < 4 {
		weeks = 4
	}

	lines := []string{}
	for row := 0; row < 7 && row < h-1; row++ {
		line := " " + rowLabel(row)
		for col := 26 - weeks; col < 26; col++ {
			line += cell(d.heatmap[row*26+col])
		}
		lines = append(lines, line)
	}
	// Legend: less → more.
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bgSurface)
	cellS := func(i int) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Heatmap[i])).Background(bgSurface).Render("▪") + " "
	}
	legend := "      " + muted.Render("less ") + cellS(1) + cellS(2) + cellS(3) + cellS(4) + muted.Render("more")
	lines = append(lines, legend)
	return lines
}

// thisWeekBody renders the 7-day study-hours sparkline.
func thisWeekBody(m Model, w, h int, d homeData) []string {
	bgSurface := lipgloss.Color(m.palette.Surface)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(bgSurface)
	bright := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.BrightText)).Background(bgSurface).Bold(true)

	spark := gauges.Sparkline(d.weekHours[:])
	sparkLine := " " + lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(bgSurface).Render(spark)

	total := fmt.Sprintf("%.1f h", d.weekTotal)
	lines := []string{
		"",
		sparkLine,
		" " + bright.Render(total) + " " + muted.Render("study hours"),
		"",
		muted.Render(" last 7 days"),
	}
	return lines
}
