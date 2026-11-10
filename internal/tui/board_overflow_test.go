package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

func boardOverflowModel(t *testing.T, width, height int) (Model, *core.Task) {
	t.Helper()
	tasks := []*core.Task{
		{Title: "overflow card", Status: "todo", Priority: "high", Kind: core.KindTask},
		{Title: "overflow doing", Status: "doing", Priority: "medium", Kind: core.KindTask},
		{Title: "overflow done", Status: "done", Priority: "low", Kind: core.KindTask},
	}
	m, s := newTestModel(t, tasks)
	m.view = viewBoard
	updated, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	m = updated.(Model)
	m.resetBoardCursor()
	_ = s
	return m, tasks[0]
}

func boardHeader(frame string) string {
	for _, l := range strings.Split(frame, "\n") {
		if strings.Contains(l, "TODO ·") || strings.Contains(l, "DOING ·") || strings.Contains(l, "DONE ·") {
			return l
		}
	}
	return ""
}

// Spec 0b.2: when the columns do not all fit, the header shows ‹/›
// indicators and h/l scrolls the window into view. The overflow depends
// on the detail pane: at 120x40 with the detail open only two columns
// fit, hiding DONE; at 80x24 the same happens even with the detail gone.
func TestBoardOverflowIndicatorsAndHLScroll(t *testing.T) {
	for _, tc := range []struct {
		name string
		w, h int
	}{
		{"120x40", 120, 40},
		{"80x24", 80, 24},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := boardOverflowModel(t, tc.w, tc.h)
			frame := StripANSI(m.View())
			hdr := boardHeader(frame)
			right := m.icon("chev_right")
			if !strings.Contains(hdr, right) {
				t.Fatalf("no › indicator in the header at %dx%d — DONE is hidden but not hinted:\n%s", tc.w, tc.h, hdr)
			}
			if strings.Contains(hdr, "DONE") {
				t.Fatalf("DONE column fits at %dx%d — fixture expected it to hide:\n%s", tc.w, tc.h, hdr)
			}
			// Two l presses reach the hidden DONE column.
			updated, _ := m.Update(keyMsg("l"))
			m = updated.(Model)
			updated, _ = m.Update(keyMsg("l"))
			m = updated.(Model)
			if m.boardCol != 2 {
				t.Fatalf("boardCol = %d after l l, want 2 (done)", m.boardCol)
			}
			frame = StripANSI(m.View())
			hdr = boardHeader(frame)
			left := m.icon("chev_left")
			if !strings.Contains(hdr, left) {
				t.Fatalf("no ‹ indicator in the header at %dx%d after scrolling right:\n%s", tc.w, tc.h, hdr)
			}
			if !strings.Contains(hdr, "DONE") {
				t.Fatalf("DONE column is not visible after l l at %dx%d:\n%s", tc.w, tc.h, hdr)
			}
			// h h comes back.
			updated, _ = m.Update(keyMsg("h"))
			m = updated.(Model)
			updated, _ = m.Update(keyMsg("h"))
			m = updated.(Model)
			if m.boardCol != 0 {
				t.Fatalf("boardCol = %d after h h, want 0", m.boardCol)
			}
		})
	}
}

// The mouse scrolls columns too: clicking the right header chevron shifts
// the window right; clicking the left chevron shifts it back; wheel left/
// right move the selection one column as well.
func TestBoardOverflowMouseScroll(t *testing.T) {
	m, _ := boardOverflowModel(t, 80, 24)

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

	click := func(x, y int) Model {
		msg := tea.MouseMsg{X: x, Y: y, Type: tea.MouseLeft}
		updated, _ := m.Update(msg)
		return updated.(Model)
	}

	// Click the › chevron (header row, right inset cell).
	m = click(sidebarW+1+innerW, 1)
	if m.boardCol != 1 {
		t.Fatalf("boardCol = %d after clicking ›, want 1", m.boardCol)
	}
	m = click(sidebarW+1+innerW, 1)
	if m.boardCol != 2 {
		t.Fatalf("boardCol = %d after second › click, want 2 (done visible)", m.boardCol)
	}
	frame := StripANSI(m.View())
	if !strings.Contains(boardHeader(frame), "DONE") {
		t.Fatalf("DONE column not visible after mouse scroll:\n%s", boardHeader(frame))
	}
	// Click the ‹ chevron (left inset cell).
	m = click(sidebarW+1+1, 1)
	if m.boardCol != 1 {
		t.Fatalf("boardCol = %d after clicking ‹, want 1", m.boardCol)
	}
	// Wheel right/left on any board row moves the column selection.
	wheel := func(typ tea.MouseEventType) Model {
		updated, _ := m.Update(tea.MouseMsg{X: sidebarW + 10, Y: 6, Type: typ})
		return updated.(Model)
	}
	m = wheel(tea.MouseWheelRight)
	if m.boardCol != 2 {
		t.Fatalf("boardCol = %d after wheel right, want 2", m.boardCol)
	}
	m = wheel(tea.MouseWheelLeft)
	if m.boardCol != 1 {
		t.Fatalf("boardCol = %d after wheel left, want 1", m.boardCol)
	}
	// Wheel down on the header row slides the window forward as well.
	m.boardCol = 0
	updated, _ := m.Update(tea.MouseMsg{X: sidebarW + 10, Y: 1, Type: tea.MouseWheelDown})
	m = updated.(Model)
	if m.boardCol != 1 {
		t.Fatalf("boardCol = %d after wheel down on the header, want 1", m.boardCol)
	}
}
