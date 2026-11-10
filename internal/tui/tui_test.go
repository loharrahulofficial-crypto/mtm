package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/config"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/store"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

func newTestModel(t *testing.T, withTasks []*core.Task) (Model, *store.Store) {
	t.Helper()
	fixedEnv(t)
	root := t.TempDir()
	cache := t.TempDir() + "/index.json"
	s, err := store.Open(root, cache)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveProject(core.DefaultProject("inbox")); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.DataRepo = root
	cfg.Sync.Auto = false
	m := New(cfg, s, sync.New(root))
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 130, Height: 40})
	m = updated.(Model)
	for _, task := range withTasks {
		if task.Project == "" {
			task.Project = "inbox"
		}
		if err := s.Create(task); err != nil {
			t.Fatal(err)
		}
	}
	m.refreshTasks()
	m.sidebarItems = m.buildSidebar()
	return m, s
}

func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEscape}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "ctrl+d":
		return tea.KeyMsg{Type: tea.KeyCtrlD}
	case "ctrl+u":
		return tea.KeyMsg{Type: tea.KeyCtrlU}
	case "ctrl+h":
		return tea.KeyMsg{Type: tea.KeyCtrlH}
	case "ctrl+l":
		return tea.KeyMsg{Type: tea.KeyCtrlL}
	case "ctrl+s":
		return tea.KeyMsg{Type: tea.KeyCtrlS}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case "ctrl+x":
		return tea.KeyMsg{Type: tea.KeyCtrlX}
	case "ctrl+b":
		return tea.KeyMsg{Type: tea.KeyCtrlB}
	}
	r := []rune(s)
	if len(r) == 1 {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: r}
	}
	// multi-rune (like "??" edge cases) — not used by bindings
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: r}
}

func press(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, k := range keys {
		updated, _ := m.Update(keyMsg(k))
		m = updated.(Model)
	}
	return m
}

func pressCmd(t *testing.T, m Model, k string) (Model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(keyMsg(k))
	return updated.(Model), cmd
}

func typeInto(t *testing.T, m Model, text string) Model {
	t.Helper()
	for _, r := range text {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	updated, _ := m.Update(keyMsg("enter"))
	return updated.(Model)
}

func taskTitles(m Model) []string {
	out := make([]string, len(m.rows))
	for i, r := range m.rows {
		out[i] = r.Task.Title
	}
	return out
}

func findTaskInStore(s *store.Store, title string) *core.Task {
	for _, t := range s.Tasks() {
		if t.Title == title {
			return t
		}
	}
	return nil
}

func TestNewModelLandsOnHomeView(t *testing.T) {
	m, _ := newTestModel(t, []*core.Task{
		{Title: "alpha", Priority: "high", Kind: core.KindTask, Status: "todo"},
		{Title: "beta", Priority: "low", Kind: core.KindTask, Status: "todo"},
	})
	if m.view != viewHome {
		t.Fatalf("default view = %v, want Home", m.view)
	}
	if sel := m.sidebarSelection(); sel == nil || sel.Key != "home" {
		t.Fatalf("default sidebar selection = %+v", sel)
	}
	if len(m.rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(m.rows))
	}
	if !strings.Contains(m.View(), "Home") {
		t.Fatal("footer missing Home pill")
	}
}

// selectSidebarKey moves the sidebar selection to kind/key and rebuilds rows.
func selectSidebarKey(m Model, kind, key string) Model {
	for i, it := range m.sidebarItems {
		if it.Kind == kind && it.Key == key {
			m.sidebarSel = i
			break
		}
	}
	m.refreshTasks()
	return m
}

func TestSortOrderListRows(t *testing.T) {
	m, _ := newTestModel(t, []*core.Task{
		{Title: "done urgent", Status: "done", Priority: "urgent", Kind: core.KindTask},
		{Title: "open low", Status: "todo", Priority: "low", Kind: core.KindTask},
		{Title: "open high", Status: "todo", Priority: "high", Kind: core.KindTask},
	})
	m = selectSidebarKey(m, "smart", "all")
	got := strings.Join(taskTitles(m), ",")
	want := "open high,open low,done urgent"
	if got != want {
		t.Fatalf("order = %s, want %s", got, want)
	}
}

func TestCursorAndGG(t *testing.T) {
	m, _ := newTestModel(t, []*core.Task{
		{Title: "one", Status: "todo", Priority: "medium", Kind: core.KindTask},
		{Title: "two", Status: "todo", Priority: "medium", Kind: core.KindTask},
		{Title: "three", Status: "todo", Priority: "medium", Kind: core.KindTask},
	})
	m = press(t, m, "1", "j", "j")
	if m.cursor != 2 {
		t.Fatalf("cursor = %d after jj", m.cursor)
	}
	m = press(t, m, "g", "g")
	if m.cursor != 0 {
		t.Fatalf("cursor = %d after gg", m.cursor)
	}
	m = press(t, m, "G")
	if m.cursor != 2 {
		t.Fatalf("cursor = %d after G", m.cursor)
	}
	m = press(t, m, "k")
	if m.cursor != 1 {
		t.Fatalf("cursor = %d after k", m.cursor)
	}
}

func TestAddTaskViaInputBar(t *testing.T) {
	m, s := newTestModel(t, nil)
	m = press(t, m, "a")
	if m.mode != modeInput || m.inputKind != inputAdd {
		t.Fatalf("mode = %v inputKind = %v", m.mode, m.inputKind)
	}
	m = typeInto(t, m, "write dbms report friday !high")
	if m.mode != modeNormal {
		t.Fatalf("mode after submit = %v", m.mode)
	}
	ft := findTaskInStore(s, "write dbms report")
	if ft == nil {
		t.Fatal("task not created")
	}
	if ft.Priority != "high" {
		t.Fatalf("priority = %q", ft.Priority)
	}
	if ft.Status != "todo" {
		t.Fatalf("status = %q", ft.Status)
	}
	if len(m.pendingCommits) != 1 || !strings.HasPrefix(m.pendingCommits[0], "add: ") {
		t.Fatalf("pendingCommits = %v", m.pendingCommits)
	}
}

func TestToggleDoneRecurringSpawnsNext(t *testing.T) {
	now := time.Now()
	// A recurring task due in the future.
	next := now.AddDate(0, 0, 7).Format("2006-01-02")
	m, s := newTestModel(t, []*core.Task{
		{Title: "gym", Status: "todo", Priority: "medium", Kind: core.KindTask, Recurrence: "weekly", Due: next},
	})
	m = press(t, m, "x")
	gym := findTaskInStore(s, "gym")
	_ = gym
	count := 0
	for _, task := range s.Tasks() {
		if task.Title == "gym" {
			count++
			if task.Status == "done" {
				if task.Completed == "" {
					t.Fatal("completed timestamp not set")
				}
			}
		}
	}
	if count != 2 {
		t.Fatalf("expected original + spawned occurrence, got %d gym tasks", count)
	}
	// spawned occurrence must be open
	open := 0
	for _, task := range s.Tasks() {
		if task.Title == "gym" && task.Status != "done" {
			open++
		}
	}
	if open != 1 {
		t.Fatalf("expected 1 open spawned occurrence, got %d", open)
	}
}

func TestDeleteConfirmFlow(t *testing.T) {
	m, s := newTestModel(t, []*core.Task{
		{Title: "doomed", Status: "todo", Priority: "low", Kind: core.KindTask},
	})
	m = press(t, m, "d", "d")
	if m.mode != modeConfirm {
		t.Fatalf("mode = %v, want confirm", m.mode)
	}
	// Cancel keeps the task.
	m = press(t, m, "n")
	if findTaskInStore(s, "doomed") == nil {
		t.Fatal("task deleted on cancel")
	}
	// Delete for real.
	m = press(t, m, "d", "d", "y")
	if findTaskInStore(s, "doomed") != nil {
		t.Fatal("task not deleted")
	}
}

func TestSearchFiltersLiveAndClears(t *testing.T) {
	m, _ := newTestModel(t, []*core.Task{
		{Title: "buy milk", Status: "todo", Priority: "low", Kind: core.KindTask},
		{Title: "write report", Status: "todo", Priority: "low", Kind: core.KindTask},
	})
	m = press(t, m, "/")
	if m.mode != modeInput || m.inputKind != inputSearch {
		t.Fatalf("search not opened: mode=%v kind=%v", m.mode, m.inputKind)
	}
	for _, r := range "milk" {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	if len(m.rows) != 1 || m.rows[0].Task.Title != "buy milk" {
		t.Fatalf("rows = %v", taskTitles(m))
	}
	m = press(t, m, "esc")
	if m.searchQuery != "" || len(m.rows) != 2 {
		t.Fatalf("search not cleared: query=%q rows=%d", m.searchQuery, len(m.rows))
	}
}

func TestSearchTagSyntax(t *testing.T) {
	m, _ := newTestModel(t, []*core.Task{
		{Title: "tax return", Status: "todo", Priority: "low", Kind: core.KindTask, Tags: []string{"finance"}},
		{Title: "buy milk", Status: "todo", Priority: "low", Kind: core.KindTask},
	})
	m = press(t, m, "/")
	for _, r := range "tag:finance" {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	if len(m.rows) != 1 || m.rows[0].Task.Title != "tax return" {
		t.Fatalf("tag: filter failed: %v", taskTitles(m))
	}
}

func TestBoardMoveCardWithL(t *testing.T) {
	m, s := newTestModel(t, []*core.Task{
		{Title: "card task", Status: "todo", Priority: "high", Kind: core.KindTask},
	})
	m = press(t, m, "2")
	if m.view != viewBoard {
		t.Fatal("not in board view")
	}
	m = press(t, m, "L") // move card right: todo -> doing
	if ft := findTaskInStore(s, "card task"); ft == nil || ft.Status != "doing" {
		t.Fatalf("status after L = %+v", ft)
	}
	m = press(t, m, "L") // doing -> done
	if ft := findTaskInStore(s, "card task"); ft == nil || ft.Status != "done" {
		t.Fatalf("status after LL = %+v", ft)
	}
	m = press(t, m, "L") // already at last column: stays done
	if ft := findTaskInStore(s, "card task"); ft.Status != "done" {
		t.Fatalf("status after LLL = %q", ft.Status)
	}
	if len(m.pendingCommits) == 0 {
		t.Fatal("board move not queued for commit")
	}
}

func TestArchiveDoneTaskSelected(t *testing.T) {
	m, s := newTestModel(t, []*core.Task{
		{Title: "old news", Status: "done", Priority: "low", Kind: core.KindTask},
	})
	m = selectSidebarKey(m, "smart", "all")
	m = press(t, m, "X")
	if findTaskInStore(s, "old news") != nil {
		t.Fatal("done task not archived")
	}
	// file moved under archive/
	matches, err := filepath.Glob(filepath.Join(s.Root, "archive", "*", "*old-news*.md"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("archived file missing: %v %v", matches, err)
	}
}

func TestArchiveOpenTaskRefused(t *testing.T) {
	m, s := newTestModel(t, []*core.Task{
		{Title: "still open", Status: "todo", Priority: "low", Kind: core.KindTask},
	})
	m = press(t, m, "X")
	if !strings.Contains(m.statusMsg, "only done") {
		t.Fatalf("statusMsg = %q", m.statusMsg)
	}
	if findTaskInStore(s, "still open") == nil {
		t.Fatal("open task wrongly archived")
	}
}

func TestCommandLineDone(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	m, s := newTestModel(t, []*core.Task{
		{Title: "via command", Status: "todo", Priority: "low", Kind: core.KindTask, Due: today},
	})
	m = press(t, m, ":")
	if m.mode != modePalette {
		t.Fatalf("palette mode not entered: %v", m.mode)
	}
	m = typeInto(t, m, "done")
	m = press(t, m, "enter")
	if ft := findTaskInStore(s, "via command"); ft == nil || ft.Status != "done" {
		t.Fatalf("command did not mark done: %+v", ft)
	}
}

func TestSidebarFocusAndProjectFilter(t *testing.T) {
	m, _ := newTestModel(t, []*core.Task{
		{Title: "work item", Status: "todo", Priority: "low", Kind: core.KindTask},
	})
	// Focus sidebar with ctrl+h, move to overdue-free "Today", then back.
	m = press(t, m, "ctrl+h")
	if m.focus != focusSidebar {
		t.Fatalf("focus = %v", m.focus)
	}
	m = press(t, m, "j", "j") // Home -> All -> Today
	if sel := m.sidebarSelection(); sel == nil || sel.Key != "today" {
		t.Fatalf("selection = %+v", sel)
	}
	if len(m.rows) != 0 {
		t.Fatalf("today filter should hide unrelated tasks, got %v", taskTitles(m))
	}
	m = press(t, m, "k") // Today -> All
	if len(m.rows) != 1 {
		t.Fatal("backslash to All did not restore tasks")
	}
	m = press(t, m, "ctrl+l")
	if m.focus != focusMain {
		t.Fatalf("focus = %v, want main", m.focus)
	}
}

func TestKeymapConfigOverride(t *testing.T) {
	km := DefaultKeyMap()
	unknown := km.ApplyOverrides(map[string]string{"quit": "Q", "bogus_action": "z"})
	if len(unknown) != 1 || unknown[0] != "bogus_action" {
		t.Fatalf("unknown = %v", unknown)
	}
	if km.Seq(aQuit) != "Q" {
		t.Fatalf("quit binding = %q", km.Seq(aQuit))
	}
}

func TestDueInputRejectedStaysOpen(t *testing.T) {
	m, _ := newTestModel(t, []*core.Task{
		{Title: "due test", Status: "todo", Priority: "low", Kind: core.KindTask},
	})
	m = press(t, m, "D")
	if m.mode != modeInput || m.inputKind != inputDue {
		t.Fatalf("due input not opened: %v", m.mode)
	}
	m = typeInto(t, m, "notadate")
	if m.mode != modeInput {
		t.Fatal("invalid due closed the input — should stay open with an error")
	}
	if !strings.Contains(m.statusMsg, "invalid") {
		t.Fatalf("statusMsg = %q", m.statusMsg)
	}
}

func TestViewSwitchingAndTab(t *testing.T) {
	m, _ := newTestModel(t, nil)
	m = press(t, m, "2")
	if m.view != viewBoard {
		t.Fatal()
	}
	m = press(t, m, "tab")
	if m.view != viewCalendar {
		t.Fatal("tab 1 -> want Calendar")
	}
	m = press(t, m, "tab")
	if m.view != viewCourses {
		t.Fatal("tab 2 -> want Courses")
	}
	m = press(t, m, "tab")
	if m.view != viewStats {
		t.Fatal("tab 3 -> want Stats")
	}
	m = press(t, m, "tab")
	if m.view != viewIdeas {
		t.Fatal("tab 4 -> want Ideas")
	}
	m = press(t, m, "tab")
	if m.view != viewHome {
		t.Fatal("tab 5 -> want Home")
	}
	m = press(t, m, "tab")
	if m.view != viewList {
		t.Fatal("tab 6 -> want List")
	}
	// Direct jump with its key.
	m = press(t, m, "4")
	if m.view != viewCourses {
		t.Fatal("4 -> want Courses")
	}
	// Back to Home for a stable end state.
	m = press(t, m, "0")
}

func TestNotesSaveViaEditor(t *testing.T) {
	m, s := newTestModel(t, []*core.Task{
		{Title: "note me", Status: "todo", Priority: "low", Kind: core.KindTask, Body: "old body"},
	})
	m = press(t, m, "m")
	if m.mode != modeNotes {
		t.Fatalf("notes editor not opened: %v", m.mode)
	}
	m.notes.SetValue("new body\nwith lines")
	updated, _ := m.Update(keyMsg("ctrl+s"))
	m = updated.(Model)
	if m.mode != modeNormal {
		t.Fatal("notes editor did not close")
	}
	if ft := findTaskInStore(s, "note me"); ft == nil || ft.Body != "new body\nwith lines" {
		t.Fatalf("body = %q", ft.Body)
	}
}

func TestBoardMouseDragMovesCard(t *testing.T) {
	m, s := newTestModel(t, []*core.Task{
		{Title: "drag me", Status: "todo", Priority: "high", Kind: core.KindTask},
	})
	m = press(t, m, "2")
	// Press on the card at (x=60, y=4) — main pane starts after the sidebar.
	sidebarW := sideWidth(m.width, m.sidebarHidden)
	x := sidebarW + 8
	updated, _ := m.Update(tea.MouseMsg{X: x, Y: 4, Type: tea.MouseLeft})
	m = updated.(Model)
	if !m.boardDragging {
		t.Fatal("drag did not start on card press")
	}
	// Drag into the second column (stride = colW + 1 separator).
	detailW := detailWidth(m.width, m.detailOn)
	mainW := m.width - sidebarW - detailW - 1
	innerW := mainW - 2
	_, visCols, colW := boardLayout(m, innerW)
	if visCols < 2 {
		t.Skip("not enough columns visible for drag target")
	}
	releaseX := sidebarW + 1 + 2 + 1 + colW + 1 + 5 // gutter + body inset + cardW + gap
	updated, _ = m.Update(tea.MouseMsg{X: releaseX, Y: 4, Type: tea.MouseRelease})
	m = updated.(Model)
	if ft := findTaskInStore(s, "drag me"); ft == nil || ft.Status != "doing" {
		t.Fatalf("card status after drag = %+v", ft)
	}
}

func TestMouseWheelMovesSelection(t *testing.T) {
	m, _ := newTestModel(t, []*core.Task{
		{Title: "one", Status: "todo", Priority: "low", Kind: core.KindTask},
		{Title: "two", Status: "todo", Priority: "low", Kind: core.KindTask},
		{Title: "three", Status: "todo", Priority: "low", Kind: core.KindTask},
	})
	updated, _ := m.Update(tea.MouseMsg{X: 40, Y: 5, Type: tea.MouseWheelDown})
	m = updated.(Model)
	if m.cursor != 2 {
		t.Fatalf("cursor after wheel down = %d, want 2 (clamped to last row)", m.cursor)
	}
}

func seedTestIdea(t *testing.T, s *store.Store, title, status string) *core.Idea {
	t.Helper()
	i := &core.Idea{Title: title, Status: status}
	if err := s.CreateIdea(i); err != nil {
		t.Fatal(err)
	}
	return i
}

func TestIdeasViewKeyFlows(t *testing.T) {
	m, s := newTestModel(t, nil)
	i := seedTestIdea(t, s, "sync brainstorm", core.IdeaSeed)

	m = press(t, m, "6")
	if m.view != viewIdeas {
		t.Fatalf("view = %v, want ideas", m.view)
	}
	// s cycles the status seed -> exploring.
	m = press(t, m, "s")
	got, err := s.ResolveIdea(i.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != core.IdeaExploring {
		t.Fatalf("status after s = %q, want exploring", got.Status)
	}

	// r opens the rating picker; enter accepts the highlighted value.
	m = press(t, m, "r")
	if m.mode != modePicker || m.picker != pickerRating {
		t.Fatalf("after r: mode %v picker %v, want rating picker", m.mode, m.picker)
	}
	m = press(t, m, "enter")
	got, err = s.ResolveIdea(i.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Rating != 1 {
		t.Fatalf("rating after picker = %d, want 1", got.Rating)
	}

	// a opens the input bar wired to capture an idea.
	m = press(t, m, "a")
	if m.mode != modeInput || m.inputKind != inputIdea {
		t.Fatalf("after a: mode %v kind %v, want idea input", m.mode, m.inputKind)
	}
	m = typeInto(t, m, "captured in the tui")
	found := false
	for _, ii := range s.Ideas() {
		if ii.Title == "captured in the tui" && ii.Status == core.IdeaSeed {
			found = true
		}
	}
	if !found {
		t.Fatal("typed idea not captured")
	}

	// m opens the built-in notes editor bound to the idea; ctrl+s saves.
	m = press(t, m, "m")
	if m.mode != modeNotes || m.notesKind != "idea" {
		t.Fatalf("after m: mode %v kind %v, want idea notes", m.mode, m.notesKind)
	}
	m = typeInto(t, m, "compare CRDT vs oplog")
	// typeInto ends with enter; close with ctrl+s.
	m = press(t, m, "ctrl+s")
	captured := ""
	for _, ii := range s.Ideas() {
		if ii.Title == "captured in the tui" {
			captured = ii.Body
		}
	}
	if !strings.Contains(captured, "compare CRDT vs oplog") {
		t.Fatalf("captured idea body missing typed notes:\n%s", captured)
	}
}

func TestIdeasPromoteFromTUI(t *testing.T) {
	m, s := newTestModel(t, nil)
	i := seedTestIdea(t, s, "status page generator", core.IdeaSeed)
	m = press(t, m, "6")
	m = press(t, m, "p")
	if m.mode != modeConfirm {
		t.Fatalf("after p: mode %v, want confirm", m.mode)
	}
	m = press(t, m, "y")
	got, err := s.ResolveIdea(i.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != core.IdeaPromoted {
		t.Fatalf("status after promote = %q, want promoted", got.Status)
	}
	if !s.ProjectExists(got.PromotedTo) {
		t.Fatalf("promoted project %q missing", got.PromotedTo)
	}
	if ms := s.Milestones(got.PromotedTo); len(ms) != 1 {
		t.Fatalf("milestones on promoted project = %d, want 1", len(ms))
	}
}

func TestProjectPageOpensAndTabs(t *testing.T) {
	m, s := newTestModel(t, []*core.Task{
		{Title: "scoped task", Project: "inbox", Status: "todo", Priority: "high", Kind: core.KindTask},
	})
	if err := s.SaveProject(core.DefaultProject("inbox")); err != nil {
		t.Fatal(err)
	}
	m.sidebarItems = m.buildSidebar()
	// Jump to the project entry in the sidebar (find it), then enter.
	for i, it := range m.sidebarItems {
		if it.Kind == "project" && it.Key == "inbox" {
			m.sidebarSel = i
			break
		}
	}
	m.focus = focusSidebar
	m = press(t, m, "enter")
	if m.projectPage != "inbox" {
		t.Fatalf("projectPage = %q, want inbox", m.projectPage)
	}
	if m.projectTab != projectTabOverview {
		t.Fatalf("tab = %v, want overview", m.projectTab)
	}
	// [ ] tab cycles through all four tabs.
	for _, step := range []struct {
		tab int
		key string
	}{
		{projectTabTasks, "]"},
		{projectTabRoadmap, "]"},
		{projectTabDevlog, "]"},
		{projectTabOverview, "]"},
	} {
		m = press(t, m, step.key)
		if m.projectTab != step.tab {
			t.Fatalf("after %q tab = %v, want %v", step.key, m.projectTab, step.tab)
		}
	}
	// esc closes the page.
	m = press(t, m, "esc")
	if m.projectPage != "" {
		t.Fatalf("after esc projectPage = %q, want closed", m.projectPage)
	}
}

func TestRoadmapKeyPansWindow(t *testing.T) {
	m, _ := newTestModel(t, nil)
	m.projectPage = "inbox"
	m.projectTab = projectTabRoadmap
	m.width, m.height = 120, 40
	before := m.roadmapPan
	m = press(t, m, "l")
	if m.roadmapPan != before+28 {
		t.Fatalf("roadmapPan after l = %d, want %d", m.roadmapPan, before+28)
	}
	m = press(t, m, "h")
	m = press(t, m, "h")
	if m.roadmapPan != before-28 {
		t.Fatalf("roadmapPan after h,h = %d, want %d", m.roadmapPan, before-28)
	}
}
