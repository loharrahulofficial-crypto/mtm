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

	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

type viewID int

const (
	viewList viewID = iota
	viewBoard
	viewCalendar
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
	inputCommand
)

type pickerKind int

const (
	pickerStatus pickerKind = iota
	pickerPriority
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

	width  int
	height int

	mode     modeID
	view     viewID
	focus    focusID
	keymap   KeyMap
	styles   styles
	palette  theme.Palette
	themeMod time.Time

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

	// list
	rows   []flatRow
	cursor int
	scroll int

	// board
	boardCol      int
	boardRow      int
	boardDragging bool
	boardDragFrom int
	boardDragCol  int

	// calendar
	calMonth     time.Time
	calDay       time.Time
	calAgendaSel int

	// input bar
	input      textinput.Model
	inputKind  inputKind
	inputLabel string

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
	notes   textarea.Model
	notesID string

	// conflict screen
	conflicts      []string
	conflictSel    int
	conflictDone   map[string]bool
	conflictReturn modeID

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
	now := time.Now()
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
		cfg:            cfg,
		store:          s,
		repo:           repo,
		root:           s.Root,
		mode:           modeNormal,
		view:           viewList,
		focus:          focusMain,
		keymap:         DefaultKeyMap(),
		styles:         newStyles(pal),
		palette:        pal,
		themeMod:       theme.PollModTime(theme.StateDir()),
		detailOn:       true,
		input:          input,
		notes:          notes,
		pickerItems:    []string{},
		calMonth:       day,
		calDay:         time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
		conflictDone:   map[string]bool{},
		conflictReturn: modeNormal,
		hasRemote:      repo.HasRemote(cfg.Sync.Remote),
		spinner:        sp,
		lastSync:       time.Time{},
	}
	if unknown := m.keymap.ApplyOverrides(cfg.Keys); len(unknown) > 0 {
		m.statusMsg = fmt.Sprintf("ignoring unknown keys: entries: %s", strings.Join(unknown, ", "))
	}
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
