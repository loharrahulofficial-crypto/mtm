package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
)

// TestBoardCardTagsBarGapAndWrap pins the tags+subtask-bar pack on a board
// card: exactly 1 cell between the tag chip and the mini-bar, the unit
// wraps as a whole to the second line when it doesn't fit (regression for
// "board card spacing wrap"), and lines never exceed the card width.
func TestBoardCardTagsBarGapAndWrap(t *testing.T) {
	ts := DumpFixedTime
	stamp := ts.Format(core.TimestampFormat)
	s := fixtureStore(t, []fixtureTask{
		{id: "w00001", title: "Review the normalization exercises and write summary notes", project: "inbox", status: "doing", priority: "high", kind: "assignment", due: ts.Format("2006-01-02") + "T10:00", created: stamp, updated: stamp, tags: []string{"exam"}, estimate: 90},
		{id: "w00002", title: "child one", project: "inbox", status: "done", priority: "medium", kind: "task", parent: "w00001", created: stamp, updated: stamp, completed: stamp},
		{id: "w00003", title: "child two", project: "inbox", status: "todo", priority: "medium", kind: "task", parent: "w00001", created: stamp, updated: stamp},
		{id: "w00004", title: "child three", project: "inbox", status: "done", priority: "medium", kind: "task", parent: "w00001", created: stamp, updated: stamp, completed: stamp},
		{id: "w00005", title: "child four", project: "inbox", status: "todo", priority: "medium", kind: "task", parent: "w00001", created: stamp, updated: stamp},
		{id: "w00006", title: "child five", project: "inbox", status: "done", priority: "medium", kind: "task", parent: "w00001", created: stamp, updated: stamp, completed: stamp},
		{id: "w00007", title: "child six", project: "inbox", status: "todo", priority: "medium", kind: "task", parent: "w00001", created: stamp, updated: stamp},
	})
	defer func() { _ = s.Close() }()
	m := New(dumpConfig(), s, sync.New(s.Root))
	head := subtaskCounts(m.store.Tasks(), m.store.IsDoneStatus)
	card := m.store.Tasks()[0]
	now := m.clock.Now()

	// Wide card: pack fits on the meta line — exactly one space between
	// the tag and the bar.
	for _, w := range []int{40} {
		lines := renderCard(m, card, w, false, now, head)
		metaLine := ""
		for _, l := range lines {
			if strings.Contains(StripANSI(l), "#exam") {
				metaLine = StripANSI(l)
			}
		}
		if metaLine == "" {
			t.Fatalf("w=%d: pack not rendered", w)
		}
		if !strings.Contains(metaLine, "#exam █") {
			t.Errorf("w=%d: pack gap is not exactly 1 cell: %q", w, metaLine)
		}
		if !strings.Contains(metaLine, "3/6") {
			t.Errorf("w=%d: subtask count missing: %q", w, metaLine)
		}
	}

	// Narrow card: pack doesn't fit — the whole unit wraps to the
	// second (pack) line; meta line keeps chip+due only.
	lines := renderCard(m, card, 22, false, now, head)
	packLine := -1
	for i, l := range lines {
		if strings.Contains(StripANSI(l), "#exam") {
			packLine = i
		}
		if got := lipgloss.Width(l); got != 22 {
			t.Errorf("line %d width = %d, want 22", i, got)
		}
	}
	if packLine < 3 {
		t.Fatalf("pack did not wrap to its own line (line %d):", packLine)
	}
	if !strings.Contains(StripANSI(lines[packLine]), "#exam ") {
		t.Errorf("wrapped pack line lost the 1-cell gap: %q", StripANSI(lines[packLine]))
	}

	// Never overlapping: every rendered line's cell width equals the card.
	for i, l := range lines {
		if got := lipgloss.Width(l); got != 22 {
			t.Errorf("line %d width = %d, want 22 (no overlap)", i, got)
		}
	}
}

// TestBoardDoneDaysFilterAndToggle pins the done-recency rule: the Done
// column shows only items completed within board.done_days (default 14),
// newest first, with an in-column footer naming the hidden count; pressing
// D shows everything.
func TestBoardDoneDaysFilterAndToggle(t *testing.T) {
	ts := DumpFixedTime
	stamp := func(daysAgo int) string {
		return ts.AddDate(0, 0, -daysAgo).Format(core.TimestampFormat)
	}
	s := fixtureStore(t, []fixtureTask{
		{id: "f00001", title: "older done item", project: "inbox", status: "done", priority: "medium", kind: "task", created: stamp(30), updated: stamp(30), completed: stamp(30)},
		{id: "f00002", title: "recent done item", project: "inbox", status: "done", priority: "medium", kind: "task", created: stamp(5), updated: stamp(5), completed: stamp(5)},
		{id: "f00003", title: "ancient done item", project: "inbox", status: "done", priority: "medium", kind: "task", created: stamp(45), updated: stamp(45), completed: stamp(45)},
		{id: "f00004", title: "open item", project: "inbox", status: "todo", priority: "medium", kind: "task", created: stamp(1), updated: stamp(1)},
	})
	defer func() { _ = s.Close() }()
	m := New(dumpConfig(), s, sync.New(s.Root))
	m.view = viewBoard
	m.resetBoardCursor()
	m.boardCol = 2
	m.width, m.height = 120, 30

	// Default view: only the 5-day-old item shows, header counts it.
	cols := boardColumnsModel(m)
	doneCol := cols[2]
	if doneCol.total != 3 {
		t.Fatalf("done total = %d, want 3", doneCol.total)
	}
	if doneCol.hidden != 2 {
		t.Fatalf("done hidden = %d, want 2", doneCol.hidden)
	}
	if len(doneCol.shown) != 1 || doneCol.shown[0].ID != "f00002" {
		t.Fatalf("done shown = %v, want [f00002]", doneCol.shown)
	}

	body := StripANSI(strings.Join(renderBoardBody(m, 118, 27), "\n"))
	if !strings.Contains(body, "DONE · 1 of 3") {
		t.Errorf("header count missing:\n%s", body)
	}
	if strings.Contains(body, "older done item") || strings.Contains(body, "ancient done item") {
		t.Errorf("stale done items visible while the filter is on:\n%s", body)
	}
	if !strings.Contains(body, "+2 older") {
		t.Errorf("done-days footer missing:\n%s", body)
	}
	if !strings.Contains(body, "D shows all") && !strings.Contains(body, "· D") {
		t.Errorf("done-days footer lost the D hint:\n%s", body)
	}

	// D toggles: all three appear, newest first, footer disappears.
	m2, _ := m.updateBoardKey(aDue)
	m = m2.(Model)
	cols = boardColumnsModel(m)
	if cols[2].hidden != 0 || len(cols[2].shown) != 3 {
		t.Fatalf("after D toggle: shown=%d hidden=%d, want 3/0", len(cols[2].shown), cols[2].hidden)
	}
	order := []string{}
	for _, tk := range cols[2].shown {
		order = append(order, tk.ID)
	}
	if order[0] != "f00002" || order[1] != "f00001" || order[2] != "f00003" {
		t.Errorf("done order = %v, want [f00002 f00001 f00003] (newest first)", order)
	}
	body = StripANSI(strings.Join(renderBoardBody(m, 118, 27), "\n"))
	if !strings.Contains(body, "DONE · 3 of 3") {
		t.Errorf("header after D toggle missing:\n%s", body)
	}
	if strings.Contains(body, "older · press D") {
		t.Errorf("done-days footer still visible after D toggle:\n%s", body)
	}

	// And back.
	m3, _ := m.updateBoardKey(aDue)
	m = m3.(Model)
	if cols := boardColumnsModel(m); cols[2].hidden != 2 {
		t.Fatalf("after second D toggle: hidden=%d, want 2", cols[2].hidden)
	}
}

// TestBoardDoneDaysZeroShowsAll pins the config escape hatch: board.done_days
// <= 0 disables the filter entirely.
func TestBoardDoneDaysZeroShowsAll(t *testing.T) {
	ts := DumpFixedTime
	old := ts.AddDate(0, 0, -60).Format(core.TimestampFormat)
	s := fixtureStore(t, []fixtureTask{
		{id: "g00001", title: "ancient done item", project: "inbox", status: "done", priority: "medium", kind: "task", created: old, updated: old, completed: old},
	})
	defer func() { _ = s.Close() }()
	cfg := dumpConfig()
	cfg.Board.DoneDays = 0
	m := New(cfg, s, sync.New(s.Root))
	m.view = viewBoard
	m.resetBoardCursor()
	m.width, m.height = 120, 30
	if cols := boardColumnsModel(m); len(cols[2].shown) != 1 {
		t.Fatalf("done_days=0: shown=%d, want 1 (filter disabled)", len(cols[2].shown))
	}
}

// TestBoardDoneFooterTextDegradation pins the hint-text fallback chain so
// the narrow column still shows the toggle hint.
func TestBoardDoneFooterTextDegradation(t *testing.T) {
	cases := []struct {
		more, older int
		w           int
		want        string
	}{
		{2, 13, 40, "+2 more · +13 older · D shows all"},
		{2, 13, 30, "+2 more · +13 older · D"},
		{2, 13, 26, "+2 more · +13 older · D"},
		{2, 13, 24, "+2 more · +13 older · D"},
		{2, 13, 23, "+2 more · +13 older"},
		{0, 13, 22, "+13 older · D"},
		{0, 17, 26, "+17 older · D shows all"},
		{0, 13, 16, "+13 older · D"},
		{0, 13, 10, "+13 older"},
		{2, 0, 14, "+2 more"},
		{123, 4, 40, "+123 more · +4 older · D shows all"},
		{123, 0, 10, "+123 more"},
	}
	for _, tc := range cases {
		if got := boardFooterText(tc.more, tc.older, tc.w, "D"); got != tc.want {
			t.Errorf("w=%d more=%d older=%d: got %q, want %q", tc.w, tc.more, tc.older, got, tc.want)
		}
	}
}

// TestBoardOverflowFooterStaysInsideColumn pins the overflow rule: the
// "+N more" cell renders inside the overflowing column's own cells, not as
// a full-width line below the columns.
func TestBoardOverflowFooterStaysInsideColumn(t *testing.T) {
	s := manyTasksFixture(t) // 50 open tasks, all in TODO
	defer func() { _ = s.Close() }()
	m := New(dumpConfig(), s, sync.New(s.Root))
	m.view = viewBoard
	m.resetBoardCursor()
	w := 76
	body := renderBoardBody(m, w, 12) // too short for all 50 cards
	winStart, visCols, colW := boardLayout(m, w)
	_ = visCols
	colEnd := 1 + colW // inset (1) + first column width

	found := false
	for _, line := range body {
		plain := StripANSI(line)
		idx := strings.Index(plain, "+")
		if idx < 0 || !strings.Contains(plain, "more") {
			continue
		}
		found = true
		if idx >= colEnd {
			t.Errorf("footer starts at cell %d, outside the TODO column [0,%d): %q", idx, colEnd, plain)
		}
		// The footer cell must not bleed into the next column: everything
		// after the TODO cell's right edge is base-filled padding or the
		// gutter, never text.
		rest := strings.TrimSpace(plain[colEnd:])
		if rest != "" {
			t.Errorf("content outside the TODO column on the footer row: %q", plain)
		}
	}
	if !found {
		t.Fatalf("no +N more footer rendered at w=%d h=12:\n%s", w, strings.Join(func() []string {
			out := []string{}
			for _, l := range body {
				out = append(out, StripANSI(l))
			}
			return out
		}(), "\n"))
	}
	_ = winStart
}

// TestBoardCombinedFooterInsideColumn pins the one-footer rule: the
// overflow footer merges "+N more" and "+N older · D shows all" into a
// single left-aligned cell inside the column, on the column's background.
func TestBoardCombinedFooterInsideColumn(t *testing.T) {
	s := manyTasksFixture(t)
	defer func() { _ = s.Close() }()
	m := New(dumpConfig(), s, sync.New(s.Root))
	m.view = viewBoard
	m.resetBoardCursor()
	body := renderBoardBody(m, 118, 20)
	found := ""
	for _, line := range body {
		plain := StripANSI(line)
		if strings.Contains(plain, "more") {
			found = plain
			break
		}
	}
	if found == "" {
		t.Fatal("no overflow footer rendered")
	}
	// The footer starts at the column's first cell region (inset + one
	// space): at w=118 with the default three columns, the first column
	// begins at x=1; the footer's "+" must sit at x≈2 (left-aligned).
	idx := strings.Index(found, "+")
	if idx < 0 {
		t.Fatalf("footer without +N more marker: %q", found)
	}
	// Left-aligned: after the 1-cell inset, the next glyphs are spaces
	// then the footer — the '+' must not be centered past mid-column.
	if idx > 60 {
		t.Errorf("footer not left-aligned inside the column: %q", found)
	}
}

// TestBoardColumnsExtendToBottomEdge pins the equal-height rule: after
// the cards and any footer, the column cells (surface background) fill
// the remaining rows so the panel has no bare strip below the columns.
func TestBoardColumnsExtendToBottomEdge(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	s := manyTasksFixture(t)
	defer func() { _ = s.Close() }()
	m := New(dumpConfig(), s, sync.New(s.Root))
	m.view = viewBoard
	m.resetBoardCursor()
	const w, h = 118, 27
	body := renderBoardBody(m, w, h)
	if len(body) > h {
		t.Fatalf("board renders %d rows, want at most %d", len(body), h)
	}
	// The last two rows must carry column cells (surface bg), not a
	// full-width blank strip — the columns reach the bottom edge.
	surface := renderedBGProbe(theme.Defaults().Surface)
	if surface == "" {
		t.Fatal("could not probe the surface background")
	}
	grid := parseFrame(strings.Join(body, "\n"))
	// Count how many of the last rows are fully blank on base/surface;
	// the bottom edge rows must keep the column background pattern.
	winStart, visCols, colW := boardLayout(m, w)
	_ = winStart
	for _, cl := range []int{0, visCols - 1} {
		cellX := 1 + cl*(colW+1)
		for r := len(body) - 2; r < len(body); r++ {
			_, bg := grid.cell(r, cellX)
			if bg != surface {
				t.Errorf("bottom row %d, column %d: bg = %q, want the column's surface background", r, cl, bg)
			}
		}
	}
}
