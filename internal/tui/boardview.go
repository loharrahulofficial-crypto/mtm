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

// boardColumn is the per-column view of the board: the tasks shown
// (after the done-recency filter), the column total before that filter,
// and how many were hidden by it (done columns only).
type boardColumn struct {
	name   string
	shown  []*core.Task
	total  int
	hidden int
	isDone bool
}

// doneStampTime is the completion timestamp used for done-column ordering
// and the recency filter: Completed → Updated → Created → Due.
func doneStampTime(t *core.Task) time.Time {
	for _, s := range []string{t.Completed, t.Updated, t.Created} {
		if ts, err := time.ParseInLocation(core.TimestampFormat, s, time.Local); err == nil {
			return ts
		}
	}
	if d, err := core.ParseDue(t.Due, time.Local); err == nil {
		return d
	}
	return time.Time{}
}

// boardColumnsModel groups tasks of the board project by status. Done
// columns obey board.done_days (default 14; 0 hides the filter): items
// completed before the cutoff are hidden unless the user toggled D.
// Done columns sort newest first; the rest follow cfg.Sort.
func boardColumnsModel(m Model) []boardColumn {
	statuses := m.boardStatuses()
	cols := make([]boardColumn, len(statuses))
	for i, st := range statuses {
		cols[i].name = st.Name
		cols[i].isDone = m.store.IsDoneStatus(m.boardProject(), st.Name)
	}
	isDone := func(p, st string) bool { return m.store.IsDoneStatus(p, st) }

	now := m.clock.Now()
	cutoff := time.Time{}
	if days := m.cfg.Board.DoneDays; days > 0 && !m.boardDoneAll {
		y, mo, d := now.Date()
		cutoff = time.Date(y, mo, d, 0, 0, 0, 0, now.Location()).AddDate(0, 0, -days)
	}

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
				cols[i].total++
				if cols[i].isDone && !cutoff.IsZero() && doneStampTime(t).Before(cutoff) {
					cols[i].hidden++
					placed = true
					break
				}
				cols[i].shown = append(cols[i].shown, t)
				placed = true
				break
			}
		}
		if !placed && len(cols) > 0 {
			cols[0].shown = append(cols[0].shown, t) // unknown status: first column
			cols[0].total++
		}
	}
	for i := range cols {
		if cols[i].isDone {
			shown := cols[i].shown
			for j := 1; j < len(shown); j++ {
				for k := j; k > 0 && doneStampTime(shown[k]).After(doneStampTime(shown[k-1])); k-- {
					shown[k], shown[k-1] = shown[k-1], shown[k]
				}
			}
			continue
		}
		core.SortTasks(cols[i].shown, m.cfg.Sort, isDone)
	}
	return cols
}

// boardColumns groups tasks of the board project by status, sorted per config.
func boardColumns(m Model) [][]*core.Task {
	cols := boardColumnsModel(m)
	out := make([][]*core.Task, len(cols))
	for i := range cols {
		out[i] = cols[i].shown
	}
	return out
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
	case aDue:
		// D toggles the Done column's recency filter (the in-column footer
		// advertises it). Setting the due date stays available everywhere
		// else (list view, palette).
		m.boardDoneAll = !m.boardDoneAll
		if m.boardDoneAll {
			m.statusMsg = "board: showing all completed tasks"
		} else {
			m.statusMsg = fmt.Sprintf("board: done limited to the last %d days", m.cfg.Board.DoneDays)
		}
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

// contentHForClick mirrors renderBoardBody's usable card area height:
// inner panel height minus the two header rows.
func contentHForClick(m Model) int {
	bodyH := m.height - 4 // panel pad+border (2) + header rows (2)
	if bodyH < boardCardH {
		bodyH = boardCardH
	}
	return bodyH
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
	colsModel := boardColumnsModel(m)
	cols := make([][]*core.Task, len(colsModel))
	for i := range colsModel {
		cols[i] = colsModel[i].shown
	}

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
		label := fmt.Sprintf("%s · %d of %d", strings.ToUpper(st.Name), len(cols[i]), colsModel[i].total)
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

	// Cards. A card is 5 lines; its tags+sub-bar pack wraps to a 6th
	// line when it doesn't fit (never clipped, never overlapping). Slot
	// heights adapt per row across all visible columns, so card bottoms
	// stay aligned. If the panel is too short for another slot, the
	// overflow is summarized in ONE combined footer cell inside the
	// column ("+N more · +N older · D shows all").
	maxFitPreview := (h - 2) / boardCardH
	reserve := 0
	for i := winStart; i < view; i++ {
		if len(cols[i]) > maxFitPreview {
			reserve = 1
			break
		}
	}
	for i := winStart; i < view; i++ {
		if colsModel[i].hidden > 0 {
			reserve = 1
			break
		}
	}
	bodyH := h - 2 - reserve
	if bodyH < boardCardH {
		bodyH = boardCardH
	}
	isDone := func(p, st string) bool { return m.store.IsDoneStatus(p, st) }
	head := subtaskCounts(m.store.Tasks(), isDone)
	y := 0
	slotsRendered := 0
	for r := 0; ; r++ {
		cardLines := make([][]string, 0, visCols)
		for i := winStart; i < view; i++ {
			if r < len(cols[i]) {
				sel := i == m.boardCol && r == m.boardRow && m.focus != focusSidebar
				cardLines = append(cardLines, renderCard(m, cols[i][r], colW, sel, now, head))
			} else if r == 0 {
				cardLines = append(cardLines, cardPlaceholder(m, colW, boardCardH))
			} else {
				cardLines = append(cardLines, blankCard(m, colW, boardCardH))
			}
		}
		slotH := boardCardH
		for _, cl := range cardLines {
			if len(cl) > slotH {
				slotH = len(cl)
			}
		}
		if y+slotH > bodyH {
			break
		}
		for lineIdx := 0; lineIdx < slotH; lineIdx++ {
			cells := make([]string, 0, len(cardLines))
			for _, cl := range cardLines {
				if lineIdx < len(cl) {
					cells = append(cells, cl[lineIdx])
				} else {
					// Shorter card in a wrapped slot: pad with the card background.
					cells = append(cells, lipgloss.NewStyle().Background(lipgloss.Color(m.palette.SurfaceRaised)).Render(strings.Repeat(" ", colW)))
				}
			}
			lines = append(lines, baseInsetRow(baseBG, gutter, baseBG.Render(" "), cells, w, ""))
		}
		y += slotH
		slotsRendered++
		// Stop when every visible column is fully shown.
		maxSlots := 0
		for i := winStart; i < view; i++ {
			if len(cols[i]) > maxSlots {
				maxSlots = len(cols[i])
			}
		}
		if r+1 >= maxSlots {
			break
		}
	}
	// Footer: ONE left-aligned footer cell inside each column that has
	// something to say — "+N more · +N older · D shows all", shortened to
	// fit the column width, on the column's own background.
	footerNeeded := false
	for i := winStart; i < view; i++ {
		if len(cols[i]) > slotsRendered || colsModel[i].hidden > 0 {
			footerNeeded = true
			break
		}
	}
	if footerNeeded {
		cells := make([]string, 0, visCols)
		for i := winStart; i < view; i++ {
			more := len(cols[i]) - slotsRendered
			if more < 0 {
				more = 0
			}
			older := colsModel[i].hidden
			if more == 0 && older == 0 {
				cells = append(cells, blankFooterCell(m, colW))
				continue
			}
			cells = append(cells, boardFooterCell(m, colW, boardFooterText(more, older, colW)))
		}
		lines = append(lines, baseInsetRow(baseBG, gutter, baseBG.Render(" "), cells, w, ""))
	}
	// The columns extend to the panel's bottom edge: blank surface
	// column cells pad any remaining rows, equal heights, no strip.
	blankCell := lipgloss.NewStyle().Background(lipgloss.Color(m.palette.Surface)).Render(strings.Repeat(" ", colW))
	for len(lines) < h {
		cells := make([]string, 0, visCols)
		for i := winStart; i < view; i++ {
			cells = append(cells, blankCell)
		}
		lines = append(lines, baseInsetRow(baseBG, gutter, baseBG.Render(" "), cells, w, ""))
	}
	return lines
}

// blankFooterCell renders a surface-filled footer cell.
func blankFooterCell(m Model, w int) string {
	return lipgloss.NewStyle().Background(lipgloss.Color(m.palette.Surface)).Render(strings.Repeat(" ", w))
}

// boardFooterCell renders the one combined column footer left-aligned on
// the column's own background.
func boardFooterCell(m Model, w int, text string) string {
	surf := lipgloss.Color(m.palette.Surface)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(surf)
	t := truncatePlain(text, w-1) // leading space
	fill := w - 1 - lipgloss.Width(t)
	if fill < 0 {
		fill = 0
	}
	return lipgloss.NewStyle().Background(surf).Render(" ") + muted.Render(t) +
		lipgloss.NewStyle().Background(surf).Render(strings.Repeat(" ", fill))
}

// boardFooterText composes the overflow footer for one column:
// "+N more · +N older · D shows all", shortening the hint text and then
// the suffixes until it fits w cells (left-aligned, leading space).
func boardFooterText(more, older, w int) string {
	parts := []string{}
	if more > 0 {
		parts = append(parts, fmt.Sprintf("+%d more", more))
	}
	if older > 0 {
		parts = append(parts, fmt.Sprintf("+%d older", older))
	}
	base := strings.Join(parts, " · ")
	if older > 0 {
		for _, cand := range []string{
			base + " · D shows all",
			base + " · D",
			base,
		} {
			if lipgloss.Width(cand) <= w-1 {
				return cand
			}
		}
		for _, cand := range []string{
			fmt.Sprintf("+%d older · D", older),
			fmt.Sprintf("+%d older", older),
		} {
			if lipgloss.Width(cand) <= w-1 {
				return cand
			}
		}
		return truncatePlain(base, w-1)
	}
	for _, cand := range []string{
		base,
		fmt.Sprintf("+%d", more),
	} {
		if lipgloss.Width(cand) <= w-1 {
			return cand
		}
	}
	return truncatePlain(base, w-1)
}

func blankCard(m Model, w, n int) []string {
	fill := lipgloss.NewStyle().Background(lipgloss.Color(m.palette.Surface))
	out := make([]string, n)
	for i := range out {
		out[i] = fill.Render(strings.Repeat(" ", w))
	}
	return out
}

func cardPlaceholder(m Model, w, n int) []string {
	surf := lipgloss.Color(m.palette.Surface)
	fill := lipgloss.NewStyle().Background(surf)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Muted)).Background(surf)
	out := make([]string, n)
	for i := range out {
		out[i] = fill.Render(strings.Repeat(" ", w))
	}
	txt := "No tasks"
	tw := lipgloss.Width(txt)
	left := (w - tw) / 2
	if left < 0 {
		left = 0
	}
	out[n/2] = fill.Render(strings.Repeat(" ", left)) + muted.Render(txt) + fill.Render(strings.Repeat(" ", w-left-tw))
	return out
}

// chipWidths are the padded widths of the priority chips (" URG " …).
func chipLabelWidth(p string) int {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "urgent":
		return 5
	case "high":
		return 6
	default:
		return 5
	}
}

// cardDueLabel is the due slot text for a card (done-aware).
func cardDueLabel(m Model, t *core.Task, now time.Time, done bool) string {
	if done {
		if t.Completed != "" || t.Updated != "" || t.Due != "" {
			return doneLabelFor(t, now)
		}
		return ""
	}
	if t.Due == "" {
		return ""
	}
	label, _ := gauges.DueLabel(t.Due, now)
	return label
}

// cardMetaPack builds the tags+sub-bar pack unit shared by rendering and
// the mouse hit test: exactly 1 cell between "#tag" and the mini-bar,
// never more and never less. Returns the rendered pack, its cell width
// and the raw parts for the second line.
func cardMetaPack(m Model, t *core.Task, now time.Time, head map[string][2]int, cardBg lipgloss.Color) (string, int, string) {
	dimColor := lipgloss.Color(m.palette.Muted)
	fill := func(st lipgloss.Style) lipgloss.Style { return st.Background(cardBg) }
	sp := func(n int) string { return fill(lipgloss.NewStyle()).Render(strings.Repeat(" ", n)) }
	hl := func(c string) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Background(cardBg)
	}

	tagChip := ""
	if len(t.Tags) > 0 {
		tagChip = "#" + t.Tags[0]
	}
	subFill, subRest := "", ""
	if head[t.ID][1] > 0 {
		frac := float64(head[t.ID][0]) / float64(head[t.ID][1])
		fillBar, _ := gauges.Bar(frac, 4)
		subFill = fillBar
		subRest = fmt.Sprintf(" %d/%d", head[t.ID][0], head[t.ID][1])
	}
	pack, parts := "", ""
	packW := 0
	if tagChip != "" {
		pack += fill(lipgloss.NewStyle().Foreground(dimColor)).Render(tagChip)
		parts += fill(lipgloss.NewStyle().Foreground(dimColor)).Render(tagChip)
		packW += lipgloss.Width(tagChip)
		if subFill != "" {
			pack += sp(1)
			parts += sp(1)
			packW++
		}
	}
	if subFill != "" {
		pack += hl(theme.CourseColor(m.palette, t.Project, projectColorOverride(m, t.Project))).Render(subFill) + fill(lipgloss.NewStyle().Foreground(dimColor)).Render(subRest)
		parts += hl(theme.CourseColor(m.palette, t.Project, projectColorOverride(m, t.Project))).Render(subFill) + fill(lipgloss.NewStyle().Foreground(dimColor)).Render(subRest)
		packW += lipgloss.Width(subFill) + lipgloss.Width(subRest)
	}
	return pack, packW, parts
}

// cardMetaFits reports whether the pack unit fits after stripe+chip+due
// on one line at card width w.
func cardMetaFits(m Model, t *core.Task, w int, now time.Time, head map[string][2]int) bool {
	isDone := func(p, st string) bool { return m.store.IsDoneStatus(p, st) }
	done := isDone(t.Project, t.Status)
	_, packW, _ := cardMetaPack(m, t, now, head, lipgloss.Color(m.palette.SurfaceRaised))
	if packW == 0 {
		return true
	}
	prefixW := 2 + chipLabelWidth(t.Priority) + 1
	if label := cardDueLabel(m, t, now, done); label != "" {
		prefixW += lipgloss.Width(label) + 1
	}
	return prefixW+packW <= w
}

// boardSlotHeights measures each visible card slot: 5 lines normally,
// 6 when the tags+sub-bar pack wraps. Shared by the renderer and the
// mouse hit test so they always agree on row positions.
func boardSlotHeights(m Model, cols [][]*core.Task, w, bodyH int) [][2]int {
	winStart, visCols, colW := boardLayout(m, w)
	view := winStart + visCols
	head := subtaskCounts(m.store.Tasks(), func(p, st string) bool { return m.store.IsDoneStatus(p, st) })
	now := m.clock.Now()
	maxSlots := 0
	for i := winStart; i < view; i++ {
		if len(cols[i]) > maxSlots {
			maxSlots = len(cols[i])
		}
	}
	slots := [][2]int{}
	y := 0
	for r := 0; ; r++ {
		h := boardCardH
		for i := winStart; i < view; i++ {
			if r < len(cols[i]) && !cardMetaFits(m, cols[i][r], colW, now, head) {
				h = boardCardH + 1
				break
			}
		}
		if y+h > bodyH {
			break
		}
		slots = append(slots, [2]int{y, h})
		y += h
		if r+1 >= maxSlots {
			break
		}
	}
	return slots
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
	bgFill := lipgloss.NewStyle().Background(cardBg)

	stripe := hl(cardBg).Render("▌")

	// Title wrapped to max 2 lines.
	titleW := w - 3
	cut1, cut2 := wrapTitle(t.Title, titleW)
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Text)).Background(cardBg)
	padTitle := func(title string) string {
		var titleStyled string
		if done {
			titleStyled = strikeTitle(lipgloss.Color(m.palette.Muted), cardBg, title)
		} else {
			titleStyled = titleStyle.Render(title)
		}
		if pad := titleW - lipgloss.Width(titleStyled); pad > 0 {
			return titleStyled + bgFill.Render(strings.Repeat(" ", pad))
		}
		return titleStyled
	}
	line1 := stripe + bgFill.Render(" ") + padTitle(cut1)
	line2 := stripe + bgFill.Render(" ") + padTitle(cut2)

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
	spCard := func(n int) string { return bgFill.Render(strings.Repeat(" ", n)) }
	meta := stripe + spCard(1) + chip + spCard(1)
	metaPrefixW := 2 + lipgloss.Width(chip) + 1

	if t.Due != "" || (done && (t.Completed != "" || t.Updated != "")) {
		dc := lipgloss.NewStyle().Background(cardBg)
		var label string
		if done {
			label = doneLabelFor(t, now)
			dc = dc.Foreground(lipgloss.Color(m.palette.Muted))
		} else {
			l, heat := gauges.DueLabel(t.Due, now)
			label = l
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
		}
		dueW := lipgloss.Width(label)
		meta += dc.Render(label) + spCard(1)
		metaPrefixW += dueW + 1
	}

	// Tags and the subtask mini-bar form ONE pack unit ("#exam ▇▇ 4/6")
	// separated by exactly 1 cell; it wraps as a unit to a second line
	// when it doesn't fit.
	pack, packW, packParts := cardMetaPack(m, t, now, head, cardBg)

	metaLines := []string{}
	if pack != "" && metaPrefixW+packW <= w {
		meta += pack
		metaLines = append(metaLines, padRightCard(meta, w, cardBg))
	} else if pack != "" {
		metaLines = append(metaLines, padRightCard(meta, w, cardBg))
		metaLines = append(metaLines, padRightCard(stripe+spCard(1)+packParts, w, cardBg))
	} else {
		metaLines = append(metaLines, padRightCard(meta, w, cardBg))
	}

	lines := append([]string{line1, line2}, metaLines...)

	if selected {
		top := border.Render("╭" + strings.Repeat("─", w-2) + "╮")
		bot := border.Render("╰" + strings.Repeat("─", w-2) + "╯")
		framed := []string{top}
		for _, l := range lines {
			framed = append(framed, border.Render("│")+padRightCard(l, w-2, cardBg)+border.Render("│"))
		}
		framed = append(framed, bot)
		return strings.Split(paint(strings.Join(framed, "\n"), w, len(framed), cardBg), "\n")
	}
	// The 5-line card is [blank, title1, title2, meta, blank]; a wrapped
	// meta grows the card to 6 lines ([..., meta1, meta2, blank]).
	blank := fill(lipgloss.NewStyle()).Render(strings.Repeat(" ", w))
	out := []string{blank}
	for _, l := range lines {
		out = append(out, padRightCard(l, w, cardBg))
	}
	out = append(out, blank)
	return strings.Split(paint(strings.Join(out, "\n"), w, len(out), cardBg), "\n")
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
	// Header row: the inset chevrons scroll the column window (the window
	// derives from boardCol, so this mirrors h/l).
	if y == 1 {
		winStart, visCols, _ := boardLayout(m, innerW)
		n := len(m.boardStatuses())
		switch {
		case x == 1 && winStart > 0:
			m.boardCol = clampInt(m.boardCol-1, 0, n-1)
			m.syncCursorToBoardCard()
			return m, nil
		case x == innerW && winStart+visCols < n:
			m.boardCol = clampInt(m.boardCol+1, 0, n-1)
			m.syncCursorToBoardCard()
			return m, nil
		}
		return m, nil
	}
	winStart, visCols, colW := boardLayout(m, innerW)
	if visCols < 1 {
		return m, nil
	}
	local := x - 2 // panel pad (1) + body inset (1)
	view := winStart + visCols
	bodyH := contentHForClick(m)
	slots := boardSlotHeights(m, cols, innerW, bodyH)
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
	row := -1
	line := y - 3 // pad(1) + header(2)
	for idx, sl := range slots {
		if line >= sl[0] && line < sl[0]+sl[1] {
			row = idx
			break
		}
	}
	if row < 0 {
		return m, nil
	}

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
