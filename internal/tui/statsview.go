package tui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/gauges"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

// Stats view: a master-detail dashboard mirroring the Courses view. The
// left column lists the time scopes (this week / month / semester / 52
// weeks), the right pane shows the selected report: completed count and
// on-time rate, hours per day, hours by course, streak, the shared
// activity heatmap and the estimate-accuracy rollup. Read-only;
// ctrl+d/ctrl+u, PgDn/PgUp and the mouse wheel scroll the details
// (same pattern as the Courses pane), with ▲ more / ▼ more indicators on
// the panel borders when the content clips.

const (
	statsScopeWeek = iota
	statsScopeMonth
	statsScopeSemester
	statsScopeAll
	statsScopeCount
)

type statsScopeDef struct {
	label  string // THIS WEEK
	period string // week|month|semester|weeks
	since  func(now time.Time) time.Time
}

func statsScopes(m Model) []statsScopeDef {
	return []statsScopeDef{
		{"THIS WEEK", "week", func(now time.Time) time.Time { return now.AddDate(0, 0, -6) }},
		{"THIS MONTH", "month", func(now time.Time) time.Time { return now.AddDate(0, 0, -29) }},
		{"THIS SEMESTER", "semester", func(now time.Time) time.Time {
			if raw := strings.TrimSpace(m.cfg.UI.SemesterStart); raw != "" {
				if t, err := time.ParseInLocation("2006-01-02", raw, now.Location()); err == nil {
					return t
				}
			}
			return time.Time{}
		}},
		{"52 WEEKS", "weeks52", func(now time.Time) time.Time { return now.AddDate(0, -12, 0) }},
	}
}

// statsCache memoizes the per-scope reports across renders (invalidated by
// store id + generation, like the course cards).
var statsCache struct {
	mu      sync.Mutex
	id      uint64
	gen     uint64
	reports map[string]*actions.StatsReport
}

func statsReport(m Model, def statsScopeDef, now time.Time) (*actions.StatsReport, error) {
	s := m.store
	id, gen := s.ID(), s.Generation()
	statsCache.mu.Lock()
	defer statsCache.mu.Unlock()
	if statsCache.id != id || statsCache.gen != gen {
		statsCache.id, statsCache.gen, statsCache.reports = id, gen, map[string]*actions.StatsReport{}
	}
	if r, ok := statsCache.reports[def.label]; ok {
		return r, nil
	}
	since := def.since(now)
	rep, err := actions.CollectStats(s, now, since, def.period, m.cfg.Stats.StreakMinMinutes)
	if err != nil {
		return nil, err
	}
	statsCache.reports[def.label] = rep
	return rep, nil
}

func (m Model) selectedStatsScope() statsScopeDef {
	scopes := statsScopes(m)
	sel := m.statsSel
	if sel < 0 {
		sel = 0
	}
	if sel >= len(scopes) {
		sel = len(scopes) - 1
	}
	return scopes[sel]
}

// statsScrollBounds measures the details pane for the current scope.
func (m Model) statsScrollBounds() int {
	w, h := mainPaneSize(m)
	body, visible := statsDetailFor(m, w, h)
	maxScroll := len(body) - visible
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}

// statsPageStep is a half-pane scroll step (at least 3 lines).
func (m Model) statsPageStep() int {
	w, h := mainPaneSize(m)
	_, visible := statsDetailFor(m, w, h)
	step := visible / 2
	if step < 3 {
		step = 3
	}
	return step
}

func (m Model) updateStatsKey(act string) (tea.Model, tea.Cmd) {
	prevSel := m.statsSel
	switch act {
	case aUp, aLeft:
		m.statsSel--
	case aDown, aRight:
		m.statsSel++
	case aTop:
		m.statsSel = 0
	case aBottom:
		m.statsSel = statsScopeCount - 1
	case aPageDown:
		m.statsScroll = clampInt(m.statsScroll+m.statsPageStep(), 0, m.statsScrollBounds())
		return m, nil
	case aPageUp:
		m.statsScroll = clampInt(m.statsScroll-m.statsPageStep(), 0, m.statsScrollBounds())
		return m, nil
	}
	if m.statsSel < 0 {
		m.statsSel = 0
	}
	if m.statsSel >= statsScopeCount {
		m.statsSel = statsScopeCount - 1
	}
	if m.statsSel != prevSel {
		m.statsScroll = 0
	}
	return m, nil
}

// statsDetailFor renders the details pane for the current scope and returns
// its full body and visible height (shared by render and key handling so
// scroll bounds are measured on the same geometry).
func statsDetailFor(m Model, w, h int) (body []string, visible int) {
	if w >= 100 {
		inner := w - 2
		listW := 24
		gutter := 1
		return renderStatsDetail(m, inner-listW-gutter, h-2), h - 2
	}
	// Stacked: list on top (content-sized), dashboard below.
	listH := statsScopeCount + 2
	rest := h - 2 - listH - 1
	if rest < 6 {
		rest = 6
	}
	return renderStatsDetail(m, w-2, rest), rest
}

// renderStatsBody renders the stats panel at exactly w×h.
func renderStatsBody(m Model, w, h int) string {
	scopes := statsScopes(m)
	hint := fmt.Sprintf("%d scopes", len(scopes))
	now := m.clock.Now()

	if m.statsSel < 0 {
		m.statsSel = 0
	}
	if m.statsSel >= len(scopes) {
		m.statsSel = len(scopes) - 1
	}

	dashBody, dashVisible := statsDetailFor(m, w, h)
	maxScroll := len(dashBody) - dashVisible
	if maxScroll < 0 {
		maxScroll = 0
	}
	scroll := m.statsScroll
	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	end := scroll + dashVisible
	if end > len(dashBody) {
		end = len(dashBody)
	}
	dashWindow := dashBody[scroll:end]
	centerHint, bottomHint := "", ""
	if scroll > 0 {
		centerHint = "▲ more"
	}
	if scroll+dashVisible < len(dashBody) {
		bottomHint = "▼ more"
	}
	panel := func(body []string) string {
		return renderPanelBottom(m, "STATS", w, h, m.focus == focusMain, body, centerHint, bottomHint, hint)
	}

	if w >= 100 {
		inner := w - 2
		listW := 24
		gutter := 1
		rightW := inner - listW - gutter
		listBody := renderStatsList(m, scopes, now, listW)
		lines := []string{}
		for i := 0; i < maxInt(len(listBody), len(dashWindow)); i++ {
			l, r := blankSurfaceLine(m, listW), blankSurfaceLine(m, rightW)
			if i < len(listBody) {
				l = listBody[i]
			}
			if i < len(dashWindow) {
				r = dashWindow[i]
			}
			lines = append(lines, l+blankSurfaceLine(m, gutter)+r)
		}
		return panel(lines)
	}
	// Stacked: list on top, dashboard below.
	listW := w - 2
	listBody := renderStatsList(m, scopes, now, listW)
	rest := h - 2 - len(listBody) - 1
	if rest < 6 {
		rest = 6
	}
	body := append(listBody, dashWindow...)
	return panel(body)
}

// renderStatsList renders the scope list (selected row on raised).
func renderStatsList(m Model, scopes []statsScopeDef, now time.Time, w int) []string {
	surface := lipgloss.Color(m.palette.Surface)
	raised := lipgloss.Color(m.palette.SurfaceRaised)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(surface)

	rep := map[string]*actions.StatsReport{}
	for _, def := range scopes {
		if r, err := statsReport(m, def, now); err == nil {
			rep[def.label] = r
		}
	}

	lines := []string{padToWidth(" "+muted.Render("S C O P E S"), w, surface)}
	for i, def := range scopes {
		selBG := surface
		if i == m.statsSel {
			selBG = raised
		}
		stripe := "  "
		if i == m.statsSel {
			stripe = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(selBG).Render("▌") + " "
		}
		nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(selBG)
		count := "—"
		if r, ok := rep[def.label]; ok {
			count = fmt.Sprintf("%d", r.Completed)
		} else if def.label == "THIS SEMESTER" && strings.TrimSpace(m.cfg.UI.SemesterStart) == "" {
			count = "off"
		}
		countStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(selBG)
		if i == m.statsSel {
			countStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.BrightText)).Background(selBG).Bold(true)
		}
		label := truncatePlain(def.label, w-8)
		line := " " + stripe + nameStyle.Render(label)
		cw := lipgloss.Width(countStyle.Render(" " + count + " "))
		if pad := w - lipgloss.Width(line) - cw; pad > 0 {
			line += lipgloss.NewStyle().Background(selBG).Render(strings.Repeat(" ", pad))
		}
		line += countStyle.Render(" " + count + " ")
		lines = append(lines, padToWidth(line, w, selBG))
	}
	return lines
}

func renderStatsDetail(m Model, w, h int) []string {
	if h < 1 {
		h = 1
	}
	bg := lipgloss.Color(m.palette.Surface)
	surfaceBG := lipgloss.NewStyle().Background(bg)

	bright := func(s string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.BrightText)).Background(bg).Bold(true).Render(s)
	}
	muted := func(s string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(bg).Render(s)
	}
	mutedFG := func(s string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bg).Render(s)
	}
	accent := func(s string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(bg).Bold(true).Render(s)
	}

	scope := m.selectedStatsScope()
	now := m.clock.Now()
	rep, err := statsReport(m, scope, now)
	if err != nil {
		return []string{"", " " + muted("stats unavailable: "+err.Error())}
	}
	_ = rep.Days // CollectStats caps the day arrays internally

	lines := []string{}
	pad := func(line string) string { return padToWidth(line, w, bg) }

	// Semester requested but not configured → a single hint instead of a
	// misleading zero-filled report.
	if scope.label == "THIS SEMESTER" && strings.TrimSpace(m.cfg.UI.SemesterStart) == "" {
		lines = append(lines, pad(""))
		lines = append(lines, pad(" "+bright("semester stats need an anchor")))
		lines = append(lines, pad(" "+muted("set ui.semester_start: \"2026-09-01\" in config.yaml")))
		lines = append(lines, pad(" "+muted("then the semester figures appear here")))
		return lines
	}

	// --- Summary: big digits + on-time beside them -----------------------
	digits := gauges.Digits(fmt.Sprintf("%d", rep.Completed))
	summary := []string{}
	right := []string{
		muted("C O M P L E T E D"),
		bright(fmt.Sprintf("%d task%s", rep.Completed, pluralS(rep.Completed))),
		"",
	}
	for row := 0; row < 3; row++ {
		l := "  " + accent(digits[row])
		if row < len(right) && right[row] != "" {
			l += "   " + right[row]
		}
		summary = append(summary, pad(l))
	}
	summary = append(summary,
		pad(" "+muted(fmt.Sprintf("%.1fh logged%s", rep.TotalHours(), onTimeFragment(rep)))),
		pad(""),
	)
	lines = append(lines, summary...)

	// --- Hours per day (last 14 days max) --------------------------------
	chartStart := len(rep.DayHours)
	if chartStart > 14 {
		chartStart -= 14
	} else {
		chartStart = 0
	}
	chartValues := rep.DayHours[chartStart:]
	chartDays := 14
	if len(chartValues) < 14 {
		chartDays = len(chartValues)
	}
	_ = chartDays
	todayIdx := len(chartValues) - 1
	letters := []string{}
	for i := range chartValues {
		d := rep.Since.AddDate(0, 0, chartStart+i)
		letters = append(letters, strings.ToUpper(d.Format("Mon")[:1]))
	}
	barH := 6
	if h > 30 {
		barH = 7
	}
	lines = append(lines, detailSection(m, w, "Hours per day"))
	chart := gauges.RenderBarChart(gauges.BarChartConfig{
		Values:     chartValues,
		Labels:     letters,
		Width:      w,
		Height:     barH + 3,
		MaxBarRows: barH,
		BarWidth:   2,
		Today:      todayIdx,
		Accent:     accent,
		Bar:        func(s string) string { return accent(s) },
		Muted:      mutedFG,
		Bright:     bright,
		Bg:         func(s string) string { return surfaceBG.Render(s) },
	})
	for _, l := range chart {
		lines = append(lines, pad(" "+l))
	}
	lines = append(lines, pad(" "+muted(fmt.Sprintf("%.1f h · %s", rep.TotalHours(), statsChartWindowLabel(rep, chartStart)))))
	lines = append(lines, pad(""))

	// --- Hours by course ---------------------------------------------------
	lines = append(lines, detailSection(m, w, "Hours by course"))
	maxCourse := 0.0
	for _, c := range rep.Courses {
		if c.Hours > maxCourse {
			maxCourse = c.Hours
		}
	}
	maxCourse = maxFloat(maxCourse, rep.OtherHours)
	courseW := w - 4
	nameW := 18
	if courseW < 60 {
		nameW = 12
	}
	for _, c := range rep.Courses {
		if c.Completed == 0 && c.Hours == 0 {
			continue
		}
		lines = append(lines, pad(" "+statsCourseRow(m, c.Course, c.Hours, c.Completed, maxCourse, w, nameW, bg, muted, accent)))
	}
	if rep.OtherCompleted > 0 || rep.OtherHours > 0 {
		lines = append(lines, pad(" "+statsCourseRow(m, "other projects", rep.OtherHours, rep.OtherCompleted, maxCourse, w, nameW, bg, muted, accent)))
	}
	if len(rep.Courses) == 0 && rep.OtherCompleted == 0 {
		lines = append(lines, pad(" "+muted("no course activity in this period")))
	}
	lines = append(lines, pad(""))

	// --- Streak -------------------------------------------------------------
	lines = append(lines, detailSection(m, w, "Streak"))
	streak := rep.Streaks
	if streak.Current > 0 {
		frac := 1.0
		if streak.Longest > 0 {
			frac = float64(streak.Current) / float64(streak.Longest)
		}
		fill, track := gauges.ThinBar(frac, clampInt(w/3, 8, 24))
		chip := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(bg).Bold(true).Render(strings.TrimSpace(m.icon("flame") + " "))
		lines = append(lines, pad(" "+chip+bright("streak "+fmt.Sprint(streak.Current)+"d")+muted("  ·  best "+fmt.Sprint(streak.Longest)+"d")))
		bar := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(bg).Render(fill) +
			lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.SurfaceStrong)).Background(bg).Render(track)
		lines = append(lines, pad(" "+bar))
		if streak.Current >= streak.Longest && streak.Longest > 1 {
			lines = append(lines, pad(" "+muted("you are at your best — longest ever")))
		}
	} else {
		lines = append(lines, pad(" "+muted("no active days yet — finish a task or log time to start a streak")))
	}
	lines = append(lines, pad(""))

	// --- Estimate accuracy ---------------------------------------------------
	lines = append(lines, detailSection(m, w, "Estimates"))
	if rep.Estimate.Total == 0 {
		lines = append(lines, pad(" "+muted("no completed tasks to score against estimates yet")))
	} else {
		lines = append(lines, pad(" "+accent(estimateLabelTUI(rep.Estimate))))
	}
	lines = append(lines, pad(""))

	// --- Activity heatmap ----------------------------------------------------
	hd := m.homeDataCached()
	lines = append(lines, detailSection(m, w, "Activity"))
	for _, l := range heatmapBody(m, w, 9, hd) {
		lines = append(lines, pad(l))
	}
	return lines
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func onTimeFragment(rep *actions.StatsReport) string {
	if rep.WithDue == 0 {
		return ""
	}
	return fmt.Sprintf("  ·  on-time %.0f%% (%d of %d)", rep.OnTimePct(), rep.OnTime, rep.WithDue)
}

func statsChartWindowLabel(rep *actions.StatsReport, startIdx int) string {
	first := rep.Since.AddDate(0, 0, startIdx)
	last := rep.Since.AddDate(0, 0, len(rep.DayHours)-1)
	if startIdx == 0 {
		switch rep.Period {
		case "week":
			return "last 7 days"
		case "month":
			return "last 30 days"
		default:
			return "last " + fmt.Sprint(rep.Days) + " days"
		}
	}
	return fmt.Sprintf("%s → %s", first.Format("2 Jan"), last.Format("2 Jan"))
}

func statsCourseRow(m Model, name string, hours float64, done int, maxHours float64, w int, nameW int, bg lipgloss.Color, muted, accent func(string) string) string {
	color := theme.CourseColor(m.palette, name, projectColorOverride(m, name))
	stripe := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(bg).Render("▌")
	name = truncatePlain(leafName(name), nameW)
	padName := nameW - len([]rune(name))
	if padName < 0 {
		padName = 0
	}
	frac := 0.0
	if maxHours > 0 {
		frac = hours / maxHours
	}
	fill, track := gauges.ThinBar(frac, 12)
	bar := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(bg).Render(fill) +
		lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.SurfaceStrong)).Background(bg).Render(track)
	hoursCell := fmt.Sprintf("%4.1fh", hours)
	nameStr := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(bg).Render(name + strings.Repeat(" ", padName))
	return " " + stripe + " " + nameStr + " " + bar + " " + accent(hoursCell) + muted(fmt.Sprintf("  %d done", done))
}

func estimateLabelTUI(a core.EstimateAccuracy) string {
	if a.Total == 0 {
		return "no completed tasks to score"
	}
	label := fmt.Sprintf("%d of %d tasks estimated", a.Estimated, a.Total)
	if a.Enough {
		label += fmt.Sprintf("  ·  %.0f%% of estimate", a.Pct())
	}
	return label
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
