package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/gauges"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

// Course dashboard: one card per course with progress (subtasks included),
// the grades rollup, study hours and upcoming exams. Scroll with j/k.

// courseCard bundles everything one course row needs.
type courseCard struct {
	project  *core.Project
	stats    *actions.CourseStats
	grades   *core.CourseGrades
	hours    actions.HoursRollup
	hasHours bool
	exams    []courseExam
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
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
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

// renderCoursesBody renders the course dashboard at exactly w×h.
func renderCoursesBody(m Model, w, h int) string {
	bg := lipgloss.Color(m.palette.Surface)

	cards, err := collectCourseCards(m)
	var body []string
	if err != nil {
		body = append(body, " "+m.styles.muted.Render("courses: "+err.Error()))
		return paint(strings.Join(body, "\n"), w, h, bg)
	}
	if len(cards) == 0 {
		body = append(body, " "+m.styles.muted.Render("no courses yet — mark one with `mtm course add <name> --code XXX --name \"...\"`"))
		return paint(strings.Join(body, "\n"), w, h, bg)
	}
	body = append(body, coursesHeaderLine(m, w, len(cards)))
	for i, card := range cards {
		if i > 0 {
			body = append(body, "")
		}
		body = append(body, renderCourseCard(m, card, w)...)
	}
	return paint(clipBlock(body, m.courseScroll, w, h, bg), w, h, bg)
}

// coursesHeaderLine renders the view's own section label row.
func coursesHeaderLine(m Model, w, n int) string {
	muted := m.styles.muted
	label := muted.Render(letterspace("C O U R S E S"))
	count := muted.Render(fmt.Sprintf("%d active", n))
	pad := w - lipgloss.Width(label) - lipgloss.Width(count) - 1
	if pad < 1 {
		pad = 1
	}
	return " " + label + strings.Repeat(" ", pad) + count
}

func renderCourseCard(m Model, card courseCard, w int) []string {
	s := m.styles
	bg := lipgloss.Color(m.palette.Surface)
	p := card.project
	course := p.Course
	name := p.Name
	if course != nil && strings.TrimSpace(course.Code) != "" {
		name = course.Code
		if strings.TrimSpace(course.Name) != "" && !strings.EqualFold(course.Code, course.Name) {
			name = course.Code + " — " + course.Name
		}
	}
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
	stripeColor := theme.CourseColor(m.palette, p.Name, projectColorOverride(m, p.Name))
	stripe := lipgloss.NewStyle().Foreground(lipgloss.Color(stripeColor)).Background(bg).Render("▌")
	nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.BrightText)).Background(bg).Bold(true)
	metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(bg)
	header := " " + stripe + " " + nameStyle.Render(truncatePlain(name, w/2))
	if len(meta) > 0 {
		header += "  " + metaStyle.Render(strings.Join(meta, " · "))
	}
	lines := []string{padToWidth(header, w, bg)}
	muted := s.muted
	barW := clampInt(w/4, 12, 30)

	// Overall progress (subtasks included).
	overall := gaugesBarColored(m, card.stats.Overall, barW, stripeColor)
	wline := fmt.Sprintf("  overall   %s %s", overall, muted.Render(fmt.Sprintf("%d/%d done", card.stats.Overall.Done, card.stats.Overall.Total)))
	if card.stats.HasWeights {
		wp := card.stats.OverallWeighted
		wline += muted.Render(fmt.Sprintf("   weighted %3.0f%%", wp.Fraction*100))
	}
	lines = append(lines, padToWidth(wline, w, bg))

	// Grades.
	lines = append(lines, gradesLines(m, card.grades, w)...)

	// Hours.
	hoursLine := "  hours     " + muted.Render("nothing logged yet — F starts a pomodoro")
	if card.hasHours && (card.hours.LoggedMinutes > 0 || card.hours.EstimateMinutes > 0) {
		hoursLine = fmt.Sprintf("  hours     %s logged", actions.FormatMinutes(card.hours.LoggedMinutes))
		if card.hours.EstimateMinutes > 0 {
			pct := float64(card.hours.LoggedMinutes) / float64(card.hours.EstimateMinutes) * 100
			hoursLine += muted.Render(fmt.Sprintf(" · est %s (%.0f%% of estimate)", actions.FormatMinutes(card.hours.EstimateMinutes), pct))
		}
	}
	lines = append(lines, padToWidth(hoursLine, w, bg))

	// Exams with countdowns.
	if len(card.exams) == 0 {
		lines = append(lines, padToWidth("  exams     "+muted.Render("no upcoming exams"), w, bg))
	} else {
		show := 3
		for i, ce := range card.exams {
			if i >= show {
				lines = append(lines, padToWidth(fmt.Sprintf("            %s", muted.Render(fmt.Sprintf("+%d more", len(card.exams)-i))), w, bg))
				break
			}
			label := ce.task.Title
			prefix := "  exams     "
			if i > 0 {
				prefix = "            "
			}
			weight := ""
			if ce.task.Weight != nil && *ce.task.Weight > 0 {
				weight = fmt.Sprintf(" · w %s", trimFloat(*ce.task.Weight))
			}
			count, _ := gauges.DueLabel(ce.task.Due, m.clock.Now())
			line := prefix + accentStar(m) + " " + lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(bg).Render(truncatePlain(label, w-34)) + muted.Render(" — "+count+weight)
			lines = append(lines, padToWidth(line, w, bg))
		}
	}
	return lines
}

func trimFloat(f float64) string {
	s := fmt.Sprintf("%g", f)
	return s
}

func gradesLines(m Model, g *core.CourseGrades, w int) []string {
	bg := lipgloss.Color(m.palette.Surface)
	muted := m.styles.muted
	label := "  grades    "
	if g == nil || len(g.Items) == 0 {
		return []string{padToWidth(label+muted.Render("no assignments or exams yet"), w, bg)}
	}
	if g.Unweighted {
		return []string{padToWidth(label+muted.Render(fmt.Sprintf("avg %.1f%% · no weights set", g.UnweightedPct)), w, bg)}
	}
	var lines []string
	first := fmt.Sprintf("%s%s", label, summaryGrades(g))
	lines = append(lines, padToWidth(first, w, bg))
	cont := "            "
	switch {
	case !g.HasTarget:
		lines = append(lines, padToWidth(cont+muted.Render("no target grade — set one with `mtm course edit`"), w, bg))
	case g.RemainingWeight == 0:
		if g.StandingPct >= g.TargetPct {
			lines = append(lines, padToWidth(cont+muted.Render(fmt.Sprintf("done — target met (%.1f%%)", g.StandingPct)), w, bg))
		} else {
			lines = append(lines, padToWidth(cont+muted.Render(fmt.Sprintf("done — final %.1f%%, target %.0f%% not met", g.StandingPct, g.TargetPct)), w, bg))
		}
	case g.NeededSecured:
		lines = append(lines, padToWidth(cont+muted.Render("target secured — nothing more required"), w, bg))
	case g.NeededFeasible:
		lines = append(lines, padToWidth(cont+muted.Render(fmt.Sprintf("need %.1f%% on remaining %s", g.NeededOnRemaining, trimFloat(g.RemainingWeight))), w, bg))
	default:
		best := 0.0
		if g.TotalWeight > 0 {
			best = (g.StandingPct*g.TotalWeight + 100*g.RemainingWeight) / g.TotalWeight
		}
		lines = append(lines, padToWidth(cont+muted.Render(fmt.Sprintf("out of reach — even 100%% on the rest yields %.1f%%", best)), w, bg))
	}
	if g.TotalWeightOff {
		lines = append(lines, padToWidth(cont+muted.Render(fmt.Sprintf("weights total %s, not 100", trimFloat(g.TotalWeight))), w, bg))
	}
	return lines
}

func summaryGrades(g *core.CourseGrades) string {
	return fmt.Sprintf("current %.1f%% · standing %.1f%% of %s", g.CurrentPct, g.StandingPct, trimFloat(g.TotalWeight))
}

func accentStar(m Model) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(lipgloss.Color(m.palette.Surface)).Bold(true).Render("✦")
}

// gaugesBarColored renders a filled/tracked bar in the course accent color.
func gaugesBarColored(m Model, p core.Progress, cells int, color string) string {
	fill, track := gauges.Bar(p.Fraction, cells)
	bg := lipgloss.Color(m.palette.Surface)
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(bg).Render(fill) +
		m.styles.muted.Render(fmt.Sprintf("%s %3.0f%%", track, p.Fraction*100))
}

// clipBlock returns the h-line window of lines starting at offset. Lines
// are already width-clipped by paint; scroll is cosmetic only.
func clipBlock(lines []string, offset, w, h int, surface lipgloss.Color) string {
	total := len(lines)
	maxOffset := total - h
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	var out []string
	for i := offset; i < total && len(out) < h; i++ {
		out = append(out, lines[i])
	}
	for len(out) < h {
		out = append(out, padToWidth("", w, surface))
	}
	return strings.Join(out, "\n")
}

// updateCoursesKey scrolls the course dashboard.
func (m Model) updateCoursesKey(act string) (tea.Model, tea.Cmd) {
	switch act {
	case aDown:
		m.courseScroll++
	case aUp:
		m.courseScroll--
	case aPageDown:
		m.courseScroll += m.height / 2
	case aPageUp:
		m.courseScroll -= m.height / 2
	case aTop:
		m.courseScroll = 0
	case aBottom:
		m.courseScroll = 1 << 30
	}
	if m.courseScroll < 0 {
		m.courseScroll = 0
	}
	return m, nil
}
