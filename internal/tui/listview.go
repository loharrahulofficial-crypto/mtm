package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/gauges"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

func (m Model) updateListKey(act string) (tea.Model, tea.Cmd) {
	switch act {
	case aUp:
		m.moveSelection(-1)
	case aDown:
		m.moveSelection(+1)
	case aPageUp:
		m.moveSelection(-m.visibleListRows()/2 - 1)
	case aPageDown:
		m.moveSelection(+m.visibleListRows()/2 + 1)
	case aTop:
		m.cursor = 0
		m.ensureCursorVisible()
	case aBottom:
		if len(m.rows) > 0 {
			m.cursor = len(m.rows) - 1
		}
		m.ensureCursorVisible()
	case aLeft:
		if m.focus == focusDetail || m.focus == focusSidebar {
			m.focus = focusMain
			return m, nil
		}
		if sideWidth(m.width, m.sidebarHidden) > 0 {
			m.focus = focusSidebar
		}
	case aRight:
		if m.focus == focusSidebar {
			m.focus = focusMain
			return m, nil
		}
		if detailWidth(m.width, m.detailOn) > 0 {
			m.focus = focusDetail
		}
	default:
		if m2, cmd, handled := m.updateSimpleTaskAction(act); handled {
			return m2, cmd
		}
	}
	return m, nil
}

// subtaskCounts returns parent id -> [done, total] over all tasks.
func subtaskCounts(all []*core.Task, isDone func(p, st string) bool) map[string][2]int {
	counts := map[string][2]int{}
	for _, t := range all {
		if t.Parent == "" {
			continue
		}
		c := counts[t.Parent]
		c[1]++
		if isDone(t.Project, t.Status) {
			c[0]++
		}
		counts[t.Parent] = c
	}
	return counts
}

func priorityPlain(p string) string {
	switch strings.ToLower(p) {
	case "urgent":
		return "URG "
	case "high":
		return "HIGH"
	case "medium":
		return "MED "
	case "low":
		return "LOW "
	}
	return "???? "
}

// listColumns is the single column layout for the list view. It is computed
// once per render (listColumnLayout) and shared by the header and every row:
// STS, PRIO, TITLE (flexible), COURSE, DUE. COURSE and DUE sit at fixed cell
// offsets so they line up exactly; subtask indentation lives INSIDE the
// TITLE cell and never shifts STS or PRIO.
type listColumns struct {
	statusW, prioW, courseW int // courseW == 0 when the course column is dropped (<78 cells)
	prefixW                 int // cells before the title cell (stripe/mark/status/prio)
	titleX, titleW          int
	courseX                 int // only meaningful when courseW > 0
	dueX, dueW              int
}

// listColumnLayout computes the column layout for a content width w.
//
//	x=0 stripe · x=2 mark · x=4 STS · x=sts+5 PRIO · x=prefixW TITLE
//	TITLE flexes · 2-cell gap · COURSE (optional) · 2-cell gap · DUE · 1 trailing cell.
func listColumnLayout(w int) listColumns {
	const statusW, prioW, dueW = 6, 4, 12
	courseW := 0
	if w >= 78 {
		courseW = 10
	}
	prefixW := 4 + statusW + 1 + prioW + 1
	dueX := w - 1 - dueW
	courseX := -1
	titleRight := dueX - 2
	if courseW > 0 {
		courseX = dueX - 2 - courseW
		titleRight = courseX - 2
	}
	titleW := titleRight - prefixW
	if titleW < 4 {
		titleW = 4
	}
	return listColumns{
		statusW: statusW, prioW: prioW, courseW: courseW, prefixW: prefixW,
		titleX: prefixW, titleW: titleW, courseX: courseX, dueX: dueX, dueW: dueW,
	}
}

func duePlain(due string, now time.Time) string {
	if due == "" {
		return ""
	}
	if label, _ := gauges.DueLabel(due, now); label != "" {
		return label
	}
	return due
}

func truncatePlain(s string, budget int) string {
	if budget <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= budget {
		return s
	}
	runes := []rune(s)
	for len(runes) > 0 && runewidth.StringWidth(string(runes))+1 > budget {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

// statusCell renders the status column: dimmed, padded to width cells.
func statusCell(s styles, status string, done bool, w int) string {
	s1 := strings.ToLower(strings.TrimSpace(status))
	if done {
		return s.statusDone.Width(w).Render(truncatePlain(s1, w))
	}
	if strings.EqualFold(s1, "doing") || strings.EqualFold(s1, "in progress") {
		return s.statusDoing.Width(w).Render(truncatePlain(s1, w))
	}
	return s.dim.Width(w).Render(truncatePlain(s1, w))
}

// renderListBody renders the list content (inner width × inner height).
// A pinned header row sits above the scrollable body.
func renderListBody(m Model, w, h int) []string {
	if w < 1 {
		w = 1
	}
	s := m.styles
	now := m.clock.Now()
	isDone := func(p, st string) bool { return m.store.IsDoneStatus(p, st) }
	head := subtaskCounts(m.store.Tasks(), isDone)

	col := listColumnLayout(w)

	lines := make([]string, 0, h)
	lines = append(lines, listHeaderLine(m, w, col))

	bodyH := h - 1
	if bodyH < 1 {
		bodyH = 1
	}

	// Render the line stream, skipping lines above the scroll offset.
	rendered := 0
	line := 0
	for i := 0; i < len(m.rows); i++ {
		if m.groupHeads[i] != "" {
			if line >= m.scroll && rendered < bodyH {
				lines = append(lines, groupHeaderLine(m, w, m.groupHeads[i], groupCount(m, i)))
				rendered++
			}
			line++
		}
		if line >= m.scroll && rendered < bodyH {
			lines = append(lines, listRowLine(m, w, i, col, now, isDone, head))
			rendered++
		}
		line++
		if rendered >= bodyH {
			break
		}
	}

	if len(m.rows) == 0 {
		msg := "nothing here — nice."
		if m.searchQuery != "" {
			msg = fmt.Sprintf("nothing matches %q — esc clears the filter", m.searchQuery)
		} else if sel := m.sidebarSelection(); sel != nil && sel.Kind == "smart" {
			switch sel.Key {
			case "overdue":
				msg = "nothing overdue — nice."
			case "today":
				msg = "nothing due today."
			case "week":
				msg = "nothing due this week."
			}
		}
		lines = append(lines, s.muted.Render(" ✦ "+msg))
	}
	return lines
}

func groupCount(m Model, i int) int {
	n := 0
	for j := i; j < len(m.rows); j++ {
		if j != i && m.groupHeads[j] != "" {
			break
		}
		n++
	}
	return n
}

// groupHeaderLine renders a quiet caps group header with a hairline rule.
func groupHeaderLine(m Model, w int, label string, count int) string {
	s := m.styles
	text := letterspace(label) + fmt.Sprintf("  · %d", count)
	plain := " " + text + " "
	ruleLen := w - runewidth.StringWidth(plain)
	if ruleLen < 1 {
		ruleLen = 1
	}
	return s.groupHeader.Render(plain) + s.groupHeader.Render(strings.Repeat("─", ruleLen))
}

func listHeaderLine(m Model, w int, col listColumns) string {
	surface := m.palette.Surface
	caps := func(t string, width int) string {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(lipgloss.Color(surface)).Bold(true).Render(padRightPlain(t, width))
	}
	space := func(n int) string {
		return lipgloss.NewStyle().Background(lipgloss.Color(surface)).Render(strings.Repeat(" ", n))
	}

	var b strings.Builder
	b.WriteString(space(4))
	b.WriteString(caps("STS", col.statusW))
	b.WriteString(space(1))
	b.WriteString(caps("PRIO", col.prioW))
	b.WriteString(space(1))
	b.WriteString(caps("TITLE", col.titleW))
	b.WriteString(space(2))
	if col.courseW > 0 {
		b.WriteString(caps("COURSE", col.courseW))
		b.WriteString(space(2))
	}
	b.WriteString(caps("DUE", col.dueW))
	b.WriteString(space(1))
	return b.String()
}

func padRightPlain(s string, w int) string {
	n := w - runewidth.StringWidth(s)
	if n < 0 {
		n = 0
	}
	return s + strings.Repeat(" ", n)
}

func listRowLine(m Model, w, i int, col listColumns, now time.Time, isDone func(p, st string) bool, head map[string][2]int) string {
	s := m.styles
	row := m.rows[i]
	t := row.Task
	done := isDone(t.Project, t.Status)
	selected := i == m.cursor && m.focus != focusSidebar

	bgRaised := lipgloss.Color(m.palette.SurfaceRaised)
	bgSurface := lipgloss.Color(m.palette.Surface)
	zebra := theme.Blend(m.palette.Surface, m.palette.SurfaceRaised, 0.3)
	bg := bgSurface
	if i%2 == 1 {
		bg = lipgloss.Color(zebra)
	}
	if selected {
		bg = bgRaised
	}
	hl := func(c string) lipgloss.Style { return lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Background(bg) }
	// Every gap cell carries the row background: raw spaces would fall back to
	// the terminal default once a styled segment resets SGR (the reason the
	// selected row used to show through).
	sp := func(n int) string { return lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", n)) }
	txt := func() lipgloss.Style {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(bg)
	}
	mutedTxt := func() lipgloss.Style {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(bg)
	}

	// Course stripe — replaced with an accent left bar when selected.
	stripeColor := theme.CourseColor(m.palette, t.Project, projectColorOverride(m, t.Project))
	if selected {
		stripeColor = m.palette.Accent
	}
	stripe := hl(stripeColor).Render("▌")

	// Status glyph.
	mark := "○"
	markStyle := mutedTxt()
	if done {
		mark = "✔"
		markStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Success)).Background(bg)
	} else if strings.EqualFold(t.Status, "doing") || strings.EqualFold(t.Status, "in progress") {
		mark = "◐"
		markStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Blue)).Background(bg)
	}

	statusCol := statusCell(m.styles, t.Status, done, col.statusW)
	if selected {
		statusCol = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(bg).Width(col.statusW).Render(truncatePlain(strings.ToLower(t.Status), col.statusW))
	}
	prioCol := s.priority(t.Priority).Background(bg).Width(col.prioW).Render(priorityPlain(t.Priority))

	// Subtask progress suffix.
	sub := ""
	if head[t.ID][1] > 0 {
		frac := float64(head[t.ID][0]) / float64(head[t.ID][1])
		fill, track := gauges.Bar(frac, 4)
		sub = sp(1) + hl(stripeColor).Render(fill) + mutedTxt().Render(track) + mutedTxt().Render(fmt.Sprintf(" %d/%d", head[t.ID][0], head[t.ID][1]))
	}

	// Tags follow the title as dim chips; they live inside the TITLE cell.
	tagStr := ""
	if len(t.Tags) > 0 {
		tags := t.Tags
		if len(tags) > 2 {
			tags = tags[:2]
		}
		parts := make([]string, len(tags))
		for j, tag := range tags {
			parts[j] = "#" + tag
		}
		tagStr = strings.Join(parts, " ")
	}

	// Title budget: indent and tree live inside the title cell, so STS and
	// PRIO stay pinned to their columns at every depth.
	indentW := row.Depth * 2
	treeW := 0
	if row.Depth > 0 {
		treeW = 2
	}
	tagsW := 0
	if tagStr != "" {
		tagsW = runewidth.StringWidth(tagStr) + 1
	}
	subW := lipgloss.Width(sub)
	titleBudget := col.titleW - indentW - treeW - tagsW - subW
	if titleBudget < 4 {
		titleBudget = 4
	}

	courseCol := ""
	if col.courseW > 0 {
		name := t.Project
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}
		courseCol = hl(stripeColor).Render(padRightPlain(truncatePlain(name, col.courseW), col.courseW))
	}

	// Title (+ subtask bar + tags) padded to exactly the title cell width.
	title := truncatePlain(t.Title, titleBudget)
	var titleRendered string
	if done {
		titleRendered = strikeTitle(lipgloss.Color(m.palette.Muted), bg, title)
	} else {
		titleRendered = txt().Render(title)
	}
	titleCell := sp(indentW)
	if row.Depth > 0 {
		titleCell += mutedTxt().Render("↳ ")
	}
	titleCell += titleRendered + sub
	if tagStr != "" {
		titleCell += sp(1) + mutedTxt().Render(tagStr)
	}
	if pad := col.titleW - lipgloss.Width(titleCell); pad > 0 {
		titleCell += lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", pad))
	} else if lipgloss.Width(titleCell) > col.titleW {
		titleCell = clipStyled(titleCell, col.titleW)
	}

	// Fixed cells to the right edge.
	dueStr := ""
	if done && (t.Completed != "" || t.Updated != "" || t.Due != "") {
		// Done tasks never show overdue/urgency — a muted completion marker.
		dc := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.MutedSoft)).Background(bg)
		dueStr = dc.Render(doneLabelFor(t, now))
	} else if t.Due != "" {
		label, heat := gauges.DueLabel(t.Due, now)
		dc := lipgloss.NewStyle().Background(bg).Bold(heat != gauges.HeatCalm)
		switch heat {
		case gauges.HeatWarm:
			dc = dc.Foreground(lipgloss.Color(m.palette.HeatWarm))
		case gauges.HeatHot:
			dc = dc.Foreground(lipgloss.Color(m.palette.HeatHot))
		case gauges.HeatOverdue:
			dc = dc.Foreground(lipgloss.Color(m.palette.HeatOverdue))
		default:
			dc = dc.Foreground(lipgloss.Color(m.palette.MutedSoft))
		}
		dueStr = dc.Render(label)
	}
	dueCell := dueStr
	if pad := col.dueW - lipgloss.Width(dueCell); pad > 0 {
		dueCell += lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", pad))
	} else if lipgloss.Width(dueCell) > col.dueW {
		dueCell = clipStyled(dueCell, col.dueW)
	}

	line := stripe + sp(1) + markStyle.Render(mark) + sp(1) + statusCol + sp(1) + prioCol + sp(1) + titleCell
	if col.courseW > 0 {
		line += sp(2) + courseCol
	}
	line += sp(2) + dueCell + sp(1)
	line = padToWidth(line, w, bg)
	return line
}

func padToWidth(line string, w int, bg lipgloss.Color) string {
	if lipgloss.Width(line) > w {
		return clipStyled(line, w)
	}
	return line + lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", w-lipgloss.Width(line)))
}

// dueBucket buckets a due string for group headers (sort: due first).
func dueBucket(due string, now time.Time) string {
	if strings.TrimSpace(due) == "" {
		return "NO DATE"
	}
	d, err := core.ParseDue(due, now.Location())
	if err != nil {
		return "NO DATE"
	}
	y, m, dd := now.Date()
	today := time.Date(y, m, dd, 0, 0, 0, 0, now.Location())
	dy := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, now.Location())
	days := int(dy.Sub(today).Hours() / 24)
	switch {
	case days < 0:
		return "OVERDUE"
	case days == 0:
		return "TODAY"
	case days <= 7:
		return "THIS WEEK"
	default:
		return "LATER"
	}
}

// computeGroups derives group headers + line mappings after rows are built.
// Grouping activates only for due-first sorts; otherwise line == row.
func (m *Model) computeGroups() {
	m.rowLines = make([]int, len(m.rows))
	m.groupHeads = make([]string, len(m.rows))
	sortedByDue := len(m.cfg.Sort) > 0 && strings.EqualFold(m.cfg.Sort[0], "due")
	line := 0
	last := ""
	now := m.clock.Now()
	for i, r := range m.rows {
		key := ""
		if sortedByDue {
			key = dueBucket(r.Task.Due, now)
			if key != last {
				m.groupHeads[i] = key
				line++ // header line precedes the row
				last = key
			}
		}
		m.rowLines[i] = line
		line++
	}
}

// rowsLine returns the screen line of row i (row index when ungrouped).
func (m Model) rowsLine(i int) int {
	if i >= 0 && i < len(m.rowLines) {
		return m.rowLines[i]
	}
	return i
}

// totalListLines returns the rendered height of the list body.
func (m Model) totalListLines() int {
	if len(m.rowLines) == 0 {
		return 0
	}
	return m.rowLines[len(m.rowLines)-1] + 1
}
