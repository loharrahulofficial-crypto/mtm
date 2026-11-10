package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

type themeTickMsg time.Time
type debounceMsg int
type syncDoneMsg struct {
	note string
	err  error
}

// handleTimerTick advances the shared timer, finalizes stale sessions and
// refreshes the cached view every second. Phase-transition events fire the
// desktop notification themselves.
func (m Model) handleTimerTick() (tea.Model, tea.Cmd) {
	prevPhase := m.timer.Phase
	v, _, auto, err := actions.TickTimer(m.store, m.cfg, true)
	switch {
	case err != nil:
		m.statusMsg = "timer: " + err.Error()
	case auto != nil:
		m.statusMsg = fmt.Sprintf("timer ended while away — logged %s%s", actions.FormatMinutes(auto.Minutes), capSuffix(auto.Capped))
		m.refreshTasks()
		m.sidebarItems = m.buildSidebar()
		m.clampCursor()
		m.timer = core.TimerView{Phase: core.TimerPhaseIdle}
	default:
		m.timer = v
	}
	if phaseChanged := m.timer.Phase != prevPhase; phaseChanged {
		m.phaseFadeStart = m.clock.Now()
		m.phaseFadeFrom = phaseIntensity(prevPhase)
		m.phaseFadeTo = phaseIntensity(m.timer.Phase)
		m.phaseFadeActive = true
	}
	return m, tea.Batch(tea.Tick(time.Second, func(time.Time) tea.Msg { return timerTickMsg{} }), m.armAmbient())
}

func capSuffix(capped bool) string {
	if capped {
		return " (capped)"
	}
	return ""
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

// Init starts the background sync, the theme poll, the 1-second timer tick
// and — for every linked project (bounded) — the GitHub load that feeds the
// sidebar/Home streak chips and the project Overview.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.kickSyncCmd(),
		tea.Tick(themePollInterval, func(t time.Time) tea.Msg { return themeTickMsg(t) }),
		m.spinner.Tick,
		tea.Tick(time.Second, func(time.Time) tea.Msg { return timerTickMsg{} }),
		m.kickGitHubAll(),
	)
}

// kickGitHubAll kicks off background loads for linked projects. Cheap no-op
// when nothing is linked (the common case).
func (m Model) kickGitHubAll() tea.Cmd {
	cmds := []tea.Cmd{}
	projects, _ := m.store.ListProjects()
	for i, p := range projects {
		if i >= 8 {
			break // bound the startup fan-out
		}
		cmd := m.githubLoadCmdFor(p.Name)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// githubLoadCmdFor kicks one background GitHub load when the project is
// linked — the same trigger the project page uses, but callable at startup
// too. Loads are idempotent refreshes, not cached against a session.
func (m *Model) githubLoadCmdFor(project string) tea.Cmd {
	p, err := m.store.GetProject(project)
	if err != nil || (p.GitHub == "" && p.RepoPath == "") {
		return nil
	}
	return m.githubLoadCmd(project)
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
			// A lone "d" waited out the sequence timeout: in the calendar it
			// is the completed-tasks toggle (dd still deletes — the second
			// d resolves the "dd" sequence before the timeout).
			if m.pendingKey == "d" && m.view == viewCalendar {
				m.pendingKey = ""
				return m.doAction(aToggleCalendarDone)
			}
			m.pendingKey = ""
		}
		return m, nil

	case timerTickMsg:
		return m.handleTimerTick()

	case githubInfoMsg:
		return m.handleGitHubInfo(msg)

	case animTickMsg:
		if m.animFrame > 1 {
			m.animFrame--
			return m, animTickCmd()
		}
		m.animFrame = 0
		m.animTaskID = ""
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

// animTickMsg advances the checkbox-pop / bar-ease animation.
const animFrames = 5

type animTickMsg struct{}

func animTickCmd() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return animTickMsg{} })
}

// easedFrac returns frac with an ease-in from the current animation frame,
// so bars sweep up to their new value instead of jumping.
func easedFrac(m Model, frac float64) float64 {
	if m.animFrame <= 0 {
		return frac
	}
	e := float64(animFrames-m.animFrame) / float64(animFrames)
	return frac * e
}

// phaseIntensity maps a timer phase to its ambient palette intensity:
// breaks sit calmer (0.6) than work (1.0); idle is fully vivid.
func phaseIntensity(p string) float64 {
	switch p {
	case core.TimerPhaseBreak:
		return 0.6
	case core.TimerPhaseIdle:
		return 1.0
	default:
		return 1.0
	}
}
