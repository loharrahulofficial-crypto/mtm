package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
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

func center(s string, w int) string {
	sd := runewidth.StringWidth(s)
	if sd >= w {
		return truncatePlain(s, w)
	}
	left := (w - sd) / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", w-left-sd)
}

func renderBoard(m Model, w, h int) string {
	s := m.styles
	now := time.Now()
	_ = now
	statuses := m.boardStatuses()
	cols := boardColumns(m)

	n := len(statuses)
	if n == 0 {
		return s.dim.Render("no statuses")
	}
	colW := (w - (n - 1)) / n
	if colW < 14 {
		colW = 14
	}

	sep := s.dim.Render("│")

	var b strings.Builder
	// Header line.
	headerCells := make([]string, 0, n)
	for i, st := range statuses {
		count := 0
		if i < len(cols) {
			count = len(cols[i])
		}
		label := fmt.Sprintf("%s %d", strings.ToUpper(st.Name), count)
		if i == m.boardCol {
			headerCells = append(headerCells, s.accent.Width(colW).Render(center(label, colW)))
		} else {
			headerCells = append(headerCells, s.dim.Width(colW).Render(center(label, colW)))
		}
	}
	b.WriteString(strings.Join(headerCells, sep) + "\n")
	b.WriteString(s.dim.Render(strings.Repeat("─", w)) + "\n")

	bodyH := h - 3
	if bodyH < 3 {
		bodyH = 3
	}
	maxCards := bodyH / 3
	for r := 0; r < maxCards; r++ {
		cardLines := make([][]string, n)
		maxLines := 3
		for i := 0; i < n; i++ {
			if r < len(cols[i]) {
				sel := i == m.boardCol && r == m.boardRow && m.focus != focusSidebar
				cardLines[i] = renderCard(m, cols[i][r], colW, sel, now)
			} else {
				cardLines[i] = []string{strings.Repeat(" ", colW), strings.Repeat(" ", colW), strings.Repeat(" ", colW)}
			}
		}
		for lineIdx := 0; lineIdx < maxLines; lineIdx++ {
			cells := make([]string, n)
			for i := 0; i < n; i++ {
				cells[i] = cardLines[i][lineIdx]
			}
			b.WriteString(strings.Join(cells, sep) + "\n")
		}
	}
	// Overflow indicator.
	var overflow []string
	anyOverflow := false
	for i := 0; i < n; i++ {
		extra := 0
		if i < len(cols) {
			extra = len(cols[i]) - maxCards
		}
		label := ""
		if extra > 0 {
			anyOverflow = true
			label = fmt.Sprintf(" +%d more", extra)
		}
		overflow = append(overflow, s.dim.Width(colW).Render(label))
	}
	if anyOverflow {
		b.WriteString(strings.Join(overflow, sep) + "\n")
	}
	// Pad remaining height.
	for lineCount := strings.Count(b.String(), "\n"); lineCount < h-1; lineCount++ {
		b.WriteString(strings.Repeat(" ", w) + "\n")
	}
	b.WriteString(s.dim.Render("  h/l column · H/L move card · x done · D due · a add"))
	out := b.String()
	return strings.TrimSuffix(out, "\n")
}

func renderCard(m Model, t *core.Task, w int, selected bool, now time.Time) []string {
	s := m.styles
	isDone := func(p, st string) bool { return m.store.IsDoneStatus(p, st) }
	done := isDone(t.Project, t.Status)

	inner := w - 1
	title := truncatePlain(t.Title, inner-1)
	pill := priorityPlain(t.Priority)
	due := duePlain(t.Due, now)
	meta := pill + " " + due
	meta = truncatePlain(meta, inner-1)

	if selected {
		return []string{
			s.selected.Width(w).Render(" " + title),
			s.selected.Width(w).Render(" " + meta),
			s.selected.Width(w).Render(""),
		}
	}
	l1 := " " + title
	l2 := " " + meta
	if done {
		l1 = s.done.Render(l1)
		l2 = s.dim.Render(l2)
	}
	return []string{
		padRight(l1, w),
		padRight(l2, w),
		strings.Repeat(" ", w),
	}
}

// clickBoard handles mouse press/release on the board. Columns are laid out
// left→right; a press selects the card under the cursor, a release moves it
// into the column under the cursor (drag-to-move).
func (m Model) clickBoard(x, y int, release bool) (tea.Model, tea.Cmd) {
	cols := boardColumns(m)
	n := len(cols)
	if n == 0 {
		return m, nil
	}
	mainW := m.width
	if sw := sideWidth(m.width, m.sidebarHidden); sw > 0 {
		mainW -= sw
	}
	if dw := detailWidth(m.width, m.detailOn); dw > 0 {
		mainW -= dw
	}
	sepW := 1
	colW := (mainW - (n-1)*sepW) / n
	if colW < 14 {
		colW = 14
	}
	stride := colW + sepW
	col := x / stride
	if x < 0 || col >= n {
		if release {
			m.boardDragging = false
		}
		return m, nil
	}
	row := (y - 2) / 3 // header(0) + separator(1), each card is 3 lines

	if release {
		if !m.boardDragging {
			return m, nil
		}
		m.boardDragging = false
		card := m.selectedBoardTask()
		if card == nil {
			return m, nil
		}
		if col == m.boardCol {
			return m, nil
		}
		statuses := m.boardStatuses()
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

	// Press: select card and begin drag.
	if row >= 0 && col < n && row < len(cols[col]) {
		m.boardCol, m.boardRow = col, row
		m.boardDragging = true
		m.syncCursorToBoardCard()
	}
	return m, nil
}
