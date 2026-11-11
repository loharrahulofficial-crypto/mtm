// Package tui implements the mtm terminal user interface (Bubble Tea).
// It contains no business logic: tasks are created, changed, completed and
// archived through internal/actions, core, store and sync.
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"

	"github.com/loharrahulofficial-crypto/mtm/internal/actions"
	"github.com/loharrahulofficial-crypto/mtm/internal/ambient"
	"github.com/loharrahulofficial-crypto/mtm/internal/clock"
	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

type viewID int

// frameCache holds the last block View() produced. The model is copied by
// value across every message, so the cache lives behind a pointer shared by
// all copies. Single-goroutine UI: no lock.
type frameCache struct {
	view  string
	stamp int64
	scene int64
	w, h  int
}

// panelKey keys every pane-level memo. stamp is the view stamp (any
// visible-state change), scene is the ambient stamp (nonzero only on
// scene-bearing panes, so ambient ticks leave other panes cached),
// theme is the theme version, focus is the pane's shared focus bit.
type panelKey struct {
	stamp int64
	scene int64
	w, h  int
	theme int64
	focus bool
}

// cachedPanel is one memoized string-level pane render (footer, detail,
// main). sidebar uses sidebarCache internally.
type cachedPanel struct {
	key   panelKey
	value string
	ok    bool
}

// sidebarCache memoizes only the static body rows (brand block, section
// items, pinned exam chip, filler). On ambient ticks the animated window
// is re-rendered and spliced at winAt; the static rows are never touched.
type sidebarCache struct {
	key   panelKey
	lines []string
	winAt int
	winH  int
}

type panelCache struct {
	main    cachedPanel
	detail  cachedPanel
	footer  cachedPanel
	sidebar sidebarCache
}

const (
	viewHome viewID = iota
	viewList
	viewBoard
	viewCalendar
	viewCourses
	viewStats
	viewIdeas
)

type focusID int

const (
	focusSidebar focusID = iota
	focusMain
	focusDetail
)

type modeID int

const (
	modeNormal   modeID = iota
	modeInput           // bottom input bar; see inputKind
	modePicker          // status/priority picker overlay
	modeConfirm         // yes/no confirm (delete / archive-all)
	modeHelp            // keybinding help overlay
	modeConflict        // sync conflict resolution screen
	modeNotes           // built-in textarea notes editor
	modeFocus           // full-screen single-task focus (Pomodoro)
	modeSplash          // startup ambient splash (Phase 4b-B)
	modePalette         // command palette overlay
	modeAmbient         // :ambient settings screen with live preview
)

// inputKind identifies what the input bar is editing.
type inputKind int

const (
	inputSearch inputKind = iota
	inputAdd
	inputSubtask
	inputDue
	inputTags
	inputTitle
	inputIdea
)

type pickerKind int

const (
	pickerStatus pickerKind = iota
	pickerPriority
	pickerRating
)

type syncState int

const (
	syncPending  syncState = iota // nothing done yet this session
	syncSyncing                   // a sync is running
	syncOK                        // last sync succeeded
	syncOffline                   // last sync failed for a non-conflict reason
	syncConflict                  // paused on conflicts
)

type sidebarItem struct {
	Kind  string // "smart" | "view" | "project"
	Name  string // display name
	Key   string // filter payload: today|week|overdue | view name | project name
	Count int
	Done  int
	Total int
}

// flatRow is a tree-flattened, sorted, filtered task row.
type flatRow struct {
	Task  *core.Task
	Depth int
}

// Model is the root Bubble Tea model.
type Model struct {
	cfg   config.Config
	store *store.Store
	repo  sync.Repo
	root  string
	clock clock.Clock

	width  int
	height int

	mode  modeID
	view  viewID
	focus focusID

	// frame shares the frame cache across value copies of Model (the event
	// loop passes the model by value through every message). View() returns
	// the cached block verbatim while viewStamp (bumped by every message
	// that can change the screen) is unchanged. A tick that changes nothing
	// visible therefore costs a string compare in the renderer, never a
	// rebuild. Ambient ticks bump viewStamp like anything else.
	viewStamp       int64
	frame           *frameCache
	themeVersion    int64
	ambientStamp    int64 // bumped per ambient tick; scene-bearing panes key on it
	lastClockMinute int   // minute-of-hour at the last clock-driven redraw; -1 = unset
	panels          *panelCache
	lastInput       time.Time // wall-clock of the most recent key/mouse/focus event

	// Phase 4b-B ambient state
	ambientSettingSel int
	ambientT          time.Duration // accumulated scene clock
	ambientArmed      bool          // the ambient ticker is running
	lastAmbientTick   time.Time     // for frame delta math
	ambientFocused    bool          // terminal focus (tea.FocusMsg/BlurMsg)
	splashStart       time.Time     // splash entry time
	splashTotal       time.Duration // splash lifetime (ui.splash_seconds)
	ambientSceneName  string        // currently active scene
	scene             ambient.Scene
	scenePrev         ambient.Scene // crossfade source (nil when no fade is active)
	crossfadeStart    time.Time
	crossfadeDur      time.Duration
	phaseFadeStart    time.Time
	phaseFadeFrom     float64
	phaseFadeTo       float64
	phaseFadeActive   bool
	sceneName         string
	ambientSeed       int64
	rewardUntil       time.Time
	rewardTitle       string
	rewardFires       []time.Time
	rewardArmed       bool
	keymap            KeyMap
	styles            styles
	palette           theme.Palette
	themeMod          time.Time

	// panes
	sidebarItems  []sidebarItem
	sidebarSel    int
	helpSel       int
	sidebarHidden bool
	detailOn      bool
	detailScroll  int

	// filter state
	searchQuery string
	filter      core.Filter
	needle      string

	// home dashboard
	homeSel   int
	homeCache *homeData

	// list
	rows   []flatRow
	cursor int
	scroll int
	// rowLines maps a row index to its screen line within the list body
	// (group headers consume extra lines). Rebuilt by refreshTasks.
	rowLines   []int
	groupHeads []string // header label before row i ("" = none)

	// board
	boardCol       int
	boardRow       int
	boardDragging  bool
	boardDragFrom  int
	boardDragCol   int
	boardDragHover int
	boardDoneAll   bool // D toggles: show older completed tasks in the Done column

	// calendar
	calMonth     time.Time
	calDay       time.Time
	calAgendaSel int
	calShowDone  bool // `d` toggles completed tasks in day cells

	// input bar
	input      textinput.Model
	inputKind  inputKind
	inputLabel string

	// presentation
	icons      map[string]string
	animations bool

	// picker overlay
	picker      pickerKind
	pickerItems []string
	pickerSel   int
	pickerTitle string

	// confirm overlay
	confirmTitle string
	confirmBody  string
	confirmYes   func(m *Model) tea.Cmd

	// notes editor
	notes     textarea.Model
	notesID   string
	notesKind string // "task" (default) | "idea"

	// github link state (keyed by project name)
	github map[string]*actions.GitHubInfo

	// conflict screen
	conflicts      []string
	conflictSel    int
	conflictDone   map[string]bool
	conflictReturn modeID

	// active timer (state file outside the repo; see internal/actions/timer.go)
	timer core.TimerView

	// course dashboard scroll offset (lines of the detail pane)
	courseSel        int
	coursesDrillIn   string
	coursesDrillFrom int // course list index to restore on esc
	coursesScroll    int // detail-pane scroll (ctrl+d/ctrl+u, pgup/pgdn, wheel)

	// stats view (master-detail like courses; the detail pane scrolls)
	statsSel    int
	statsScroll int

	// ideas view (cards grouped by status)
	ideaSel    int
	ideaScroll int

	// project page (tabbed overlay, opened with enter on a sidebar project)
	projectPage   string
	projectReturn viewID
	projectTab    int
	projectSel    int // tasks row / milestone selection
	projectScroll int // body scroll inside the current tab (dev-log)
	roadmapPan    int // roadmap window shift in days (±84/+112)

	// command palette
	paletteQuery  string
	paletteSel    int
	paletteItems  []paletteItem
	paletteRecent []string
	paletteInput  textinput.Model

	// animations
	animFrame  int // 0 = idle; >0 = frames left in a pop/ease animation
	animTaskID string

	// sync
	pendingCommits []string
	debounceGen    int
	syncSt         syncState
	syncNote       string
	lastSync       time.Time
	hasRemote      bool
	spinner        spinner.Model

	statusMsg string

	pendingKey string
	pendingGen int

	glamourRenderer *glamour.TermRenderer
	glamourBody     string
	glamourBodyID   string
	glamourWidth    int
}

// New builds the model. store and repo must be ready; theme is polled live.
func New(cfg config.Config, s *store.Store, repo sync.Repo) Model {
	clk := s.Clock
	now := clk.Now()
	day := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	pal, err := theme.Load()
	if err != nil {
		pal = theme.Defaults()
	}
	input := textinput.New()
	input.Prompt = ""
	input.CharLimit = 512
	input.Focus()
	notes := textarea.New()
	notes.Placeholder = "Markdown notes…"
	notes.ShowLineNumbers = false
	notes.CharLimit = 1 << 20
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	m := Model{
		cfg:             cfg,
		store:           s,
		repo:            repo,
		root:            s.Root,
		clock:           clk,
		mode:            modeNormal,
		view:            viewHome,
		focus:           focusMain,
		keymap:          DefaultKeyMap(),
		styles:          newStyles(pal),
		palette:         pal,
		themeMod:        theme.PollModTime(theme.StateDir()),
		frame:           &frameCache{},
		themeVersion:    1,
		ambientStamp:    0,
		panels:          &panelCache{},
		lastInput:       clk.Now(),
		lastClockMinute: now.Minute(),
		detailOn:        true,
		input:           input,
		notes:           notes,
		pickerItems:     []string{},
		calMonth:        day,
		calDay:          time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
		conflictDone:    map[string]bool{},
		conflictReturn:  modeNormal,
		hasRemote:       repo.HasRemote(cfg.Sync.Remote),
		github:          map[string]*actions.GitHubInfo{},
		spinner:         sp,
		lastSync:        time.Time{},
		icons:           buildIcons(cfg.UI.Icons),
		animations:      cfg.UI.Animations,
		ambientFocused:  true,
		paletteInput:    textinput.New(),
	}
	if unknown := m.keymap.ApplyOverrides(cfg.Keys); len(unknown) > 0 {
		m.statusMsg = fmt.Sprintf("ignoring unknown keys: entries: %s", strings.Join(unknown, ", "))
	}
	m.timer = actions.LoadTimerView(s)
	m.sidebarItems = m.buildSidebar()
	m.sidebarSel = 0 // land on "All"
	m.refreshTasks()
	// Startup: if sync is paused, surface the conflict screen immediately.
	if st, err := repo.LoadState(); err == nil && st.Conflicted {
		m.startConflictMode(st.Files, "sync is paused — conflicts need resolution")
	} else {
		m.syncSt = syncPending
	}
	return m
}

// clampScrolls keeps scroll offsets within bounds after a resize/toggle.
func (m *Model) clampScrolls() {
	if m.detailScroll < 0 {
		m.detailScroll = 0
	}
	m.ensureCursorVisible()
}
