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

func TestNewModelLandsOnAllView(t *testing.T) {
	m, _ := newTestModel(t, []*core.Task{
		{Title: "alpha", Priority: "high", Kind: core.KindTask, Status: "todo"},
		{Title: "beta", Priority: "low", Kind: core.KindTask, Status: "todo"},
	})
	if m.sidebarSel != 0 {
		t.Fatalf("sidebarSel = %d, want 0 (All)", m.sidebarSel)
	}
	if sel := m.sidebarSelection(); sel == nil || sel.Key != "all" {
		t.Fatalf("default sidebar selection = %+v", sel)
	}
	if len(m.rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(m.rows))
	}
	if !strings.Contains(m.View(), "List") {
		t.Fatal("footer missing view tabs")
	}
}

func TestSortOrderListRows(t *testing.T) {
	m, _ := newTestModel(t, []*core.Task{
		{Title: "done urgent", Status: "done", Priority: "urgent", Kind: core.KindTask},
		{Title: "open low", Status: "todo", Priority: "low", Kind: core.KindTask},
		{Title: "open high", Status: "todo", Priority: "high", Kind: core.KindTask},
	})
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
	m = press(t, m, "j", "j")
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
	m, s := newTestModel(t, []*core.Task{
		{Title: "via command", Status: "todo", Priority: "low", Kind: core.KindTask},
	})
	m = press(t, m, ":")
	if m.mode != modeInput || m.inputKind != inputCommand {
		t.Fatalf("command mode not entered: %v", m.mode)
	}
	m = typeInto(t, m, "done")
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
	m = press(t, m, "j") // All -> Today
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
		t.Fatal()
	}
	m = press(t, m, "tab")
	if m.view != viewList {
		t.Fatal()
	}
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
