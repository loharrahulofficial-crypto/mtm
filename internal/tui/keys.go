package tui

import (
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) dispatch(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clampDetailAndSidebar()
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tea.KeyMsg:
		return m.updateKey(msg)
	case tea.MouseMsg:
		return m.updateMouse(msg)
	}
	return m, nil
}

// clampDetailAndSidebar hides panes on narrow terminals (auto behavior;
// explicit toggles flip the *On flags).
func (m *Model) clampDetailAndSidebar() {
	// panes collapse purely at render time from width; nothing to stow
}

const (
	minSidebarWidth   = 18
	preferredSidebW   = 26
	minDetailWidth    = 30
	preferredDetailW  = 36
	sidebarAutoHideAt = 78
	detailAutoHideAt  = 116
	narrowHideAllAt   = 56
)

func sideWidth(width int, hidden bool) int {
	if hidden || width < sidebarAutoHideAt {
		return 0
	}
	w := width / 4
	if w > preferredSidebW {
		w = preferredSidebW
	}
	if w < minSidebarWidth {
		w = minSidebarWidth
	}
	if width < narrowHideAllAt {
		w = 0
	}
	return w
}

func detailWidth(width int, on bool) int {
	if !on || width < detailAutoHideAt {
		return 0
	}
	w := width / 3
	if w > preferredDetailW {
		w = preferredDetailW
	}
	if w < minDetailWidth {
		w = minDetailWidth
	}
	return w
}

func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeInput:
		return m.updateInputKey(msg)
	case modePicker:
		return m.updatePickerKey(msg)
	case modeConfirm:
		return m.updateConfirmKey(msg)
	case modeHelp:
		switch msg.String() {
		case "esc", "q", m.keymap.Seq(aHelp):
			m.mode = modeNormal
		}
		return m, nil
	case modeConflict:
		return m.updateConflictKey(msg)
	case modeNotes:
		return m.updateNotesKey(msg)
	default:
		return m.updateNormalKey(msg)
	}
}

func (m Model) updateNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()

	// Multi-key sequences (gg, dd): accumulate against the prefix buffer.
	if m.pendingKey != "" {
		seq := m.pendingKey + s
		m.pendingKey = ""
		if act := m.keymap.Match(seq); act != "" {
			return m.doAction(act)
		}
		// Fall through: treat current key as fresh input.
	}
	if m.keymap.IsPrefix(s) {
		m.pendingKey = s
		return m, m.bumpPending()
	}
	if act := m.keymap.Match(s); act != "" {
		return m.doAction(act)
	}
	// Command-line style alternatives: escape clears transient state.
	switch s {
	case "esc":
		if m.searchQuery != "" {
			m.searchQuery = ""
			m.refreshTasks()
			m.sidebarItems = m.buildSidebar()
			return m, nil
		}
	case "ctrl+c":
		return m.quitWithFlush()
	}
	return m, nil
}

func (m Model) quitWithFlush() (tea.Model, tea.Cmd) {
	if len(m.pendingCommits) > 0 {
		commitOnly(m.cfg, m.root, m.pendingCommits)
		m.pendingCommits = nil
	}
	return m, tea.Quit
}

func (m Model) doAction(act string) (tea.Model, tea.Cmd) {
	switch act {
	case aQuit:
		return m.quitWithFlush()
	case aViewList:
		m.view = viewList
		return m, nil
	case aViewBoard:
		m.view = viewBoard
		m.resetBoardCursor()
		return m, nil
	case aViewCalendar:
		m.view = viewCalendar
		return m, nil
	case aCycleView:
		m.view = (m.view + 1) % 3
		return m, nil
	case aHelp:
		m.mode = modeHelp
		m.helpSel = 0
		return m, nil
	case aDetail:
		m.detailOn = !m.detailOn
		m.clampScrolls()
		return m, nil
	case aSidebar:
		m.sidebarHidden = !m.sidebarHidden
		return m, nil
	case aFocusLeft:
		m.focus = (m.focus + 2) % 3
		return m, nil
	case aFocusRight:
		m.focus = (m.focus + 1) % 3
		return m, nil
	case aSync:
		if m.syncSt == syncConflict {
			m.statusMsg = "sync paused — resolve conflicts first (ctrl+x)"
			return m, nil
		}
		m.syncSt = syncSyncing
		m.statusMsg = "syncing…"
		msgs := m.pendingCommits
		m.pendingCommits = nil
		return m, m.flushWith(msgs)
	case aConflicts:
		if st, err := m.repo.LoadState(); err == nil && st.Conflicted {
			m.startConflictMode(st.Files, "sync paused — conflicts need resolution")
			return m, nil
		}
		m.statusMsg = "no conflicts pending"
		return m, nil
	}

	// Navigation differs per view.
	switch m.view {
	case viewList:
		return m.updateListKey(act)
	case viewBoard:
		return m.updateBoardKey(act)
	case viewCalendar:
		return m.updateCalendarKey(act)
	}
	return m, nil
}

// --- shared helpers used by list/board/calendar key handlers ---

func (m Model) focusedOnSidebar() bool { return m.focus == focusSidebar }

// moveSelection moves the main-pane cursor by delta rows, or the sidebar
// selection when the sidebar is focused.
func (m *Model) moveSelection(delta int) {
	if m.focusedOnSidebar() {
		m.moveSidebarSelection(delta)
		return
	}
	m.cursor += delta
	m.clampCursor()
	m.ensureCursorVisible()
}

func (m *Model) moveSidebarSelection(delta int) {
	if len(m.sidebarItems) == 0 {
		return
	}
	m.sidebarSel += delta
	if m.sidebarSel < 0 {
		m.sidebarSel = 0
	}
	if m.sidebarSel >= len(m.sidebarItems) {
		m.sidebarSel = len(m.sidebarItems) - 1
	}
	m.refreshTasks()
}

// ensureCursorVisible scrolls the list window so cursor is on screen.
func (m *Model) ensureCursorVisible() {
	visible := m.visibleListRows()
	if visible <= 0 {
		return
	}
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+visible {
		m.scroll = m.cursor - visible + 1
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
}

func (m Model) visibleListRows() int {
	h := m.height - 2 // footer + top padding
	if h < 1 {
		return 1
	}
	return h
}

// updateMouse routes clicks/wheel/drag to the right pane/view.
func (m Model) updateMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.mode != modeNormal {
		// Mouse does nothing while overlays are open (except conflict list).
		if m.mode == modeConflict && msg.Type == tea.MouseLeft {
			return m.updateConflictMouse(msg)
		}
		return m, nil
	}
	sidebarW := sideWidth(m.width, m.sidebarHidden)
	detailW := detailWidth(m.width, m.detailOn)
	mainW := m.width - sidebarW - detailW
	x, y := msg.X, msg.Y

	switch msg.Type {
	case tea.MouseWheelUp:
		m.focusTo(x, sidebarW, mainW, detailW)
		if m.focus == focusDetail {
			m.detailScroll = clampInt(m.detailScroll-3, 0, 1<<30)
			return m, nil
		}
		m.moveSelection(-3)
		return m, nil
	case tea.MouseWheelDown:
		m.focusTo(x, sidebarW, mainW, detailW)
		if m.focus == focusDetail {
			m.detailScroll = clampInt(m.detailScroll+3, 0, 1<<30)
			return m, nil
		}
		m.moveSelection(+3)
		return m, nil
	}

	mainX := x
	if sidebarW > 0 {
		mainX = x - sidebarW
	}
	if detailW > 0 && x >= m.width-detailW {
		mainX = -1 // not over the main pane
	}
	switch m.view {
	case viewBoard:
		// Kanban owns its mouse handling (click + drag).
		switch msg.Type {
		case tea.MouseLeft:
			return m.clickBoard(mainX, y, false)
		case tea.MouseMotion, tea.MouseRelease:
			if m.boardDragging {
				return m.clickBoard(mainX, y, msg.Type == tea.MouseRelease)
			}
		}
		return m, nil
	}

	if msg.Type != tea.MouseLeft {
		return m, nil
	}

	// Footer tabs?
	footerY := m.height - 1
	if y == footerY {
		switch {
		case x <= 12:
			m.view = viewList
		case x <= 24:
			m.view = viewBoard
		case x <= 36:
			m.view = viewCalendar
		}
		return m, nil
	}

	// Sidebar clicks.
	if sidebarW > 0 && x < sidebarW {
		m.focus = focusSidebar
		if idx := m.sidebarIndexAt(y); idx >= 0 {
			m.sidebarSel = idx
			m.refreshTasks()
			m.sidebarItems = m.buildSidebar()
			m.focus = focusMain
		}
		return m, nil
	}

	// Detail pane: clicking scrolls? keep focus.
	if detailW > 0 && x >= m.width-detailW {
		m.focus = focusDetail
		return m, nil
	}

	// Main pane: rows.
	m.focus = focusMain
	switch m.view {
	case viewList:
		if idx := m.listIndexAt(y, 0); idx >= 0 {
			m.cursor = idx
			m.clampCursor()
		}
	case viewCalendar:
		m.clickCalendar(mainX, y)
	}
	return m, nil
}

func (m *Model) focusTo(x, sidebarW, mainW, detailW int) {
	_ = mainW
	switch {
	case sidebarW > 0 && x < sidebarW:
		m.focus = focusSidebar
	case detailW > 0 && x >= m.width-detailW:
		m.focus = focusDetail
	default:
		m.focus = focusMain
	}
}

// listIndexAt maps a y coordinate (0-based from top of content) to a row index.
func (m Model) listIndexAt(y, topPad int) int {
	idx := m.scroll + (y - topPad)
	if idx < 0 || idx >= len(m.rows) {
		return -1
	}
	return idx
}

func (m Model) sidebarIndexAt(y int) int {
	row := 0
	for _, r := range sidebarRows(m.sidebarItems) {
		if y == row {
			return r.sel // -1 for headers (not selectable)
		}
		row++
	}
	return -1
}
