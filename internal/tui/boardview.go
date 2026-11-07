package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/gauges"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

// boardProject is the project whose statuses define the columns: the
// sidebar-selected project, else the default project.
func (m Model) boardProject() string {
	if sel := m.sidebarSelection(); sel != nil && sel.Kind == "project" {
		return sel.Key
	}
	return m.cfg.DefaultProject
}

func (m Model) boardStatuses() []core.ProjectStatus {
	p, err := m.store.GetProject(m.boardProject())
	if err == nil && len(p.Statuses) > 0 {
		return p.Statuses
	}
	return core.DefaultProject(m.boardProject()).Statuses
}

// boardColumns groups tasks of the board project by status, sorted per config.
func boardColumns(m Model) [][]*core.Task {
	statuses := m.boardStatuses()
	cols := make([][]*core.Task, len(statuses))
	isDone := func(p, st string) bool { return m.store.IsDoneStatus(p, st) }

	needle := strings.ToLower(strings.TrimSpace(m.needle))
	for _, t := range m.store.Tasks() {
		if !strings.EqualFold(t.Project, m.boardProject()) {
			continue
		}
		if needle != "" {
			hay := strings.ToLower(strings.Join([]string{t.Title, t.Body, strings.Join(t.Tags, " ")}, "\n"))
			if !strings.Contains(hay, needle) {
				continue
			}
		}
		placed := false
		for i, st := range statuses {
			if strings.EqualFold(t.Status, st.Name) {
				cols[i] = append(cols[i], t)
				placed = true
				break
			}
		}
		if !placed && len(cols) > 0 {
			cols[0] = append(cols[0], t) // unknown status: first column
		}
	}
	for i := range cols {
		core.SortTasks(cols[i], m.cfg.Sort, isDone)
	}
	return cols
}

// moveBoardSelection moves the selection one column left/right (h/l).
func (m *Model) moveBoardSelection(dir int) {
	cols := boardColumns(*m)
	if len(cols) == 0 {
		return
	}
	m.boardCol = clampInt(m.boardCol+dir, 0, len(cols)-1)
	if m.boardRow >= len(cols[m.boardCol]) {
		m.boardRow = len(cols[m.boardCol]) - 1
	}
	if m.boardRow < 0 {
		m.boardRow = 0
	}
	m.syncCursorToBoardCard()
}

func (m *Model) resetBoardCursor() {
	m.boardCol, m.boardRow = 0, 0
	m.boardDragging = false
}

func (m Model) selectedBoardTask() *core.Task {
	cols := boardColumns(m)
	if m.boardCol < 0 || m.boardCol >= len(cols) {
		return nil
	}
	col := cols[m.boardCol]
	if m.boardRow < 0 || m.boardRow >= len(col) {
		return nil
	}
	return col[m.boardRow]
}

func (m Model) updateBoardKey(act string) (tea.Model, tea.Cmd) {
	cols := boardColumns(m)
	switch act {
	case aUp:
		m.boardRow -= 1
	case aDown:
		m.boardRow += 1
	case aLeft:
		m.moveBoardSelection(-1)
	case aRight:
		m.moveBoardSelection(+1)
	case aJumpLeft:
		return m.boardMoveCard(-1)
	case aJumpRight:
		return m.boardMoveCard(+1)
	case aTop:
		m.boardRow = 0
	case aBottom:
		if m.boardCol < len(cols) && len(cols[m.boardCol]) > 0 {
			m.boardRow = len(cols[m.boardCol]) - 1
		}
	default:
		if m2, cmd, handled := m.updateSimpleTaskAction(act); handled {
			// Board selection mirrors the flat cursor where possible: point the
			// list cursor at the board card so detail/notes act on the same task.
			m.syncCursorToBoardCard()
			return m2, cmd
		}
		return m, nil
	}
	// Clamp.
	if m.boardCol >= len(cols) {
		m.boardCol = len(cols) - 1
	}
	if m.boardCol < 0 {
		m.boardCol = 0
	}
	if m.boardCol < len(cols) && len(cols[m.boardCol]) > 0 {
		if m.boardRow >= len(cols[m.boardCol]) {
			m.boardRow = len(cols[m.boardCol]) - 1
		}
	} else {
		m.boardRow = 0
	}
	if m.boardRow < 0 {
		m.boardRow = 0
	}
	return m, nil
}

// syncCursorToBoardCard moves the flat-list cursor to the board-selected task.
func (m *Model) syncCursorToBoardCard() {
	t := m.selectedBoardTask()
	if t == nil {
		return
	}
	for i := range m.rows {
		if m.rows[i].Task.ID == t.ID {
			m.cursor = i
			m.ensureCursorVisible()
			return
		}
	}
}

// boardMoveCard moves the selected card one column left/right (H/L).
func (m Model) boardMoveCard(dir int) (tea.Model, tea.Cmd) {
	cols := boardColumns(m)
	if len(cols) == 0 || m.boardCol >= len(cols) {
		return m, nil
	}
	target := clampInt(m.boardCol+dir, 0, len(cols)-1)
	if target == m.boardCol {
		return m, nil
	}
	card := m.selectedBoardTask()
	if card == nil {
		return m, nil
	}
	statuses := m.boardStatuses()
	newStatus := statuses[target].Name
	msg, err := actions.SetStatus(m.store, card, newStatus)
	if err != nil {
		m.statusMsg = err.Error()
		return m, nil
	}
	m.statusMsg = fmt.Sprintf("%s → %s%s", card.ID, newStatus, spawnSuffix(msg))
	m.boardCol = target
	cols = boardColumns(m)
	m.boardRow = len(cols[target]) - 1
	m.syncCursorToBoardCard()
	return m, m.afterChange("status: " + card.Title)
}

func padRight(s string, w int) string {
	d := w - runewidth.StringWidth(s)
	if d < 0 {
		return truncatePlain(s, w)
	}
	return s + strings.Repeat(" ", d)
}

const boardCardH = 5

// boardLayout computes the visible column window and column width so mouse
// hit-testing matches the render exactly.
func boardLayout(m Model, w int) (winStart, visCols, colW int) {
	n := len(m.boardStatuses())
	if n == 0 {
		return 0, 0, 0
	}
	colW = 22
	visCols = (w - boardInset + boardGap) / (colW + boardGap)
	if visCols < 1 {
		visCols = 1
	}
	if n < visCols {
		visCols = n
	}
	colW = (w - boardInset - (visCols-1)*boardGap) / visCols
	colW = clampInt(colW, 12, 48)
	winStart = 0
	if m.boardCol >= winStart+visCols {
		winStart = m.boardCol - visCols + 1
	}
	if winStart > n-visCols {
		winStart = n - visCols
	}
	if winStart < 0 {
		winStart = 0
	}
	return winStart, visCols, colW
}

// boardInset is the 1-cell base-colored inset at the left and right edge of
// the board body; boardGap is the 1-cell base-colored gap between columns.
const (
	boardInset = 1
	boardGap   = 1
)

// baseInsetRow renders one board row: a 1-cell base-colored inset (which
// carries the scroll chevron when set), cells joined by the base gutter,
// then base-colored padding to w; rightHint pins at the right edge.
func baseInsetRow(baseBG lipgloss.Style, gutter, inset string, cells []string, w int, rightHint string) string {
	line := inset + strings.Join(cells, gutter)
	if rightHint != "" {
		if pad := w - lipgloss.Width(line) - lipgloss.Width(rightHint); pad > 0 {
			line += baseBG.Render(strings.Repeat(" ", pad))
		} else if lipgloss.Width(line) > w-lipgloss.Width(rightHint) {
			line = clipStyled(line, w-lipgloss.Width(rightHint))
		}
		return line + rightHint
	}
	if pad := w - lipgloss.Width(line); pad > 0 {
		line += baseBG.Render(strings.Repeat(" ", pad))
	}
	return line
}

// renderBoardBody renders the board content (inner width × inner height).
func renderBoardBody(m Model, w, h int) []string {
	s := m.styles
	now := m.clock.Now()
	statuses := m.boardStatuses()
	cols := boardColumns(m)

	n := len(statuses)
	if n == 0 {
		return []string{s.dim.Render("no statuses")}
	}

	winStart, visCols, colW := boardLayout(m, w)
	view := winStart + visCols

	baseBG := lipgloss.NewStyle().Background(lipgloss.Color(m.palette.Base))
	gutter := baseBG.Render(strings.Repeat(" ", boardGap))
	mutedBGBase := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(lipgloss.Color(m.palette.Base))

	lines := make([]string, 0, h)

	// Header line: status · count over a thin underline; the 1-cell base
	// inset carries the ‹ › chevrons when scrolled.
	headerCells := make([]string, 0, visCols)
	underlineCells := make([]string, 0, visCols)
	for i := winStart; i < view; i++ {
		st := statuses[i]
		count := 0
		if i < len(cols) {
			count = len(cols[i])
		}
		name := strings.ToUpper(st.Name)
		label := fmt.Sprintf("%s · %d", name, count)
		hcell := s.dim
		switch {
		case m.boardDragging && i != m.boardCol:
			hcell = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(lipgloss.Color(m.palette.SurfaceStrong)).Bold(true)
		case m.boardDragging && i == m.boardDragHover:
			hcell = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.BrightText)).Background(lipgloss.Color(m.palette.SurfaceStrong)).Bold(true)
		case i == m.boardCol && m.focus != focusSidebar:
			hcell = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(lipgloss.Color(m.palette.Surface)).Bold(true)
		}
		headerCells = append(headerCells, hcell.Width(colW).Render(" "+truncatePlain(label, colW-1)))
		underlineCells = append(underlineCells, lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Border)).Background(lipgloss.Color(m.palette.Surface)).Render(strings.Repeat("─", colW)))
	}
	dirg := baseBG.Render(" ")
	if winStart > 0 {
		dirg = mutedBGBase.Render(m.icon("chev_left"))
	}
	rightHint := ""
	if view < n {
		rightHint = mutedBGBase.Render(m.icon("chev_right"))
	}
	lines = append(lines, baseInsetRow(baseBG, gutter, dirg, headerCells, w, rightHint))
	lines = append(lines, baseInsetRow(baseBG, gutter, baseBG.Render(" "), underlineCells, w, ""))

	// Cards.
	bodyH := h - 2
	if bodyH < boardCardH {
		bodyH = boardCardH
	}
	maxCards := bodyH / boardCardH
	if maxCards < 1 {
		maxCards = 1
	}
	isDone := func(p, st string) bool { return m.store.IsDoneStatus(p, st) }
	head := subtaskCounts(m.store.Tasks(), isDone)
	for r := 0; r < maxCards; r++ {
		cardLines := make([][]string, 0, visCols)
		for i := winStart; i < view; i++ {
			if r < len(cols[i]) {
				sel := i == m.boardCol && r == m.boardRow && m.focus != focusSidebar
				cardLines = append(cardLines, renderCard(m, cols[i][r], colW, sel, now, head))
			} else if r == 0 {
				cardLines = append(cardLines, cardPlaceholder(m, colW))
			} else {
				cardLines = append(cardLines, blankCard(m, colW))
			}
		}
		for lineIdx := 0; lineIdx < boardCardH; lineIdx++ {
			cells := make([]string, 0, len(cardLines))
			for _, cl := range cardLines {
				cells = append(cells, cl[lineIdx])
			}
			lines = append(lines, baseInsetRow(baseBG, gutter, baseBG.Render(" "), cells, w, ""))
		}
	}
	// Overflow indicator.
	for i := winStart; i < view; i++ {
		if i < len(cols) && len(cols[i]) > maxCards {
			more := fmt.Sprintf("+%d more in %s", len(cols[i])-maxCards, strings.ToLower(statuses[i].Name))
			lines = append(lines, padToWidth(baseBG.Render(" ")+s.muted.Render(truncatePlain(more, w-2)), w, lipgloss.Color(m.palette.Surface)))
			break
		}
	}
	return lines
}

func blankCard(m Model, w int) []string {
	fill := lipgloss.NewStyle().Background(lipgloss.Color(m.palette.Surface))
	out := make([]string, boardCardH)
	for i := range out {
		out[i] = fill.Render(strings.Repeat(" ", w))
	}
	return out
}

func cardPlaceholder(m Model, w int) []string {
	surf := lipgloss.Color(m.palette.Surface)
	fill := lipgloss.NewStyle().Background(surf)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(surf)
	out := make([]string, boardCardH)
	for i := range out {
		out[i] = fill.Render(strings.Repeat(" ", w))
	}
	txt := "No tasks"
	tw := lipgloss.Width(txt)
	left := (w - tw) / 2
	if left < 0 {
		left = 0
	}
	out[boardCardH/2] = fill.Render(strings.Repeat(" ", left)) + muted.Render(txt) + fill.Render(strings.Repeat(" ", w-left-tw))
	return out
}

func renderCard(m Model, t *core.Task, w int, selected bool, now time.Time, head map[string][2]int) []string {
	isDone := func(p, st string) bool { return m.store.IsDoneStatus(p, st) }
	done := isDone(t.Project, t.Status)
	projectColor := theme.CourseColor(m.palette, t.Project, projectColorOverride(m, t.Project))
	hl := func(bg lipgloss.Color) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(projectColor)).Background(bg)
	}
	ray := lipgloss.Color(m.palette.SurfaceRaised)

	cardBg := ray
	var border lipgloss.Style
	if selected {
		cardBg = lipgloss.Color(m.palette.SurfaceRaised)
		border = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Accent)).Background(cardBg)
	} else {
		border = lipgloss.NewStyle().Background(cardBg)
	}
	fill := func(st lipgloss.Style) lipgloss.Style { return st.Background(cardBg) }

	stripe := hl(cardBg).Render("▌")

	// Title wrapped to max 2 lines.
	titleW := w - 3
	cut1, cut2 := wrapTitle(t.Title, titleW)
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(cardBg)
	if done {
		titleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Strikethrough(true).Background(cardBg)
	}
	line1 := stripe + " " + titleStyle.Render(padRightPlain(cut1, titleW))
	line2 := stripe + " " + titleStyle.Render(padRightPlain(cut2, titleW))

	// Meta: priority badge chip, due chip, tags, subtask progress.
	chip := priorityChip(m, t.Priority, cardBg)
	popping := m.animTaskID == t.ID && m.animFrame > 0
	if popping {
		if m.animFrame%2 == 0 {
			chip = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Base)).Background(lipgloss.Color(m.palette.Accent)).Bold(true).Render(" ✦ ")
		} else {
			chip = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Base)).Background(lipgloss.Color(m.palette.Success)).Bold(true).Render(" ✔ ")
		}
	}
	meta := stripe + " " + chip + " "

	if t.Due != "" {
		label, heat := gauges.DueLabel(t.Due, now)
		dc := lipgloss.NewStyle().Background(cardBg)
		switch heat {
		case gauges.HeatWarm:
			dc = dc.Foreground(lipgloss.Color(m.palette.HeatWarm))
		case gauges.HeatHot:
			dc = dc.Foreground(lipgloss.Color(m.palette.HeatHot)).Bold(true)
		case gauges.HeatOverdue:
			dc = dc.Foreground(lipgloss.Color(m.palette.HeatOverdue)).Bold(true)
		default:
			dc = dc.Foreground(lipgloss.Color(m.palette.MutedSoft))
		}
		if done {
			dc = lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(cardBg)
		}
		meta += dc.Render(label) + " "
	}

	if len(t.Tags) > 0 {
		tags := t.Tags
		if len(tags) > 1 {
			tags = tags[:1]
		}
		meta += fill(lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted))).Render("#" + tags[0])
	}

	if head[t.ID][1] > 0 {
		frac := float64(head[t.ID][0]) / float64(head[t.ID][1])
		fillBar, _ := gauges.Bar(frac, 4)
		meta += hl(cardBg).Render(fillBar) + fill(lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted))).Render(fmt.Sprintf(" %d/%d", head[t.ID][0], head[t.ID][1]))
	}

	lines := []string{line1, line2, padRightCard(meta, w, cardBg)}

	if selected {
		top := border.Render("╭" + strings.Repeat("─", w-2) + "╮")
		bot := border.Render("╰" + strings.Repeat("─", w-2) + "╯")
		framed := []string{top}
		for _, l := range lines {
			framed = append(framed, border.Render("│")+padRightCard(l, w-2, cardBg)+border.Render("│"))
		}
		framed = append(framed, bot)
		return strings.Split(paint(strings.Join(framed, "\n"), w, boardCardH, cardBg), "\n")
	}
	blank := strings.Repeat(" ", w)
	out := []string{fill(lipgloss.NewStyle()).Render(blank)}
	for _, l := range lines {
		out = append(out, padRightCard(l, w, cardBg))
	}
	out = append(out, lipgloss.NewStyle().Background(cardBg).Render(blank))
	return strings.Split(paint(strings.Join(out, "\n"), w, boardCardH, cardBg), "\n")
}

// priorityChip renders a priority as an intentional badge chip (accent-filled
// bold label) rather than muted words; low uses a quiet strong-surface chip.
func priorityChip(m Model, p string, bg lipgloss.Color) string {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "urgent":
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Base)).Background(lipgloss.Color(m.palette.Danger)).Bold(true).Render(" URG ")
	case "high":
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Base)).Background(lipgloss.Color(m.palette.HeatHot)).Bold(true).Render(" HIGH ")
	case "medium":
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Base)).Background(lipgloss.Color(m.palette.Warning)).Bold(true).Render(" MED ")
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(lipgloss.Color(m.palette.SurfaceStrong)).Render(" LOW ")
	}
}

func padRightCard(line string, w int, bg lipgloss.Color) string {
	if lipgloss.Width(line) > w {
		line = clipStyled(line, w)
	}
	if pad := w - lipgloss.Width(line); pad > 0 {
		line += lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", pad))
	}
	return line
}

// wrapTitle word-wraps a title into at most two lines of width cells each.
// Overlong second lines are ellipsis-truncated.
func wrapTitle(s string, width int) (string, string) {
	if width <= 0 {
		return "", ""
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return "", ""
	}
	var l1, l2 string
	i := 0
	for _, word := range words {
		candidate := word
		if l1 != "" {
			candidate = l1 + " " + word
		}
		if runewidth.StringWidth(candidate) <= width {
			l1 = candidate
			i++
			continue
		}
		break
	}
	remainder := strings.Join(words[i:], " ")
	l2 = truncatePlain(remainder, width)
	return truncatePlain(l1, width), l2
}

// clickBoard handles mouse press/release on the board. Columns are laid out
// left→right; a press selects the card under the cursor, a release moves it
// into the column under the cursor (drag-to-move). Motion during a drag
// tracks the hovered column for the drop-target highlight.
func (m Model) clickBoard(x, y int, release bool) (tea.Model, tea.Cmd) {
	cols := boardColumns(m)
	statuses := m.boardStatuses()
	n := len(cols)
	if n == 0 {
		return m, nil
	}
	// x is main-pane-relative (sidebar + gutter already removed by the caller).
	sidebarW := sideWidth(m.width, m.sidebarHidden)
	detailW := detailWidth(m.width, m.detailOn)
	gutters := 0
	if sidebarW > 0 {
		gutters++
	}
	if detailW > 0 {
		gutters++
	}
	mainW := m.width - sidebarW - detailW - gutters
	innerW := mainW - 2
	winStart, visCols, colW := boardLayout(m, innerW)
	if visCols < 1 {
		return m, nil
	}
	local := x - 2 // panel pad (1) + body inset (1)
	view := winStart + visCols
	col := -1
	for i := winStart; i < view; i++ {
		off := boardInset + (i-winStart)*(colW+boardGap)
		if local >= off && local < off+colW {
			col = i
			break
		}
	}
	if col < 0 {
		if release {
			m.boardDragging = false
			m.boardDragHover = -1
		}
		return m, nil
	}
	row := (y - 3) / boardCardH // pad(1) + header(2)

	if release {
		if !m.boardDragging {
			return m, nil
		}
		m.boardDragging = false
		m.boardDragHover = -1
		card := m.selectedBoardTask()
		if card == nil {
			return m, nil
		}
		if col == m.boardCol {
			return m, nil
		}
		newStatus := statuses[col].Name
		msg, err := actions.SetStatus(m.store, card, newStatus)
		if err != nil {
			m.statusMsg = err.Error()
			return m, nil
		}
		m.statusMsg = fmt.Sprintf("%s → %s%s", card.ID, newStatus, spawnSuffix(msg))
		m.boardCol = col
		cols = boardColumns(m)
		if col < len(cols) {
			m.boardRow = len(cols[col]) - 1
		}
		m.syncCursorToBoardCard()
		return m, m.afterChange("status: " + card.Title)
	}

	if m.boardDragging {
		m.boardDragHover = col
		return m, nil
	}

	// Press: select card and begin drag.
	if row >= 0 && col < n && row < len(cols[col]) {
		m.boardCol, m.boardRow = col, row
		m.boardDragging = true
		m.boardDragHover = col
		m.syncCursorToBoardCard()
	}
	return m, nil
}
