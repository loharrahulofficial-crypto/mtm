package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

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
	innerW := w - 2*1 // no border, padX = 1 each side (panel convention)
	if innerW < 1 {
		innerW = 1
	}

	lines := brandBlock(m, innerW)
	body := make([]string, 0, h)
	body = append(body, lines...)

	items := m.sidebarItems
	rows := sidebarRows(items)

	// Budget for the scrollable part; selection stays visible.
	selRow := 0
	for i, r := range rows {
		if r.sel == m.sidebarSel {
			selRow = i
		}
	}
	avail := h - len(lines)
	if avail < 1 {
		avail = 1
	}
	top := 0
	if selRow >= avail {
		top = selRow - avail + 1
	}

	line := 0
	for i, r := range rows {
		if i < top {
			continue
		}
		if line >= avail {
			break
		}
		if r.sel < 0 {
			body = append(body, " "+sectionLabel(m, r.header, innerW-1))
			line++
			continue
		}
		it := items[r.sel]
		selected := r.sel == m.sidebarSel
		body = append(body, sidebarItemLine(m, it, innerW, selected))
		line++
	}

	_ = line
	return renderPanel(m, "", w, h, m.focus == focusSidebar, body)
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
		barFill, barTrack := gauges.Bar(frac, 4)
		bar := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(fill.GetBackground()).Render(barFill) +
			mutedOn(m, fill, barTrack)
		left = left + " " + bar
		if it.Count > 0 {
			right = fmt.Sprint(it.Count)
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
