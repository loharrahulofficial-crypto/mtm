package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/gauges"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

// Sidebar rows: brand header (pinned), then VIEWS / SAVED VIEWS / COURSES
// sections. Selectable rows carry sel >= 0 (an index into sidebarItems).
func sidebarRows(items []sidebarItem) []sidebarRow {
	var rows []sidebarRow
	lastSection := ""
	sel := 0
	sectionOf := map[string]string{
		"home":    "Views",
		"smart":   "Views",
		"view":    "Saved views",
		"saved":   "Saved views",
		"course":  "Courses",
		"project": "Courses",
	}
	for _, it := range items {
		sec := sectionOf[it.Kind]
		if sec != lastSection {
			rows = append(rows, sidebarRow{header: sec, sel: -1})
			lastSection = sec
		}
		rows = append(rows, sidebarRow{sel: sel})
		sel++
	}
	return rows
}

// brandLines is the number of pinned lines above the scrollable sections.
const brandLines = 2

func renderSidebar(m Model, w, h int) string {
	innerW := w - 2
	if innerW < 1 {
		innerW = 1
	}

	lines := brandBlock(m, innerW)
	body := make([]string, 0, h)
	body = append(body, lines...)

	items := m.sidebarItems
	rows := sidebarRows(items)

	innerH := h - 2
	if innerH < 0 {
		innerH = 0
	}
	top, rendered, footerN, winH := sidebarRowPlan(m, innerW, innerH)

	// Pinned nearest-exam chip at the bottom of the sidebar.
	footer := []string{}
	if footerN > 0 {
		if chip := nearestExamChip(m, innerW-1); chip != "" {
			footer = []string{"", chip}
		}
	}

	count := 0
	for i := top; i < len(rows); i++ {
		if count >= rendered {
			break
		}
		r := rows[i]
		if r.sel < 0 {
			body = append(body, " "+sectionLabel(m, r.header, innerW-1))
			count++
			continue
		}
		it := items[r.sel]
		selected := r.sel == m.sidebarSel
		body = append(body, sidebarItemLine(m, it, innerW, selected))
		count++
	}

	// Ambient window fills the free rows below the list (never the chip).
	if win := m.ambientSidebarWindow(innerW, winH); win != nil {
		body = append(body, win...)
	}

	if len(footer) > 0 {
		for len(body) < innerH-len(footer) {
			body = append(body, "")
		}
		body = append(body, footer...)
	}

	return renderPanel(m, "", w, h, m.focus == focusSidebar, body)
}

// nearestExamChip renders the pinned bottom line: the nearest upcoming exam,
// e.g. " ✦ in 23d · DBMS midterm". Empty when there is no upcoming exam.
func nearestExamChip(m Model, w int) string {
	if w < 8 {
		return ""
	}
	now := m.clock.Now()
	todayStart := dayStartT(now)
	best := -1
	var bestTask *core.Task
	isDone := func(p, st string) bool { return m.store.IsDoneStatus(p, st) }
	for _, t := range m.store.Tasks() {
		if !strings.EqualFold(t.Kind, core.KindExam) {
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
		if best == -1 || diff < best {
			best = diff
			bestTask = t
		}
	}
	if bestTask == nil {
		return ""
	}
	surface := lipgloss.Color(m.palette.Surface)
	star := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(surface).Bold(true).Render("✦")
	muted := func(s string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(surface).Render(s)
	}
	txt := func(s string) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(surface).Render(s)
	}

	label, _ := gauges.DueLabel(bestTask.Due, now)
	build := func(lbl, title string) string {
		return " " + star + " " + muted(" "+lbl+" · ") + txt(title)
	}
	fixedW := func(lbl string) int { return 1 + 1 + 1 + 1 + lipgloss.Width(lbl) + 3 } // " ✦ " + " dl · "

	// 1) Full label, title word-truncated with an ellipsis.
	if title, ok := wordTruncate(bestTask.Title, w-fixedW(label)); ok {
		return build(label, title)
	}
	// 2) Last resort: character cut with an ellipsis.
	budget := w - fixedW(label)
	if budget < 1 {
		budget = 1
	}
	return build(label, truncatePlain(bestTask.Title, budget))
}

// wordTruncate fits s into budget cells by dropping whole trailing words
// and appending an ellipsis. Reports ok=false when even the first word
// does not fit (callers may fall back to a character cut).
func wordTruncate(s string, budget int) (string, bool) {
	if budget < 1 {
		return "", false
	}
	if lipgloss.Width(s) <= budget {
		return s, true
	}
	words := strings.Fields(s)
	cur := ""
	for _, word := range words {
		cand := word
		if cur != "" {
			cand = cur + " " + word
		}
		if lipgloss.Width(cand)+1 > budget { // +1 for the ellipsis
			break
		}
		cur = cand
	}
	if cur == "" {
		return "", false
	}
	return cur + "…", true
}

// brandBlock renders the pinned single-line wordmark: "mtm · study desk".
func brandBlock(m Model, w int) []string {
	s := m.styles
	line := " " + s.header.Render("mtm") + s.muted.Render(" · study desk")
	return []string{line, ""}
}

// sidebarItemLine renders one selectable row at innerW cells. The selected
// row always carries a full-width highlight — raised when the sidebar is
// focused, a dimmer blend when it is not.
func sidebarItemLine(m Model, it sidebarItem, w int, selected bool) string {
	fill := m.styles.surface
	if selected {
		if m.focus == focusSidebar {
			fill = m.styles.raised
		} else {
			dimRaised := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(lipgloss.Color(theme.Blend(m.palette.Surface, m.palette.SurfaceRaised, 0.55)))
			fill = dimRaised
		}
	}

	iconFor := func(name string) string { return m.icon(name) }

	var left, right string
	switch it.Kind {
	case "home":
		ic := iconFor("home")
		left = joinIcon(ic, "Home")
	case "smart":
		ic := iconFor("list")
		switch it.Key {
		case "all":
			ic = iconFor("list")
		case "today":
			ic = iconFor("due")
		case "week":
			ic = iconFor("calendar")
		case "overdue":
			ic = iconFor("warning")
		}
		if it.Key == "overdue" && it.Count > 0 {
			left = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.HeatOverdue)).Background(fill.GetBackground()).Bold(true).Render(joinIcon(ic, it.Name))
		} else {
			left = joinIcon(ic, it.Name)
		}
		if it.Count > 0 {
			right = fmt.Sprint(it.Count)
		}
	case "view", "saved":
		left = joinIcon(iconFor("tag"), it.Name)
		if it.Count > 0 {
			right = fmt.Sprint(it.Count)
		}
	case "course", "project":
		color := theme.CourseColor(m.palette, it.Key, projectColorOverride(m, it.Key))
		stripe := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(fill.GetBackground()).Render("▌")
		name := truncatePlain(leafName(it.Name), w-12)
		left = stripe + " " + name
		frac := 0.0
		if it.Total > 0 {
			frac = float64(it.Done) / float64(it.Total)
		}
		barFill, barTrack := gauges.ThinBar(frac, 4)
		subtleRow := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.SurfaceStrong)).Background(fill.GetBackground())
		bar := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(fill.GetBackground()).Render(barFill) +
			subtleRow.Render(barTrack)
		left = left + " " + bar
		// Linked project with a live commit streak: flame chip after the open
		// count (e.g. "12✦2d"). Only shown once the background load lands.
		streak, ok := m.githubStreak(it.Key)
		right = ""
		if it.Count > 0 {
			right = fmt.Sprint(it.Count)
		}
		if ok && streak > 0 {
			flame := fmt.Sprintf("✦%dd", streak)
			right = right + flame
		}
	}
	pad := w - lipgloss.Width(left) - lipgloss.Width(right) - 1
	if pad < 1 {
		pad = 1
	}
	line := " " + left + strings.Repeat(" ", pad) + right + " "
	return fill.Width(w).Render(line)
}

func joinIcon(ic, label string) string {
	if ic == "" {
		return label
	}
	return ic + " " + label
}

func mutedOn(m Model, bg lipgloss.Style, s string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(bg.GetBackground()).Render(s)
}

// projectColorOverride reads the project's color: from the store (cached,
// cheap). Empty when unset.
func projectColorOverride(m Model, name string) string {
	if p, err := m.store.GetProject(name); err == nil {
		return p.Color
	}
	return ""
}

func leafName(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}
