package tui

import (
	"os"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/ambient"
)

// dispatch arms the ambient ticker around every message so the corner/
// focus scene starts and stops with the visible surface. The raw handler
// is wrapped so both paths settle in the same final model copy.
func (m Model) dispatch(msg tea.Msg) (tea.Model, tea.Cmd) {
	res, cmd := m.dispatchRaw(msg)
	if nm, ok := res.(Model); ok {
		cmds := []tea.Cmd{}
		if armCmd := nm.armAmbient(); armCmd != nil {
			cmds = append(cmds, armCmd)
		}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		// A view change anywhere arms the page-fade; every entry point
		// (keys, mouse, palette, sidebar) funnels through here.
		if nm.view != m.view && !nm.reducedMotionActive() {
			nm.viewFadeStart = m.clock.Now()
			cmds = append(cmds, viewFadeTickCmd())
		}
		if len(cmds) == 1 {
			return nm, cmds[0]
		}
		return nm, tea.Batch(cmds...)
	}
	return res, cmd
}

func (m Model) dispatchRaw(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ambientTickMsg:
		return m.handleAmbientTick()
	case tea.FocusMsg:
		m.ambientFocused = true
		m.lastInput = m.clock.Now()
		return m, m.armAmbient()
	case tea.BlurMsg:
		m.ambientFocused = false
		m.ambientArmed = false
		return m, nil
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		// Re-read the cell pixel size on resize (SIGWINCH usually also
		// changes the pixel geometry). ioctl only — no XTWINOPS round
		// trip mid-TUI; keep the last known size when it reports none.
		if cp, ok := ambient.WinsizeCellPixels(os.Stdout); ok {
			m.cellW, m.cellH = cp.W, cp.H
		}
		m.clampDetailAndSidebar()
		return m, m.armAmbient()
	case spinner.TickMsg:
		// The spinner renders only while a sync is in flight; anywhere else
		// the tick is dropped and the chain stops instead of ticking forever.
		if m.syncSt != syncSyncing {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tea.KeyMsg:
		m.lastInput = m.clock.Now()
		if m.mode == modeSplash {
			// Any key skips the splash with a quick fade: jump to the
			// dissolve tail, keeping the frame flow.
			m.skipSplash()
			return m, m.armAmbient()
		}
		return m.updateKey(msg)
	case tea.MouseMsg:
		m.lastInput = m.clock.Now()
		if m.mode == modeSplash {
			m.skipSplash()
			return m, m.armAmbient()
		}
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
		case "j", "down":
			m.helpScroll++
			return m, nil
		case "k", "up":
			if m.helpScroll > 0 {
				m.helpScroll--
			}
			return m, nil
		case "pgdown", "ctrl+d":
			m.helpScroll += 8
			return m, nil
		case "pgup", "ctrl+u":
			m.helpScroll -= 8
			if m.helpScroll < 0 {
				m.helpScroll = 0
			}
			return m, nil
		case "g":
			m.helpScroll = 0
			return m, nil
		case "G":
			m.helpScroll = 1 << 30 // clamped at render
			return m, nil
		default:
			m.helpScroll = 0
			m.mode = modeNormal
			return m, nil
		}
	case modeConflict:
		return m.updateConflictKey(msg)
	case modeNotes:
		return m.updateNotesKey(msg)
	case modeFocus:
		return m.updateFocusKey(msg)
	case modeAmbient:
		return m.updateAmbientKey(msg)
	case modePalette:
		return m.updatePaletteKey(msg)
	default:
		return m.updateNormalKey(msg)
	}
}

func (m Model) updateNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	s := msg.String()

	// Page Down/Up mirror the ctrl+d/ctrl+u page bindings everywhere.
	if s == "pgdown" {
		s = m.keymap.Seq(aPageDown)
	} else if s == "pgup" {
		s = m.keymap.Seq(aPageUp)
	}

	// The project page owns its navigation keys while open (j/k/h/l, tabs,
	// pan, esc). Everything else (palette, sync, help, ctrl+c, ...) keeps
	// flowing through the regular handlers — the page sits underneath the
	// overlays.
	if m.projectPage != "" {
		switch s {
		case "j", "k", "up", "down", "h", "l", "[", "]", "g", "G",
			m.keymap.Seq(aPageDown), m.keymap.Seq(aPageUp),
			"ctrl+d", "ctrl+u", "esc", "q", "enter":
			return m.updateProjectPageKey(msg)
		}
	}

	// Courses view: a bare grade key records a grade (gg's top-of-list
	// binding is sacrificed there; the rest of the board keeps gg). The
	// prefix check must not swallow it, so it runs before the multi-key
	// buffer. aGrade exists in the keymap so help renders it.
	if s == m.keymap.Seq(aGrade) && m.view == viewCourses && m.mode == modeNormal {
		return m.openGradePalette()
	}

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
		// Drilled into a course from the Courses view? esc returns there.
		if m.coursesDrillIn != "" {
			m.coursesDrillIn = ""
			m.view = viewCourses
			m.coursesScroll = 0
			if m.coursesDrillFrom < 0 {
				m.coursesDrillFrom = 0
			}
			if m.coursesDrillFrom >= 0 && m.courseSel != m.coursesDrillFrom {
				m.courseSel = m.coursesDrillFrom
			}
			return m, nil
		}
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
	case aViewHome:
		m.view = viewHome
		return m, nil
	case aViewCalendar:
		m.view = viewCalendar
		return m, nil
	case aViewCourses:
		m.view = viewCourses
		m.coursesScroll = 0
		return m, nil
	case aViewStats:
		m.view = viewStats
		m.statsScroll = 0
		return m, nil
	case aViewIdeas:
		m.view = viewIdeas
		m.ideaSel = 0
		m.ideaScroll = 0
		return m, nil
	case aCycleView:
		m.view = (m.view + 1) % 7
		return m, nil
	case aFocus:
		if t := m.selectedTask(); t != nil {
			return m.enterFocusTask(t)
		}
		m.statusMsg = "no task selected to focus"
		return m, nil
	case aCycleScene:
		m.cycleScene(1)
		return m, m.armAmbient()
	case aPalette, aPaletteColon:
		return m, m.openPalette()
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
	case aEnter:
		// Sidebar enter on a project/course item opens the project page.
		if m.focus == focusSidebar {
			if sel := m.sidebarSelection(); sel != nil && (sel.Kind == "project" || sel.Kind == "course") {
				cmd := m.openProjectPage(sel.Key, projectTabOverview)
				return m, cmd
			}
			return m, nil
		}
		switch m.view {
		case viewCourses:
			return m.updateCoursesKey(aEnter)
		}
		return m, nil
	case aSync:
		return m.startSync()
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
	case viewHome:
		return m.updateHomeKey(act)
	case viewCourses:
		return m.updateCoursesKey(act)
	case viewStats:
		return m.updateStatsKey(act)
	case viewIdeas:
		return m.updateIdeasKey(act)
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
	if sel := m.sidebarSelection(); sel != nil {
		switch sel.Kind {
		case "home":
			m.view = viewHome
		default:
			if m.view == viewHome {
				m.view = viewList
			}
		}
	}
	m.refreshTasks()
}

// ensureCursorVisible scrolls the list window so cursor is on screen.
// Scroll is tracked in body-line units (group headers consume lines).
func (m *Model) ensureCursorVisible() {
	visible := m.visibleListRows()
	if visible <= 0 {
		return
	}
	cursorLine := m.rowsLine(m.cursor)
	if cursorLine < m.scroll {
		m.scroll = cursorLine
	}
	if cursorLine >= m.scroll+visible {
		m.scroll = cursorLine - visible + 1
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
}

func (m Model) visibleListRows() int {
	// The list body excludes the footer (1), the panel chrome (2) and the
	// pinned header row (1).
	h := m.height - 1 - 2 - 1
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
	case tea.MouseWheelLeft:
		if m.view == viewBoard {
			m.boardCol = clampInt(m.boardCol-1, 0, len(m.boardStatuses())-1)
			m.syncCursorToBoardCard()
			return m, nil
		}
	case tea.MouseWheelRight:
		if m.view == viewBoard {
			m.boardCol = clampInt(m.boardCol+1, 0, len(m.boardStatuses())-1)
			m.syncCursorToBoardCard()
			return m, nil
		}
	case tea.MouseWheelUp:
		m.focusTo(x, sidebarW, mainW, detailW)
		if m.focus == focusDetail {
			m.detailScroll = clampInt(m.detailScroll-3, 0, m.maxDetailScroll())
			return m, nil
		}
		if m.view == viewCourses && m.focus != focusSidebar {
			m.coursesScroll = clampInt(m.coursesScroll-3, 0, m.coursesScrollBounds())
			return m, nil
		}
		if m.view == viewStats && m.focus != focusSidebar {
			m.statsScroll = clampInt(m.statsScroll-3, 0, m.statsScrollBounds())
			return m, nil
		}
		if m.view == viewBoard && y == 1 && m.focus != focusSidebar {
			// Hovering the header: the wheel slides the column window.
			m.boardCol = clampInt(m.boardCol-1, 0, len(m.boardStatuses())-1)
			m.syncCursorToBoardCard()
			return m, nil
		}
		m.moveSelection(-3)
		return m, nil
	case tea.MouseWheelDown:
		m.focusTo(x, sidebarW, mainW, detailW)
		if m.focus == focusDetail {
			m.detailScroll = clampInt(m.detailScroll+3, 0, m.maxDetailScroll())
			return m, nil
		}
		if m.view == viewCourses && m.focus != focusSidebar {
			m.coursesScroll = clampInt(m.coursesScroll+3, 0, m.coursesScrollBounds())
			return m, nil
		}
		if m.view == viewStats && m.focus != focusSidebar {
			m.statsScroll = clampInt(m.statsScroll+3, 0, m.statsScrollBounds())
			return m, nil
		}
		if m.view == viewBoard && y == 1 && m.focus != focusSidebar {
			// Hovering the header: the wheel slides the column window.
			m.boardCol = clampInt(m.boardCol+1, 0, len(m.boardStatuses())-1)
			m.syncCursorToBoardCard()
			return m, nil
		}
		m.moveSelection(+3)
		return m, nil
	}

	mainX := x
	if sidebarW > 0 {
		mainX = x - sidebarW - 1 // sidebar + gutter
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
	if y == m.height-1 {
		if v, ok := m.footerPillAt(x); ok {
			m.view = v
			if v == viewBoard {
				m.resetBoardCursor()
			}
		}
		return m, nil
	}

	// Sidebar clicks.
	if sidebarW > 0 && x < sidebarW {
		m.focus = focusSidebar
		if idx := m.sidebarIndexAt(y); idx >= 0 {
			m.sidebarSel = idx
			if sel := m.sidebarSelection(); sel != nil {
				switch sel.Kind {
				case "home":
					m.view = viewHome
				default:
					if m.view == viewHome {
						m.view = viewList
					}
				}
			}
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
		if idx := m.listIndexAt(y, 2); idx >= 0 {
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

// listIndexAt maps a y coordinate (0-based from top of content, including
// the panel's top chrome row) to a row index. Group-header lines map to -1.
func (m Model) listIndexAt(y, topPad int) int {
	line := m.scroll + (y - topPad)
	if i := searchRowLine(m.rowLines, line); i >= 0 {
		return i
	}
	return -1
}

func searchRowLine(rowLines []int, line int) int {
	for i, l := range rowLines {
		if l == line {
			return i
		}
	}
	return -1
}

func (m Model) sidebarIndexAt(y int) int {
	// Match the renderer: panel top chrome (1 row), then the brand block,
	// then visible rows starting at `top` (sidebarRowPlan).
	innerW := sideWidth(m.width, m.sidebarHidden) - 2
	if innerW < 1 {
		innerW = 1
	}
	innerH := m.height - 1 - 2
	top, _, _, _ := sidebarRowPlan(m, innerW, innerH)
	row := 1 + brandLines
	for i, r := range sidebarRows(m.sidebarItems) {
		if i < top {
			continue
		}
		if y == row {
			return r.sel // -1 for headers (not selectable)
		}
		row++
	}
	return -1
}

// startSync begins a debounced sync cycle (shared by key binding + palette).
func (m Model) startSync() (Model, tea.Cmd) {
	if m.syncSt == syncConflict {
		m.statusMsg = "sync paused — resolve conflicts first (ctrl+x)"
		return m, nil
	}
	m.syncSt = syncSyncing
	m.statusMsg = "syncing…"
	msgs := m.pendingCommits
	m.pendingCommits = nil
	return m, m.flushWith(msgs)
}

// footerPillAt maps an x coordinate on the statusline to the view pill under it.
func (m Model) footerPillAt(x int) (viewID, bool) {
	pos := 0
	for _, seg := range m.footerPillSegments(m.width) {
		w := lipglossWidth(seg.rendered)
		if x >= pos && x < pos+w {
			return seg.view, true
		}
		pos += w
	}
	return 0, false
}

// updateHomeKey handles Home: j/k move the timeline cursor, other actions
// fall through to the shared task actions (which act on the Home selection).
func (m Model) updateHomeKey(act string) (tea.Model, tea.Cmd) {
	// When the sidebar is focused, j/k/pgup/pgdn drive it like everywhere else.
	if m.focus == focusSidebar {
		switch act {
		case aUp:
			m.moveSidebarSelection(-1)
			return m, nil
		case aDown:
			m.moveSidebarSelection(+1)
			return m, nil
		case aPageUp:
			m.moveSidebarSelection(-5)
			return m, nil
		case aPageDown:
			m.moveSidebarSelection(+5)
			return m, nil
		case aTop:
			m.sidebarSel = 0
			m.refreshTasks()
			return m, nil
		case aBottom:
			m.sidebarSel = len(m.sidebarItems) - 1
			m.refreshTasks()
			return m, nil
		}
		return m, nil
	}
	n := len(m.homeRows())
	switch act {
	case aUp:
		if m.homeSel > 0 {
			m.homeSel--
		}
		return m, nil
	case aDown:
		if m.homeSel < n-1 {
			m.homeSel++
		}
		return m, nil
	case aPageUp:
		m.homeSel -= 6
		if m.homeSel < 0 {
			m.homeSel = 0
		}
		return m, nil
	case aPageDown:
		m.homeSel += 6
		if m.homeSel > n-1 {
			m.homeSel = n - 1
		}
		if n == 0 {
			m.homeSel = 0
		}
		return m, nil
	case aTop:
		m.homeSel = 0
		return m, nil
	case aBottom:
		m.homeSel = n - 1
		if m.homeSel < 0 {
			m.homeSel = 0
		}
		return m, nil
	default:
		if m2, cmd, handled := m.updateSimpleTaskAction(act); handled {
			return m2, cmd
		}
	}
	return m, nil
}
