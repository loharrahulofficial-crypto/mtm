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

	lines := []string{}
	lines = append(lines, homeHeaderLine(m, w, d))
	lines = append(lines, homeGreetingLine(m, d, w))
	lines = append(lines, "")
	avail := h - len(lines)
	if avail < 5 {
		// Extremely short: just header + greeting.
		return lines
	}

	// Panel heights hug content; leftover stays blank canvas at the bottom.
	timed := 0
	for _, t := range d.todayRows {
		if len(t.Due) > len("2006-01-02") {
			timed = 1
			break
		}
	}
	todayNeed := clampInt(4+len(d.todayRows)+timed, 7, 18)
	examsNeed := 2
	if len(d.exams) > 0 {
		examsNeed += 5 * len(d.exams)
		if examsNeed > 13 {
			examsNeed = 13
		}
	}
	coursesNeed := clampInt(3+len(d.courses), 6, 12)
	row1H := clampInt(max3(todayNeed, examsNeed, coursesNeed), 8, 20)

	heatNeed := 10 // 7 rows + title + legend
	row2H := clampInt(heatNeed, 8, max(8, avail-row1H-1))

	if row2H < 8 {
		row2H = 8
	}

	switch {
	case w >= 150:
		gutter := 1
		todayW := (w*46 + 99) / 100
		examsW := (w * 24) / 100
		coursesW := w - todayW - examsW - 2*gutter
		heatW := todayW + examsW + gutter
		thisW := w - heatW - gutter
		row1 := hstack([][]string{
			splitPane(m, "TODAY", todayW, row1H, m.focus == focusMain, todayBody(m, todayW-2, row1H, d)),
			splitPane(m, "EXAMS", examsW, row1H, false, examsBody(m, examsW-2, row1H, d)),
			splitPane(m, "COURSES", coursesW, row1H, false, coursesBody(m, coursesW-2, row1H, d)),
		}, gutter, base)
		row2 := hstack([][]string{
			splitPane(m, "HEATMAP", heatW, row2H, false, heatmapBody(m, heatW-2, row2H, d)),
			splitPane(m, "THIS WEEK", thisW, row2H, false, thisWeekBody(m, thisW-2, row2H, d)),
		}, gutter, base)
		lines = append(lines, row1...)
		lines = append(lines, "")
		lines = append(lines, row2...)
	case w >= 100:
		gutter := 1
		topH := row1H
		midH := clampInt(max3(coursesNeed, 7, 7), 7, 12)
		botH := clampInt(avail-topH-midH-2, 6, 12)
		todayW := (w * 56) / 100
		examsW := w - todayW - gutter
		coursesW := (w * 52) / 100
		weekW := w - coursesW - gutter
		heatW := w
		row1 := hstack([][]string{
			splitPane(m, "TODAY", todayW, topH, m.focus == focusMain, todayBody(m, todayW-2, topH, d)),
			splitPane(m, "EXAMS", examsW, topH, false, examsBody(m, examsW-2, topH, d)),
		}, gutter, base)
		row2 := hstack([][]string{
			splitPane(m, "COURSES", coursesW, midH, false, coursesBody(m, coursesW-2, midH, d)),
			splitPane(m, "THIS WEEK", weekW, midH, false, thisWeekBody(m, weekW-2, midH, d)),
		}, gutter, base)
		lines = append(lines, row1...)
		lines = append(lines, "")
		lines = append(lines, row2...)
		lines = append(lines, "")
		lines = append(lines, splitPane(m, "HEATMAP", heatW, botH, false, heatmapBody(m, heatW-2, botH, d))...)
	default:
		// Stacked: each panel gets the height its content needs; TODAY and
		// HEATMAP flex to absorb leftover canvas.
		panels := []struct {
			title string
			need  int
			body  func(innerW, innerH int) []string
		}{
			{"TODAY", todayNeed, func(iw, ih int) []string { return todayBody(m, iw, ih, d) }},
			{"THIS WEEK", 7, func(iw, ih int) []string { return thisWeekBody(m, iw, ih, d) }},
			{"COURSES", clampInt(3+len(d.courses), 6, 12), func(iw, ih int) []string { return coursesBody(m, iw, ih, d) }},
			{"EXAMS", clampInt(2+max(1, len(d.exams))*5, 7, 12), func(iw, ih int) []string { return examsBody(m, iw, ih, d) }},
			{"HEATMAP", 10, func(iw, ih int) []string { return heatmapBody(m, iw, ih, d) }},
		}
		blanks := len(panels) - 1
		room := avail - blanks
		if room < 5*len(panels) {
			room = 5 * len(panels)
		}
		heights := make([]int, len(panels))
		total := 0
		for i, p := range panels {
			heights[i] = p.need
			total += p.need
		}
		caps := []int{22, 8, 12, 12, 13}
		if total <= room {
			// Flex toward content needs, then distribute leftover round-robin.
			leftover := room - total
			for leftover > 0 {
				gave := false
				for i := range heights {
					if heights[i] < caps[i] {
						add := 1
						if heights[i]+add > caps[i] {
							add = caps[i] - heights[i]
						}
						heights[i] += add
						leftover -= add
						gave = true
					}
					if leftover == 0 {
						break
					}
				}
				if !gave {
					heights[0] += leftover // TODAY absorbs residue
					break
				}
			}
		} else {
			// Overfull: shrink proportionally, floor 5.
			scaled := make([]int, len(heights))
			scaledTotal := 0
			for i := range heights {
				scaled[i] = heights[i] * room / total
				if scaled[i] < 5 {
					scaled[i] = 5
				}
				scaledTotal += scaled[i]
			}
			for rem := room - scaledTotal; rem != 0; {
				for i := 0; i < len(scaled) && rem != 0; i++ {
					if rem > 0 {
						scaled[i]++
						rem--
					} else if scaled[i] > 5 {
						scaled[i]--
						rem++
					}
				}
				if rem < 0 && func() bool {
					for _, h := range scaled {
						if h > 5 {
							return true
						}
					}
					return false
				}() == false {
					break
				}
			}
			heights = scaled
		}
		focused := m.focus == focusMain
		for i, p := range panels {
			ph := heights[i]
			body := p.body(w-2, ph-2)
			if i > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, splitPane(m, p.title, w, ph, focused && i == 0, body)...)
		}
	}
	return lines
}

// homeHeaderLine renders "mtm · Sat 7 Nov · Week 11 of 16" left and the
// clock + sync glyph right, on base.
func homeHeaderLine(m Model, w int, d homeData) string {
	left := m.styles.pillActive.Render(" mtm ") + "  " +
		lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(lipgloss.Color(m.palette.Base)).
			Render(d.dateLine+" · "+d.weekLine)

	s := m.styles
	syncGlyph := ""
	switch m.syncSt {
	case syncOK:
		syncGlyph = s.syncOK.Render(" ✓ synced ")
	case syncSyncing:
		syncGlyph = s.footer.Render(" " + m.spinner.View() + " syncing ")
	case syncConflict:
		syncGlyph = s.syncBad.Render(" ⚠ conflicts ")
	case syncOffline:
		syncGlyph = s.footer.Render(" ○ offline ")
	default:
		syncGlyph = s.footer.Render(" ○ local ")
	}
	clock := s.pillActive.Render(" " + m.clock.Now().Format("15:04") + " ")
	right := syncGlyph + clock

	pad := w - lipgloss.Width(left) - lipgloss.Width(right)
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + right
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
		line := " " + lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(bg).Render(timeStr) +
			" " + stripe + " " + rowBg.Render(mark+" ") + titleStyle.Render(label)
		rest := strings.Repeat(" ", 1) + muted.Render(est) + dangerStyleFor(t, m, bg).Render(prio)
		line += strings.Repeat(" ", max(0, w-lipglossWidth(line)-lipglossWidth(rest)-1)) + rest + " "
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

// examsBody renders up to two exam countdown cards.
func examsBody(m Model, w, h int, d homeData) []string {
	bgSurface := lipgloss.Color(m.palette.Surface)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(bgSurface)

	if len(d.exams) == 0 {
		return []string{
			"",
			muted.Render(" ✦ no exams — enjoy the calm"),
		}
	}
	if h < 4 {
		// Too short for block digits — single-line summary per exam.
		lines := []string{}
		for i, e := range d.exams {
			if i >= h {
				break
			}
			name := truncatePlain(e.task.Title, w-16)
			lines = append(lines, fmt.Sprintf(" ✦ %s — %dd", name, e.daysLeft))
		}
		return lines
	}
	lines := []string{}
	perCard := h / len(d.exams)
	if perCard > 6 {
		perCard = 6
	}
	for i, e := range d.exams {
		if i > 0 && len(lines)+perCard > h {
			break
		}
		color := theme.CourseColor(m.palette, e.task.Project, projectColorOverride(m, e.task.Project))
		name := truncatePlain(e.task.Title, w-1)
		lines = append(lines, m.styles.header.Render(" "+name))

		digits := gauges.Digits(fmt.Sprintf("%d", e.daysLeft))
		days := "days"
		if e.daysLeft == 1 {
			days = "day"
		}
		for row := 0; row < 3; row++ {
			line := "  " + lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(bgSurface).Render(digits[row])
			if row == 1 {
				line += " " + muted.Render(days)
			}
			lines = append(lines, line)
		}
		// Weight + topics covered bar.
		meta := ""
		if e.weight >= 0 {
			meta += fmt.Sprintf("weight %.0f%%", e.weight*100)
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

// heatmapBody renders the 16-week completion heatmap (Mon..Sun rows).
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
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Heatmap[level(v)])).Background(bgSurface).Render("█") + " "
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
	for row := 0; row < 7 && row < h-1; row++ {
		line := " " + rowLabel(row)
		for col := 0; col < 16; col++ {
			line += cell(d.heatmap[row*16+col])
		}
		lines = append(lines, line)
	}
	// Legend: less → more.
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bgSurface)
	legend := "      " + muted.Render("less") + " "
	for i := 0; i < 5; i++ {
		legend += lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Heatmap[i])).Background(bgSurface).Render("█") + " "
	}
	legend += muted.Render("more")
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

func max3(a, b, c int) int {
	m := a
	if b > m {
		m = b
	}
	if c > m {
		m = c
	}
	return m
}
