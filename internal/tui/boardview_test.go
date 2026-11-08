package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
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
