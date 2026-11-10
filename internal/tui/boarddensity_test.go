package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

// densityPair builds one board fixture and returns models pinned to the
// given densities (sharing the store).
func densityPair(t *testing.T, withTasks []*core.Task, densities ...string) []Model {
	t.Helper()
	m, _ := newTestModel(t, withTasks)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	m.view = viewBoard
	m.resetBoardCursor()
	out := make([]Model, 0, len(densities))
	for _, d := range densities {
		dm := m
		dm.cfg.UI.Density = d
		out = append(out, dm)
	}
	return out
}

func firstTask(t *testing.T, m Model) *core.Task {
	t.Helper()
	cols := boardColumns(m)
	if len(cols) == 0 || len(cols[0]) == 0 {
		t.Fatal("fixture has no board cards")
	}
	return cols[0][0]
}

// Spec 0c.3: compact density removes the blank line between a card's
// title and its badge row — a one-line-title card is 4 rows instead of 5,
// and renderCard and cardLineCount (used by hit-testing) agree.
func TestBoardCompactRenderCardFourRows(t *testing.T) {
	tasks := []*core.Task{
		{Title: "short title", Status: "todo", Priority: "high", Kind: core.KindTask},
	}
	models := densityPair(t, tasks, "comfortable", "compact")
	mComf, mComp := models[0], models[1]
	task := firstTask(t, mComf)

	head := subtaskCounts(mComf.store.Tasks(), func(p, st string) bool { return mComf.store.IsDoneStatus(p, st) })
	now := mComf.clock.Now()
	comfortable := renderCard(mComf, task, 22, false, now, head)
	compact := renderCard(mComp, task, 22, false, now, head)
	if len(comfortable) != 5 {
		t.Errorf("comfortable card = %d rows, want 5", len(comfortable))
	}
	if len(compact) != 4 {
		t.Errorf("compact card = %d rows, want 4 (title/badge gap removed)", len(compact))
	}
	if got := cardLineCount(mComf, task, 22, now, head); got != len(comfortable) {
		t.Errorf("comfortable: cardLineCount = %d, renderCard = %d rows", got, len(comfortable))
	}
	if got := cardLineCount(mComp, task, 22, now, head); got != len(compact) {
		t.Errorf("compact: cardLineCount = %d, renderCard = %d rows", got, len(compact))
	}
}

// Hit-testing must follow the compact card heights: a click on the 4th
// row of a compact card (its badge row) selects the card.
func TestBoardCompactHitTestFollowsCardHeights(t *testing.T) {
	tasks := []*core.Task{
		{Title: "short title", Status: "todo", Priority: "high", Kind: core.KindTask},
	}
	m := densityPair(t, tasks, "compact")[0]

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

	slots := boardSlotHeights(m, boardColumns(m), innerW, m.height-4)
	if len(slots) == 0 {
		t.Fatal("no slots measured for the compact board")
	}
	if got := slots[0][1]; got != 4 {
		t.Fatalf("slot height = %d, want 4 (compact)", got)
	}
	updated, _ := m.Update(tea.MouseMsg{X: sidebarW + 3, Y: 1 + slots[0][0] + 3, Type: tea.MouseLeft})
	m = updated.(Model)
	if m.boardCol != 0 || m.boardRow != 0 {
		t.Fatalf("click on the compact badge row: boardCol=%d boardRow=%d, want 0/0", m.boardCol, m.boardRow)
	}
}

// A wrapped badge row rounds the compact card up, and the slot measurement
// follows the wrapped height.
func TestBoardCompactWrappedMetaRoundsSlot(t *testing.T) {
	parent := fixtureTask{id: "p1", title: "parent card", project: "inbox", status: "todo", priority: "high", kind: "task",
		tags: []string{"verylongtagname"}}
	sub := fixtureTask{id: "p1s", title: "child", project: "inbox", status: "todo", priority: "low", kind: "task", parent: "p1"}
	s := fixtureStore(t, []fixtureTask{parent, sub})
	defer func() { _ = s.Close() }()
	m := New(dumpConfig(), s, sync.New(s.Root))
	m.view = viewBoard
	m.width, m.height = 120, 40
	m.resetBoardCursor()
	m.cfg.UI.Density = "compact"
	task := s.Tasks()[0] // p1
	head := subtaskCounts(s.Tasks(), func(p, st string) bool { return s.IsDoneStatus(p, st) })
	now := m.clock.Now()
	if cardMetaFits(m, task, 22, now, head) {
		t.Skip("meta fits in this configuration; adjust fixture width")
	}
	if got := cardLineCount(m, task, 22, now, head); got != 5 {
		t.Fatalf("cardLineCount wrapped = %d, want 5", got)
	}
}

// Spec 0c.3: goldens for compact density (board view, 120x40).
// Regenerate with: MTM_UPDATE_GOLDEN=1 go test ./internal/tui/ -run TestDumpGoldenCompact
func TestDumpGoldenCompact(t *testing.T) {
	states := []string{"empty", "populated", "manyTasks", "boardColumns"}
	update := os.Getenv("MTM_UPDATE_GOLDEN") == "1"
	for _, state := range states {
		t.Run(state, func(t *testing.T) {
			golden := filepath.Join("testdata", "dump", "board-120x40-compact-"+state+".golden")
			s := fixtures[state](t) // pins HOME/XDG to temp dirs
			cfg := dumpConfig()
			cfg.UI.Density = "compact"
			frame, err := RenderDump(cfg, s, sync.New(s.Root), "board", 120, 40, true, DumpFixedTime)
			if err != nil {
				t.Fatal(err)
			}
			if update {
				if err := os.WriteFile(golden, []byte(frame), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("missing golden %s (run MTM_UPDATE_GOLDEN=1)", golden)
			}
			if frame != string(want) {
				t.Fatalf("dump mismatch in %s\n--- got ---\n%s\n--- want ---\n%s", golden, frame, want)
			}
		})
	}
}
