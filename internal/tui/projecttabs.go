package tui

// The four tab renderers for the project page. All are pure reads over the
// store plus the cached GitHub info; none mutate state beyond optional
// scroll clamping done locally.

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/gauges"
)

// section builds a letterspaced muted section label line.
func section(m Model, label string) string {
	return sectionLabelText(m, label)
}

// projectIsDone is the project-aware done check.
func projectIsDone(m Model) func(p, st string) bool {
	return func(p, st string) bool { return m.store.IsDoneStatus(p, st) }
}

// projectTaskStats counts done/total and logged minutes over the project's
// tasks (subtasks included).
func projectTaskStats(m Model) (done, total int, loggedMin int, hoursThisWeek float64) {
	isDone := projectIsDone(m)
	loc := m.clock.Now().Location()
	todayStart := dayStartT(m.clock.Now())
	weekStart := todayStart.AddDate(0, 0, -6)
	for _, t := range m.store.Tasks() {
		if t.Project != m.projectPage {
			continue
		}
		total++
		if isDone(t.Project, t.Status) {
			done++
		}
		if t.TimeLoggedMinutes <= 0 {
			continue
		}
		loggedMin += t.TimeLoggedMinutes
		stamp := t.Completed
		if stamp == "" && isDone(t.Project, t.Status) {
			stamp = t.Updated
		}
		if ts, ok := parseStampT(stamp, loc); ok {
			day := time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, loc)
			if !day.Before(weekStart) {
				hoursThisWeek += float64(t.TimeLoggedMinutes) / 60.0
			}
		}
	}
	return done, total, loggedMin, hoursThisWeek
}

func parseStampT(s string, loc *time.Location) (time.Time, bool) {
	for _, layout := range []string{core.TimeFormat, "2006-01-02 15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// wordWrapGreedy wraps plain text to width (simple, undistracting).
func wordWrapGreedy(s string, width int) []string {
	if width < 1 {
		width = 1
	}
	out := []string{}
	for _, para := range strings.Split(s, "\n") {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		line := ""
		for _, word := range strings.Fields(para) {
			switch {
			case line == "":
				line = word
			case len(line)+1+len(word) <= width:
				line += " " + word
			default:
				out = append(out, line)
				line = word
			}
		}
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// daysUntilTarget reports whole days from today to the target (negative when
// overdue). ok=false for missing/unparseable targets.
func daysUntilTarget(m Model, target string) (int, bool) {
	if strings.TrimSpace(target) == "" {
		return 0, false
	}
	now := m.clock.Now()
	day, err := parseMilestoneDay(target, now.Location())
	if err != nil {
		return 0, false
	}
	return int(dayStartT(now).Sub(time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, now.Location())).Hours() / -24), true
}

// relTimeAgo renders "3d ago" style durations.
func relTimeAgo(now, ts time.Time) string {
	if ts.IsZero() {
		return "never"
	}
	d := now.Sub(ts)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 48*time.Hour:
		return "yesterday"
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// tildePath shortens the user's home dir to ~ for display.
func tildePath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+string(os.PathSeparator)) {
		return "~" + p[len(home):]
	}
	return p
}

// githubSectionLines renders the G I T H U B section.
func (m Model) githubSectionLines(w int) []string {
	s := m.styles
	base := lipgloss.Color(m.palette.Base)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(base)
	text := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(base)

	p, _ := m.store.GetProject(m.projectPage)
	linked := p != nil && (strings.TrimSpace(p.GitHub) != "" || strings.TrimSpace(p.RepoPath) != "")
	if !linked {
		return []string{"  " + muted.Render("not linked — mtm project link "+m.projectPage+" [--repo owner/name] [--path ~/Projects/foo]")}
	}

	lines := []string{}
	head := "  "
	if m.hasIcon("github") {
		head += s.mAccent.Render(m.icon("github")) + " "
	}
	slug := strings.TrimSpace(p.GitHub)
	if slug == "" {
		slug = "local repo"
	}
	head += s.mAccent.Render(slug)
	gi := m.githubFor(m.projectPage)
	if gi != nil && gi.Branch != "" {
		head += muted.Render("   " + gi.Branch)
	}
	// The path is the identity only when there is no slug; with a slug the
	// path is noise (and machine-specific — keep dumps byte-stable).
	if slug == "local repo" && p.RepoPath != "" {
		head += muted.Render("   " + tildePath(p.RepoPath))
	}
	lines = append(lines, head)

	if gi == nil {
		lines = append(lines, "  "+muted.Render("loading…"))
		return lines
	}
	if !gi.LocalOK {
		note := gi.LocalNote
		if note == "" {
			note = "repo unreadable"
		}
		lines = append(lines, "  "+muted.Render("local: "+truncatePlain(note, w-12)))
	} else {
		now := m.clock.Now()
		streak := ""
		if gi.Streak > 0 {
			streak = "   "
			if m.hasIcon("flame") {
				streak += m.icon("flame") + " "
			}
			streak += fmt.Sprintf("%dd streak", gi.Streak)
		}
		lines = append(lines, "  "+muted.Render("last commit "+relTimeAgo(now, gi.LastCommit)+streak))
		vals := make([]float64, StreakWeeksT)
		for i, n := range gi.Weeks {
			vals[i] = n
		}
		lines = append(lines, "  "+muted.Render("commits/12w ")+s.mAccent.Render(gauges.Sparkline(vals)))
		for i, sub := range gi.Subjects {
			if i >= 5 {
				break
			}
			lines = append(lines, "    "+muted.Render("· ")+text.Render(truncatePlain(sub, w-8)))
		}
	}
	switch {
	case gi.RemoteOK:
		lines = append(lines, "  "+text.Render(fmt.Sprintf("issues %d open · %d PRs open", gi.OpenIssues, gi.OpenPRs)))
		for _, it := range gi.LatestIssues {
			lines = append(lines, "    "+s.mAccent.Render(fmt.Sprintf("#%d", it.Number))+muted.Render(" "+truncatePlain(it.Title, w-12)))
		}
		for _, it := range gi.LatestPRs {
			lines = append(lines, "    "+s.mAccent.Render(fmt.Sprintf("#%d", it.Number))+muted.Render(" "+truncatePlain(it.Title, w-12)))
		}
	case strings.TrimSpace(gi.RemoteNote) != "":
		lines = append(lines, "  "+muted.Render("remote unavailable — "+truncatePlain(gi.RemoteNote, w-26)))
	}
	return lines
}

const StreakWeeksT = actions.StreakWeeks

// ---- Roadmap tab -----------------------------------------------------------

// milestoneForRoadmap sorts milestones for display: open ones by target
// (earliest first), missing targets last, done milestones at the end.
func milestonesForRoadmap(m Model) []*core.Milestone {
	ms := append([]*core.Milestone{}, m.store.Milestones(m.projectPage)...)
	sort.SliceStable(ms, func(i, j int) bool {
		di, dj := strings.EqualFold(ms[i].Status, core.MilestoneDone), strings.EqualFold(ms[j].Status, core.MilestoneDone)
		if di != dj {
			return !di
		}
		if ms[i].Target == "" || ms[j].Target == "" {
			return ms[j].Target == ""
		}
		return ms[i].Target < ms[j].Target
	})
	return ms
}

var weekdayAbbr = []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

var monthAbbr = []string{"", "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

// mondayOf returns the Monday 00:00 of the week containing day.
func mondayOf(day time.Time) time.Time {
	offset := (int(day.Weekday()) + 6) % 7 // days since Monday
	return dayStartT(day).AddDate(0, 0, -offset)
}

// renderProjectRoadmap renders the horizontal timeline.
func (m Model) renderProjectRoadmap(w, h int) []string {
	const windowDays = 84 // 12 weeks
	const cols = 12
	s := m.styles
	base := lipgloss.Color(m.palette.Base)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(base)
	text := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(base)
	bright := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.BrightText)).Background(base).Bold(true)
	raised := lipgloss.NewStyle().Background(lipgloss.Color(m.palette.SurfaceRaised)).Foreground(lipgloss.Color(m.palette.BrightText)).Bold(true)

	now := m.clock.Now()
	todayStart := dayStartT(now)
	anchor := todayStart.AddDate(0, 0, -28+m.roadmapPan)
	windowStart := mondayOf(anchor)

	labelW := w / 5
	if labelW > 22 {
		labelW = 22
	}
	if labelW < 14 {
		labelW = 14
	}
	gridX := labelW + 2
	gridW := w - gridX - 1
	if gridW < 10 {
		gridW = 10
	}

	lines := []string{}
	// Month header: label the first week bucket of each new month.
	header := strings.Repeat(" ", gridX)
	for i := 0; i < cols; i++ {
		bucket := windowStart.AddDate(0, 0, 7*i)
		prev := bucket.AddDate(0, 0, -7)
		if i == 0 || bucket.Month() != prev.Month() {
			label := monthAbbr[bucket.Month()]
			cell := gridX + i*gridW/cols
			if cell+len(label) <= w {
				header = splicePlain(header, cell, label)
			}
		}
	}
	lines = append(lines, muted.Render(header))

	todayX := gridX + int(float64(int(todayStart.Sub(windowStart).Hours()/24))*float64(gridW)/float64(windowDays))
	if todayX < gridX {
		todayX = gridX
	}

	milestones := milestonesForRoadmap(m)
	isDone := projectIsDone(m)
	if len(milestones) == 0 {
		lines = append(lines, "")
		lines = append(lines, "  "+muted.Render("no milestones in this project yet."))
		lines = append(lines, "  "+muted.Render("mtm milestone add "+m.projectPage+" \"title\" --target 2026-12-15"))
		for len(lines) < h {
			lines = append(lines, "")
		}
		return lines
	}
	sel := m.projectSel
	if sel < 0 {
		sel = 0
	}
	if sel >= len(milestones) {
		sel = len(milestones) - 1
	}

	barRows := h - 8 // reserve the detail block
	if barRows < 3 {
		barRows = 3
	}
	shown := 0
	for i, ms := range milestones {
		if shown >= barRows {
			break
		}
		shown++
		frac := 0.0
		doneT, totalT, f := core.MilestoneProgress(m.store.Tasks(), ms.ID, isDone)
		frac = f
		risk := core.MilestoneRisk(ms, now)
		barColor := lipgloss.Color(m.palette.Success)
		switch risk {
		case core.MilestoneRiskAt:
			barColor = lipgloss.Color(m.palette.Warning)
		case core.MilestoneRiskOverdue:
			barColor = lipgloss.Color(m.palette.Danger)
		}
		if strings.EqualFold(ms.Status, core.MilestoneDone) {
			barColor = lipgloss.Color(m.palette.Success)
		}

		// Label cell: selection caret + title + pct.
		caret := "  "
		if i == sel {
			caret = " ▸"
		}
		pct := fmt.Sprintf("%3.0f%%", frac*100)
		label := truncatePlain(ms.Title, labelW-6) + " " + pct
		label = padWidth(label, labelW-2)

		// Bar cell.
		bar := strings.Repeat(" ", gridW)
		if ms.Target != "" {
			createdDay, cerr := parseMilestoneDay(ms.Created, now.Location())
			if cerr != nil {
				createdDay = windowStart // born at the window start
			}
			targetDay, err := parseMilestoneDay(ms.Target, now.Location())
			if err == nil {
				startX, endX := 0, 0
				dayOff := func(day time.Time) int {
					return int(dayStartT(day).Sub(windowStart).Hours() / 24)
				}
				sOff := dayOff(createdDay)
				eOff := dayOff(targetDay)
				if sOff < 0 {
					sOff = 0
				}
				startX = sOff * gridW / windowDays
				endX = eOff * gridW / windowDays
				if endX < 0 {
					startX, endX = 0, 1
				}
				if startX >= gridW {
					// Entirely beyond the window.
					startX, endX = gridW-1, gridW
				}
				if endX > gridW {
					endX = gridW
				}
				if endX <= startX {
					endX = startX + 1
					if endX > gridW {
						endX = gridW
					}
				}
				fillCells := int(frac * float64(endX-startX))
				// A completed milestone (status done) is a full bar: no
				// track cells inside it, wherever the today marker lands.
				milestoneDone := strings.EqualFold(ms.Status, core.MilestoneDone) || frac >= 1
				if milestoneDone && endX > startX {
					fillCells = endX - startX
				}
				cells := []rune{}
				for x := 0; x < gridW; x++ {
					switch {
					case x < startX || x >= endX:
						cells = append(cells, ' ')
					case x-startX < fillCells:
						cells = append(cells, '█')
					default:
						cells = append(cells, '░')
					}
				}
				barStyle := lipgloss.NewStyle().Foreground(barColor).Background(base)
				trackStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.SurfaceStrong)).Background(base)
				if i == sel {
					barStyle = lipgloss.NewStyle().Foreground(barColor).Background(lipgloss.Color(m.palette.SurfaceRaised))
					trackStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.SurfaceStrong)).Background(lipgloss.Color(m.palette.SurfaceRaised))
				}
				var sb strings.Builder
				for _, c := range cells {
					switch c {
					case '█':
						sb.WriteString(barStyle.Render("█"))
					case '░':
						sb.WriteString(trackStyle.Render("░"))
					default:
						if i == sel {
							sb.WriteString(raised.Render(" "))
						} else {
							sb.WriteString(" ")
						}
					}
				}
				bar = sb.String()
			}
		}

		// Compose the row, then stamp the today marker.
		labelCell := caret + label + " "
		full := labelCell + bar
		if i == sel {
			// Continuous raised block behind the selection.
			full = raised.Render(padWidth(labelCell+strings.Repeat(" ", gridW), w))
			// Re-stamp the bar on top of the raised block.
			full = raised.Render(labelCell) + bar
		}
		_ = doneT
		_ = totalT
		lines = append(lines, full)
	}
	// Overlay the today marker on the bar rows.
	for i := 1; i < len(lines); i++ {
		lines[i] = stampToday(lines[i], todayX, bright)
	}

	// Detail block for the selected milestone.
	lines = append(lines, muted.Render(strings.Repeat("─", w)))
	ms := milestones[sel]
	doneT, totalT, frac := core.MilestoneProgress(m.store.Tasks(), ms.ID, isDone)
	lines = append(lines, " ▸ "+bright.Render(truncatePlain(ms.Title, w-34))+"  "+milestoneStatusChip(m, ms))
	barFill, barTrack := gauges.Bar(frac, 20)
	lines = append(lines, "   "+s.mAccent.Render(barFill)+muted.Render(barTrack+fmt.Sprintf(" %3.0f%%  (%d/%d)", frac*100, doneT, totalT)))
	risk := core.MilestoneRisk(ms, now)
	riskText, riskColor := "on track", m.palette.Success
	switch risk {
	case core.MilestoneRiskAt:
		riskText, riskColor = "at risk", m.palette.Warning
	case core.MilestoneRiskOverdue:
		riskText, riskColor = "overdue", m.palette.Danger
	}
	target := ms.Target
	if target == "" {
		target = "no target date"
	} else if strings.EqualFold(ms.Status, core.MilestoneDone) {
		target = "target " + target + " · done"
	} else if d, ok := daysUntilTarget(m, ms.Target); ok {
		switch {
		case d == 0:
			target = "target " + target + " · today"
		case d > 0:
			target = fmt.Sprintf("target %s · in %dd", ms.Target, d)
		default:
			target = fmt.Sprintf("target %s · %dd overdue", ms.Target, -d)
		}
	}
	riskStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(riskColor)).Background(base).Bold(true)
	lines = append(lines, "   "+muted.Render(target)+"  ·  "+riskStyle.Render(riskText))

	// Linked tasks, open first.
	scope := core.MilestoneScope(m.store.Tasks(), ms.ID)
	sortTasksForPage(scope, m)
	maxTaskLines := h - len(lines) - 1
	shownTasks := 0
	for _, t := range scope {
		if shownTasks >= maxTaskLines {
			break
		}
		doneT := isDone(t.Project, t.Status)
		box := muted.Render("○")
		textStyle := text
		if doneT {
			box = s.mAccent.Render("✓")
			textStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(base)
		}
		due := ""
		if t.Due != "" {
			if label, _ := gauges.DueLabel(t.Due, now); label != "" {
				due = "  " + label
			}
		}
		lines = append(lines, "   "+box+" "+textStyle.Render(truncatePlain(t.Title, w-16-len(due)))+muted.Render(due))
		shownTasks++
	}
	if len(scope) > shownTasks {
		lines = append(lines, "   "+muted.Render(fmt.Sprintf("+%d more", len(scope)-shownTasks)))
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return lines
}

// splicePlain stamps s into base at cell offset left, extending with spaces
// when needed.
func splicePlain(base string, left int, s string) string {
	br := []rune(base)
	for i, r := range []rune(s) {
		idx := left + i
		for len(br) <= idx {
			br = append(br, ' ')
		}
		br[idx] = r
	}
	return string(br)
}

// stampToday overlays a ┃ at cell x on a line (marking today on the timeline).
func stampToday(line string, x int, style lipgloss.Style) string {
	if fastWidth(line) <= x {
		return line
	}
	prefix := clipStyled(line, x)
	suffix := clipStyled(ansi.Cut(line, x+1, ansi.StringWidth(line)), fastWidth(line)-x-1)
	return prefix + style.Render("┃") + suffix
}

// ---- Dev-log tab -----------------------------------------------------------

// renderProjectDevlog renders the dev-log as a glamour-rendered timeline.
func (m Model) renderProjectDevlog(w, h int) []string {
	s := m.styles
	base := lipgloss.Color(m.palette.Base)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(base)

	entries := m.store.DevLogs(m.projectPage)
	if len(entries) == 0 {
		lines := []string{
			"",
			"  " + muted.Render("no dev-log entries yet."),
			"  " + muted.Render("mtm log "+m.projectPage+" \"text\"  — or open this scaffold in $EDITOR:"),
			"  " + muted.Render("mtm log "+m.projectPage),
		}
		for len(lines) < h {
			lines = append(lines, "")
		}
		for len(lines) > h {
			lines = lines[:h]
		}
		return lines
	}
	sort.Slice(entries, func(a, b int) bool { return entries[a].Created > entries[b].Created })
	var doc strings.Builder
	for _, e := range entries {
		date := e.Created
		if len(date) >= 10 {
			date = date[:10]
		}
		doc.WriteString("### " + date + " — " + e.Title + "\n\n")
		meta := []string{}
		if e.TimeSpentMinutes > 0 {
			meta = append(meta, actions.FormatMinutes(e.TimeSpentMinutes))
		}
		if e.Mood != "" {
			meta = append(meta, "mood: "+e.Mood)
		}
		if len(e.Tags) > 0 {
			meta = append(meta, "#"+strings.Join(e.Tags, " #"))
		}
		if len(meta) > 0 {
			doc.WriteString("*" + strings.Join(meta, " · ") + "*\n\n")
		}
		body := strings.TrimRight(e.Body, "\n")
		if body != "" {
			doc.WriteString(body + "\n\n")
		}
		doc.WriteString("---\n\n")
	}
	rendered := m.glamourRender(doc.String(), w-4)
	lines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")
	offset := clampScroll(m.projectScroll, len(lines), h)
	out := sliceLines(lines, offset, h)
	for len(out) < h {
		out = append(out, "")
	}
	_ = s
	return out
}

func (m Model) renderProjectOverview(w, h int) []string {
	s := m.styles
	base := lipgloss.Color(m.palette.Base)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(base)
	text := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(base)
	bright := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.BrightText)).Background(base).Bold(true)

	p, _ := m.store.GetProject(m.projectPage)
	isDone := projectIsDone(m)
	now := m.clock.Now()

	lines := []string{}

	// DESCRIPTION -----------------------------------------------------------
	lines = append(lines, section(m, "DESCRIPTION"))
	if p != nil && strings.TrimSpace(p.Description) != "" {
		for _, line := range wordWrapGreedy(strings.TrimSpace(p.Description), w-2) {
			lines = append(lines, "  "+text.Render(line))
		}
	} else {
		lines = append(lines, "  "+muted.Render("no description — mtm project edit "+m.projectPage))
	}
	lines = append(lines, "")

	// PROGRESS ---------------------------------------------------------------
	lines = append(lines, section(m, "PROGRESS"))
	done, total, loggedMin, weekH := projectTaskStats(m)
	mDone, mTotal := 0, 0
	for _, ms := range m.store.Milestones(m.projectPage) {
		mTotal++
		if strings.EqualFold(ms.Status, core.MilestoneDone) {
			mDone++
		}
	}
	frac, enough := core.CombineProgress(done, total, mDone, mTotal)
	barFill, barTrack := gauges.Bar(frac, 22)
	pct := "0%"
	if enough {
		pct = fmt.Sprintf("%3.0f%%", frac*100)
	}
	lines = append(lines, "  "+bright.Render(barFill)+muted.Render(barTrack+" "+pct))
	parts := []string{fmt.Sprintf("tasks %d/%d", done, total)}
	if mTotal > 0 {
		parts = append(parts, fmt.Sprintf("milestones %d/%d", mDone, mTotal))
	}
	lines = append(lines, "  "+text.Render(strings.Join(parts, "  ·  ")))
	if loggedMin > 0 {
		hoursLine := fmt.Sprintf("%sh logged", actions.FormatMinutes(loggedMin))
		if weekH > 0 {
			hoursLine += fmt.Sprintf(" · %.1fh this week", weekH)
		}
		lines = append(lines, "  "+muted.Render(hoursLine))
	}
	lines = append(lines, "")

	// VELOCITY ---------------------------------------------------------------
	lines = append(lines, section(m, "VELOCITY · 12 WEEKS"))
	isDoneFn := projectIsDone(m)
	v := core.VelocityReport(projectTasksFlat(m), isDoneFn, now, core.VelocityWindow)
	vals := make([]float64, len(v.Weeks))
	for i, n := range v.Weeks {
		vals[i] = n
	}
	spark := gauges.Sparkline(vals)
	lines = append(lines, "  "+s.mAccent.Render(spark)+text.Render(fmt.Sprintf("  %.1f tasks/wk", v.AvgPerWeek)))
	remain := 0
	for _, t := range projectTasksFlat(m) {
		if !isDone(t.Project, t.Status) {
			remain++
		}
	}
	if remain == 0 && total > 0 {
		lines = append(lines, "  "+text.Render("scope complete — every task done"))
	} else if finish, ok := core.EstimateFinish(remain, v, now); ok {
		lines = append(lines, "  "+text.Render("est. finish "+finish.Format("2006-01-02")+fmt.Sprintf("  (%d open)", remain)))
	} else {
		lines = append(lines, "  "+muted.Render("not enough data — need ≥2 active weeks"))
	}
	lines = append(lines, "")

	// NEXT MILESTONE ----------------------------------------------------------
	lines = append(lines, section(m, "NEXT MILESTONE"))
	best, bestDays, ok := nextMilestoneIn(m, m.projectPage)
	if !ok {
		lines = append(lines, "  "+muted.Render("none pending — mtm milestone add "+m.projectPage+" \"title\""))
	} else {
		doneM, totalM, fracM := core.MilestoneProgress(m.store.Tasks(), best.ID, isDone)
		pctM := fmt.Sprintf("%3.0f%%", fracM*100)
		chip := milestoneChipText(m, best, bestDays)
		lines = append(lines, "  "+bright.Render("▸ "+truncatePlain(best.Title, w-30))+"  "+milestoneStatusChip(m, best))
		lines = append(lines, "  "+muted.Render(fmt.Sprintf("target %s · %s · %s (%d/%d)", best.Target, chip, pctM, doneM, totalM)))
	}
	lines = append(lines, "")

	// GITHUB ------------------------------------------------------------------
	lines = append(lines, section(m, "GITHUB"))
	lines = append(lines, m.githubSectionLines(w)...)

	offset := clampScroll(m.projectScroll, len(lines), h)
	return sliceLines(lines, offset, h)
}

// parseMilestoneDay parses a milestone day from the date-only form used by
// mtm milestone add ("2006-01-02") or the timestamp form the store keeps in
// frontmatter ("2006-01-02T15:04"). A zero day means "missing".
func parseMilestoneDay(s string, loc *time.Location) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"2006-01-02", "2006-01-02T15:04", "2006-01-02 15:04"} {
		if day, err := time.ParseInLocation(layout, s, loc); err == nil {
			return day, nil
		}
	}
	return time.Time{}, fmt.Errorf("unparseable milestone day %q", s)
}

// projectTasksFlat returns the project's tasks (parents + subtasks).
func projectTasksFlat(m Model) []*core.Task {
	out := []*core.Task{}
	for _, t := range m.store.Tasks() {
		if t.Project == m.projectPage {
			out = append(out, t)
		}
	}
	return out
}

// nextMilestoneIn finds the nearest open milestone with a target, scoped to project.
func nextMilestoneIn(m Model, project string) (*core.Milestone, int, bool) {
	now := m.clock.Now()
	today := dayStartT(now)
	var best *core.Milestone
	bestDays := 0
	for _, ms := range m.store.Milestones(project) {
		if strings.EqualFold(ms.Status, core.MilestoneDone) || ms.Target == "" {
			continue
		}
		day, err := parseMilestoneDay(ms.Target, now.Location())
		if err != nil {
			continue
		}
		days := int(dayStartT(day).Sub(today).Hours() / 24)
		if best == nil || days < bestDays {
			best, bestDays = ms, days
		}
	}
	if best == nil {
		return nil, 0, false
	}
	return best, bestDays, true
}

func milestoneChipText(m Model, best *core.Milestone, bestDays int) string {
	switch {
	case bestDays == 0:
		return "today"
	case bestDays > 0:
		return fmt.Sprintf("in %dd", bestDays)
	default:
		return fmt.Sprintf("%dd overdue", -bestDays)
	}
}

func milestoneStatusChip(m Model, ms *core.Milestone) string {
	color := m.palette.MutedSoft
	switch strings.ToLower(ms.Status) {
	case core.MilestonePlanned:
		color = m.palette.Warning
	case core.MilestoneActive:
		color = m.palette.Accent
	case core.MilestoneDone:
		color = m.palette.Success
	case core.MilestoneMissed:
		color = m.palette.Danger
	}
	surface := lipgloss.Color(m.palette.Base)
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(surface).Bold(true).Render(ms.Status)
}

func clampScroll(offset, total, h int) int {
	max := total - h
	if max < 0 {
		max = 0
	}
	if offset > max {
		return max
	}
	if offset < 0 {
		return 0
	}
	return offset
}

func sliceLines(lines []string, offset, h int) []string {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(lines) {
		offset = len(lines) - 1
		if offset < 0 {
			offset = 0
		}
	}
	out := lines[offset:]
	if len(out) > h {
		out = out[:h]
	}
	return out
}

// ---- Tasks tab ------------------------------------------------------------

// renderProjectTasks renders the project's task list (scrollable).
func (m Model) renderProjectTasks(w, h int) []string {
	s := m.styles
	base := lipgloss.Color(m.palette.Base)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(base)
	isDone := projectIsDone(m)
	now := m.clock.Now()

	tasks := projectTasks(m)
	open, done := 0, 0
	for _, t := range tasks {
		if isDone(t.Project, t.Status) {
			done++
		} else {
			open++
		}
	}
	lines := []string{fmt.Sprintf("  %s", muted.Render(fmt.Sprintf("%d open · %d done", open, done)))}

	mstoneTitles := map[string]string{}
	for _, ms := range m.store.Milestones("") {
		mstoneTitles[ms.ID] = ms.Title
	}

	entries := []struct {
		t     *core.Task
		depth int
	}{}
	depthOf := map[string]int{}
	for _, t := range tasks {
		d := 0
		if t.Parent != "" {
			d = depthOf[t.Parent] + 1
		}
		depthOf[t.ID] = d
		entries = append(entries, struct {
			t     *core.Task
			depth int
		}{t, d})
	}
	start := clampScroll(m.projectScroll, len(entries), h-1)
	for i, e := range entries {
		if i < start {
			continue
		}
		if len(lines) >= h {
			break
		}
		t := e.t
		doneT := isDone(t.Project, t.Status)
		box := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(base).Render("○")
		textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(base)
		if doneT {
			box = s.mAccent.Render("✓")
			textStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(base)
		}
		indent := strings.Repeat("  ", e.depth)
		due := ""
		if t.Due != "" {
			if label, _ := gauges.DueLabel(t.Due, now); label != "" {
				due = label
			}
		}
		prio := priorityPlain(t.Priority)
		rowText := fmt.Sprintf("%s%s %s %s", indent, box, prio, truncatePlain(t.Title, w-30-e.depth*2))
		dueW := 10
		mstone := ""
		if t.Milestone != "" {
			if title, ok := mstoneTitles[t.Milestone]; ok {
				mstone = truncatePlain(title, dueW)
			}
		}
		line := fmt.Sprintf(" %s", textStyle.Render(rowText))
		fillW := dueW - fastWidth(mstone)
		if fillW < 0 {
			fillW = 0
		}
		fill := " " + mstone + strings.Repeat(" ", fillW) + due
		pad := w - fastWidth(line) - fastWidth(fill) - 1
		if pad < 1 {
			pad = 1
		}
		row := line + strings.Repeat(" ", pad) + muted.Render(fill)
		if i == m.projectSel {
			row = lipgloss.NewStyle().Background(lipgloss.Color(m.palette.SurfaceRaised)).Foreground(lipgloss.Color(m.palette.BrightText)).Bold(true).Render(padWidth(row, w))
		}
		lines = append(lines, row)
	}
	if len(entries) == 0 {
		lines = append(lines, "  "+muted.Render("no tasks in this project — mtm add \"...\" --project "+m.projectPage))
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return lines
}

func padWidth(s string, w int) string {
	if fastWidth(s) >= w {
		return clipStyled(s, w)
	}
	return s + strings.Repeat(" ", w-fastWidth(s))
}
