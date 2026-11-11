package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ambient_sidebar.go — the sidebar ambient window (spec 5.1): a framed
// panel pinned below the course list, on every view, filling only the
// rows the list leaves free. Never overlaps content; collapses to nothing
// on short terminals; hidden by ui.ambient_sidebar: false and by :ambient.

const (
	ambientSidebarMinRows  = 8  // incl. the border rows
	ambientSidebarMinWidth = 16 // incl. the border cells
)

// sidebarRowPlan is the single source of truth for the sidebar's row
// budget: brand block, scrollable sections (selection visible), optional
// ambient window, pinned exam chip. winH > 0 means the window fits.
func sidebarRowPlan(m Model, innerW, innerH int) (top, rendered, footer, winH int) {
	footer = 0
	if chip := nearestExamChip(m, innerW-1); chip != "" {
		footer = 2
	}
	rows := sidebarRows(m.sidebarItems)
	selRow := 0
	for i, r := range rows {
		if r.sel == m.sidebarSel {
			selRow = i
		}
	}
	avail := innerH - brandLines - footer
	if avail < 1 {
		avail = 1
	}
	if selRow >= avail {
		top = selRow - avail + 1
	}
	rendered = len(rows) - top
	if rendered > avail {
		rendered = avail
	}
	if m.ambientSidebarWindowWanted() {
		if free := avail - rendered; free >= ambientSidebarMinRows && innerW >= ambientSidebarMinWidth {
			winH = free
		}
	}
	return top, rendered, footer, winH
}

// sidebarAmbientWindowWanted reports whether the sidebar window may show:
// the ambient surface allows sidebars and ui.ambient_sidebar is on.
func (m Model) ambientSidebarWindowWanted() bool {
	return m.ambientAllowSidebar() && m.cfg.UI.AmbientSidebar
}

// ambientAllowSidebar reports whether ui.ambient enables the sidebar window.
func (m Model) ambientAllowSidebar() bool {
	switch m.cfg.UI.Ambient {
	case "sidebar", "both":
		return true
	}
	return false
}

// sidebarFreeRows reports whether the sidebar is visible and the window
// fits, and the inner width. The layout pass returns ok=false when hidden.
func sidebarFreeRows(m Model) (free, innerW int, ok bool) {
	w := sideWidth(m.width, m.sidebarHidden)
	if w <= 0 {
		return 0, 0, false
	}
	innerW = w - 2
	if innerW < 1 {
		innerW = 1
	}
	innerH := m.height - 1 - 2
	if innerH < 0 {
		innerH = 0
	}
	_, _, _, winH := sidebarRowPlan(m, innerW, innerH)
	return winH, innerW, winH >= ambientSidebarMinRows
}

// slabSceneLines renders the scene at w×h on a Surface-based palette so the
// frame blends into the pane slab (never punches Base cells into it).
func (m Model) slabSceneLines(w, h int) []string {
	slab := m.scenePalette()
	slab.Base = m.palette.Surface
	st := m.sceneState()
	st.Palette = slab
	return m.currentScene().Frame(m.ambientT, w, h, st)
}

// ambientSidebarWindow renders the window filling exactly winH rows and
// innerW cells, or nil when it doesn't fit / isn't wanted.
func (m Model) ambientSidebarWindow(innerW, winH int) []string {
	if winH < ambientSidebarMinRows || innerW < ambientSidebarMinWidth {
		return nil
	}
	return renderSideWindow(m, innerW, winH)
}

// renderSideWindow draws the framed ambient window: rounded border, title
// = the scene name, scene below. Every line is exactly w cells.
func renderSideWindow(m Model, w, h int) []string {
	surface := lipgloss.Color(m.palette.Surface)
	edge := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.AccentSoft)).Background(surface)
	pad := lipgloss.NewStyle().Background(surface)
	title := strings.ToUpper(m.currentSceneName())
	top := titledBorder(m, title, w, true, edge)
	bot := titledBorder(m, "", w, false, edge)
	right := edge.Render("│")
	out := []string{top}
	for _, l := range m.slabSceneLines(w-2, h-2) {
		if lw := fastWidth(l); lw > w-2 {
			l = clipStyled(l, w-2)
		} else if lw < w-2 {
			l += pad.Render(strings.Repeat(" ", w-2-lw))
		}
		out = append(out, edge.Render("│")+l+right)
	}
	out = append(out, bot)
	return out
}
