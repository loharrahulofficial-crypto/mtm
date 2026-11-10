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
	"github.com/loharrahulofficial-crypto/mtm/internal/dates"
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

// courseCardsCache memoizes the course read model across renders (spec
// 0.4). The key is (store id, store generation): any store mutation
// invalidates it, so a frame never renders stale stats, and repeated
// renders in one mutation epoch cost no recompute.
var courseCardsCache struct {
	mu    sync.Mutex
	id    uint64
	gen   uint64
	cards []courseCard
	err   error
	ok    bool
}

func collectCourseCards(m Model) ([]courseCard, error) {
	s := m.store
	id, gen := s.ID(), s.Generation()
	courseCardsCache.mu.Lock()
	defer courseCardsCache.mu.Unlock()
	if courseCardsCache.ok && courseCardsCache.id == id && courseCardsCache.gen == gen {
		return courseCardsCache.cards, courseCardsCache.err
	}
	cards, err := computeCourseCards(m)
	courseCardsCache.id, courseCardsCache.gen = id, gen
	courseCardsCache.cards, courseCardsCache.err = cards, err
	courseCardsCache.ok = true
	return cards, err
}

func computeCourseCards(m Model) ([]courseCard, error) {
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

// mainPaneSize computes the main pane's outer width/height as View()
// arranges it: full width minus the sidebar (and detail pane), footer
// excluded. Shared by the render path and key handlers so scroll bounds
// are measured on the same geometry.
func mainPaneSize(m Model) (w, h int) {
	sidebarW := sideWidth(m.width, m.sidebarHidden)
	detailW := detailWidth(m.width, m.detailOn)
	if m.view == viewCourses || m.view == viewStats || m.view == viewIdeas {
		detailW = 0 // read-only dashboards; no detail pane
	}
	gutters := 0
	if sidebarW > 0 {
		gutters++
	}
	if detailW > 0 {
		gutters++
	}
	w = m.width - sidebarW - detailW - gutters
	if w < 20 {
		w = 20
	}
	return w, m.height - 1 // footer
}

// coursesDetailFor measures the dashboard detail pane — width, visible
// height and the fully rendered body — on the same numbers the renderer
// uses, so key handlers can clamp the scroll without duplicating layout.
func coursesDetailFor(m Model, w, h int) (body []string, visible int) {
	cards, err := collectCourseCards(m)
	if err != nil || len(cards) == 0 {
		visible = h - 2
		if visible < 1 {
			visible = 1
		}
		return nil, visible
	}
	sel := m.courseSel
	if sel < 0 {
		sel = 0
	}
	if sel >= len(cards) {
		sel = len(cards) - 1
	}
	if w >= 100 {
		inner := w - 2
		listW := 28
		if listW > inner-20 {
			listW = inner - 20
		}
		if listW < 16 {
			listW = 16
		}
		gutter := 1
		visible = h - 2
		if visible < 1 {
			visible = 1
		}
		return renderCourseDetail(m, cards[sel], inner-listW-gutter, h-2), visible
	}
	listW := w - 2
	listH := len(cards) + 1
	if listH > 9 {
		listH = 9
	}
	rest := h - 2 - listH - 1
	if rest < 6 {
		rest = 6
	}
	return renderCourseDetail(m, cards[sel], listW, rest), rest
}

// renderCoursesBody renders the courses panel at exactly w×h.
func renderCoursesBody(m Model, w, h int) string {
	cards, err := collectCourseCards(m)
	hint := ""
	if err == nil {
		hint = fmt.Sprintf("%d active", len(cards))
	}
	if err != nil {
		return renderPanelBottom(m, "COURSES", w, h, m.focus == focusMain,
			[]string{"", " " + m.styles.muted.Render("courses: "+err.Error())}, "", "", hint)
	}
	if len(cards) == 0 {
		return renderPanelBottom(m, "COURSES", w, h, m.focus == focusMain,
			[]string{"", " " + m.styles.muted.Render("no courses yet — see `mtm course add -h`")}, "", "", hint)
	}
	if m.courseSel < 0 {
		m.courseSel = 0
	}
	if m.courseSel >= len(cards) {
		m.courseSel = len(cards) - 1
	}
	dashBody, dashVisible := coursesDetailFor(m, w, h)
	maxScroll := len(dashBody) - dashVisible
	if maxScroll < 0 {
		maxScroll = 0
	}
	scroll := m.coursesScroll
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
		return renderPanelBottom(m, "COURSES", w, h, m.focus == focusMain, body, centerHint, bottomHint, hint)
	}
	if w >= 100 {
		inner := w - 2
		listW := 28
		if listW > inner-20 {
			listW = inner - 20
		}
		if listW < 16 {
			listW = 16
		}
		gutter := 1
		rightW := inner - listW - gutter
		listBody := renderCourseList(m, cards, listW, h-2)
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
	// Stacked: list on top (content-sized), dashboard below.
	listW := w - 2
	listH := len(cards) + 1
	if listH > 9 {
		listH = 9
	}
	listBody := renderCourseList(m, cards, listW, listH)
	body := append(listBody, dashWindow...)
	return panel(body)
}

// blankSurfaceLine renders a surface-filled run of n cells.
func blankSurfaceLine(m Model, n int) string {
	if n < 0 {
		n = 0
	}
	return lipgloss.NewStyle().Background(lipgloss.Color(m.palette.Surface)).Render(strings.Repeat(" ", n))
}

// renderCourseList renders the compact course list: stripe, name, thin
// bar, grade %, next-exam chip. The selected row sits on raised.
func renderCourseList(m Model, cards []courseCard, w, h int) []string {
	surface := lipgloss.Color(m.palette.Surface)
	raised := lipgloss.Color(m.palette.SurfaceRaised)

	nameW := 6
	for _, c := range cards {
		if n := lipgloss.Width(leafName(c.project.Name)); n > nameW {
			nameW = n
		}
	}
	nameW = clampInt(nameW, 4, 14)
	lines := []string{}
	for i, c := range cards {
		if i >= h {
			break
		}
		selected := i == m.courseSel
		bg := surface
		if selected {
			bg = raised
		}
		color := theme.CourseColor(m.palette, c.project.Name, projectColorOverride(m, c.project.Name))
		stripe := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(bg).Render("▌")
		name := padRightPlain(truncatePlain(leafName(c.project.Name), nameW), nameW)
		nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(bg)
		if selected {
			nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.BrightText)).Background(bg).Bold(true)
		}
		pct := "   —"
		if c.grades != nil && len(c.grades.Items) > 0 {
			g := c.grades
			standing := g.StandingPct
			if g.Unweighted {
				standing = g.UnweightedPct
			}
			pct = fmt.Sprintf("%4.0f%%", clampFloat(standing, 0, 100))
		}
		pctStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(bg)
		if selected {
			pctStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.BrightText)).Background(bg).Bold(true)
		}
		chip := padRightPlain("", 5)
		if len(c.exams) > 0 {
			d := c.exams[0].daysLeft
			t := "today"
			switch {
			case d == 1:
				t = "1d"
			case d > 1:
				t = fmt.Sprintf("%dd", d)
			}
			if len(t) > 4 {
				t = t[:4]
			}
			chip = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(bg).Bold(true).Render(padRightPlain("✦"+t, 5))
		}
		line := " " + stripe + " " + nameStyle.Render(name) + " " + pctStyle.Render(pct) + " " + chip
		lines = append(lines, padToWidth(line, w, bg))
	}
	if len(cards) > len(lines) {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(surface).Render(fmt.Sprintf(" +%d more", len(cards)-len(lines))))
	}
	return lines
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
	switch g.TargetOutcome() {
	case core.TargetOutcomeNoTarget:
		return "no target grade — set one with `mtm course edit`"
	case core.TargetOutcomeFinalMet:
		return fmt.Sprintf("done — target met (%.1f%%)", g.StandingPct)
	case core.TargetOutcomeFinalMiss:
		return fmt.Sprintf("done — final %.1f%%, target %.0f%% not met", g.StandingPct, g.TargetPct)
	case core.TargetOutcomeSecured:
		return "target secured — nothing more required"
	case core.TargetOutcomeOnTrack:
		return fmt.Sprintf("on track — need %.1f%% on %s ungraded (%.1f%% margin)", g.NeededOnUngraded, trimFloat(g.UngradedWeight), g.MarginPct)
	default:
		return fmt.Sprintf("out of reach — even 100%% on the rest yields %.1f%%", g.BestCasePct())
	}
}

func trimFloat(f float64) string {
	return fmt.Sprintf("%g", f)
}

var _ = dates.HumanDay

// coursesScrollBounds measures the detail pane and returns the maximum
// scroll offset (0 when the content fits).
func (m Model) coursesScrollBounds() int {
	w, h := mainPaneSize(m)
	body, visible := coursesDetailFor(m, w, h)
	maxScroll := len(body) - visible
	if maxScroll < 0 {
		return 0
	}
	return maxScroll
}

// coursesPageStep is a half-pane scroll step (at least 3 lines).
func (m Model) coursesPageStep() int {
	w, h := mainPaneSize(m)
	_, visible := coursesDetailFor(m, w, h)
	step := visible / 2
	if step < 3 {
		step = 3
	}
	return step
}

// updateCoursesKey moves the course selection with j/k, opens the
// selected course's task list with enter, adds an exam with e and records
// a grade with g (both prefill the palette), and scrolls the detail pane
// with ctrl+d/ctrl+u (half page), PgDn/PgUp (half page via normalization
// in updateNormalKey) and the mouse wheel. The section order never
// changes; only the window shifts.
func (m Model) updateCoursesKey(act string) (tea.Model, tea.Cmd) {
	cards, err := collectCourseCards(m)
	n := 0
	if err == nil {
		n = len(cards)
	}
	prevSel := m.courseSel
	switch act {
	case aUp:
		m.courseSel -= 1
	case aDown:
		m.courseSel += 1
	case aTop:
		m.courseSel = 0
	case aBottom:
		m.courseSel = n - 1
	case aPageDown:
		m.coursesScroll = clampInt(m.coursesScroll+m.coursesPageStep(), 0, m.coursesScrollBounds())
		return m, nil
	case aPageUp:
		m.coursesScroll = clampInt(m.coursesScroll-m.coursesPageStep(), 0, m.coursesScrollBounds())
		return m, nil
	case aTitle: // "e" — add an exam to the selected course
		return m.openExamPalette(cards, err)
	case aEnter:
		return m.openCourseDetail(cards, err)
	}
	if m.courseSel < 0 {
		m.courseSel = 0
	}
	if m.courseSel >= n && n > 0 {
		m.courseSel = n - 1
	}
	if m.courseSel != prevSel {
		m.coursesScroll = 0
	}
	return m, nil
}

// openExamPalette opens the palette prefilled with "exam <course> " for
// the selected course.
func (m Model) openExamPalette(cards []courseCard, err error) (tea.Model, tea.Cmd) {
	if err == nil && m.courseSel >= 0 && m.courseSel < len(cards) {
		cmd := m.openPaletteWith("exam " + cards[m.courseSel].project.Name + " ")
		return m, cmd
	}
	return m, m.openPalette()
}

// openGradePalette opens the palette prefilled with "grade <id> " for the
// first ungraded weighted item of the selected course (or "grade ").
func (m Model) openGradePalette() (tea.Model, tea.Cmd) {
	prefill := "grade "
	if cards, err := collectCourseCards(m); err == nil && m.courseSel >= 0 && m.courseSel < len(cards) {
		if g := cards[m.courseSel].grades; g != nil {
			for _, it := range g.Items {
				if !it.Scored {
					prefill = "grade " + it.ID + " "
					break
				}
			}
		}
	}
	cmd := m.openPaletteWith(prefill)
	return m, cmd
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
	m.coursesDrillFrom = m.courseSel
	m.view = viewList
	m.refreshTasks()
	m.cursor = 0
	m.clampCursor()
	return m, nil
}
