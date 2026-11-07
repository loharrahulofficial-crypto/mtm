package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

type themeTickMsg time.Time
type debounceMsg int
type syncDoneMsg struct {
	note string
	err  error
}
type editorDoneMsg struct {
	kind string // "notes" | "taskfile" | "conflict"
	path string // conflict file or notes temp path
	err  error
}
type pendingKeyMsg int

const (
	themePollInterval = 2 * time.Second
	syncDebounce      = 2 * time.Second
	pendingKeyTimeout = 700 * time.Millisecond
)

// Init starts the background sync and the theme poll ticker.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.kickSyncCmd(),
		tea.Tick(themePollInterval, func(t time.Time) tea.Msg { return themeTickMsg(t) }),
		m.spinner.Tick,
	)
}

func (m Model) kickSyncCmd() tea.Cmd {
	// On start: just pull/push (no commit). Returns syncDoneMsg.
	return func() tea.Msg {
		res := syncFinish(m.cfg, m.root, nil)
		return syncDoneMsg{note: res.note, err: res.err}
	}
}

// afterChange records a mutation: queues an auto-commit, debounces a sync,
// and rebuilds the derived views. Call from every mutating key path.
func (m *Model) afterChange(commitMsg string) tea.Cmd {
	m.pendingCommits = append(m.pendingCommits, commitMsg)
	m.debounceGen++
	m.refreshTasks()
	m.sidebarItems = m.buildSidebar()
	m.clampCursor()
	gen := m.debounceGen
	return tea.Tick(syncDebounce, func(time.Time) tea.Msg { return debounceMsg(gen) })
}

// flushPendingCommits commits the queued changes (one commit each) and then
// syncs. Runs off the UI goroutine.
func (m Model) flushPendingCommits() tea.Cmd {
	if len(m.pendingCommits) == 0 {
		return m.kickSyncCmd()
	}
	msgs := append([]string(nil), m.pendingCommits...)
	return func() tea.Msg {
		res := syncFinish(m.cfg, m.root, msgs)
		return syncDoneMsg{note: res.note, err: res.err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case themeTickMsg:
		m.maybeReloadTheme()
		return m, tea.Tick(themePollInterval, func(t time.Time) tea.Msg { return themeTickMsg(t) })

	case debounceMsg:
		if int(msg) != m.debounceGen {
			return m, nil // a newer change rescheduled the sync
		}
		if m.syncSt == syncSyncing {
			return m, nil // one sync in flight; the done handler re-fires
		}
		msgs := m.pendingCommits
		m.pendingCommits = nil
		m.syncSt = syncSyncing
		if !m.cfg.Sync.Auto {
			m.syncSt = syncOK
			return m, nil
		}
		return m, m.flushWith(msgs)

	case syncDoneMsg:
		return m.handleSyncDone(msg)

	case editorDoneMsg:
		return m.handleEditorDone(msg)

	case pendingKeyMsg:
		if int(msg) == m.pendingGen {
			m.pendingKey = ""
		}
		return m, nil
	}
	return m.dispatch(msg)
}

func (m Model) flushWith(msgs []string) tea.Cmd {
	return func() tea.Msg {
		res := syncFinish(m.cfg, m.root, msgs)
		return syncDoneMsg{note: res.note, err: res.err}
	}
}

func (m Model) handleSyncDone(msg syncDoneMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.err == errSyncConflict:
		m.syncSt = syncConflict
		m.syncNote = msg.note
		st, err := m.repo.LoadState()
		if err == nil && st.Conflicted {
			m.startConflictMode(st.Files, msg.note)
		}
		return m, nil
	case msg.err != nil:
		m.syncSt = syncOffline
		m.syncNote = msg.note
	default:
		m.syncSt = syncOK
		m.syncNote = msg.note
		m.lastSync = m.clock.Now()
	}
	// Changes queued while the sync was in flight get their own debounce.
	if len(m.pendingCommits) > 0 {
		m.debounceGen++
		gen := m.debounceGen
		return m, tea.Tick(syncDebounce, func(time.Time) tea.Msg { return debounceMsg(gen) })
	}
	return m, nil
}

// pendingKeyTimeout resets the gg/dd-style prefix buffer.
func (m *Model) bumpPending() tea.Cmd {
	m.pendingGen++
	gen := m.pendingGen
	return tea.Tick(pendingKeyTimeout, func(time.Time) tea.Msg { return pendingKeyMsg(gen) })
}

// maybeReloadTheme re-reads the Omarchy theme if it changed on disk.
func (m *Model) maybeReloadTheme() {
	cur := theme.PollModTime(theme.StateDir())
	if !cur.After(m.themeMod) {
		return
	}
	pal, err := theme.Load()
	if err != nil {
		m.statusMsg = "theme reload failed: " + err.Error()
		return
	}
	m.palette = pal
	m.styles = newStyles(pal)
	m.themeMod = cur
	m.glamourRenderer = nil // glamour rebuilt lazily
	m.statusMsg = "theme: " + pal.Name
}
