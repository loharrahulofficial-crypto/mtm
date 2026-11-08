package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"

	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

// cellSlice returns the cell window [start, start+width) of a plain (no ANSI) line.
func cellSlice(line string, start, width int) string {
	var b strings.Builder
	x := 0
	for _, r := range line {
		w := runewidth.RuneWidth(r)
		if x+w > start && x < start+width {
			b.WriteRune(r)
		}
		x += w
		if x >= start+width {
			break
		}
	}
	return b.String()
}

func cellIndex(line, substr string) int {
	// Byte index of substr converted to cell offset.
	byteIdx := strings.Index(line, substr)
	if byteIdx < 0 {
		return -1
	}
	return runewidth.StringWidth(line[:byteIdx])
}

// TestListHeaderMatchesRows pins the single column layout: the header's
// column start cells must equal every row's column start cells — including
// subtask rows, at 80, 120 and 200 columns. Regression for "list header
// misalignment".
func TestListHeaderMatchesRows(t *testing.T) {
	for _, w := range []int{80, 120, 200} {
		t.Run(fmt.Sprintf("%d", w), func(t *testing.T) {
			s := fixtures["subtasks"](t)
			defer func() { _ = s.Close() }()
			m := New(dumpConfig(), s, sync.New(s.Root))
			col := listColumnLayout(w)
			lines := renderListBody(m, w, 200)
			if len(lines) < 2 {
				t.Fatal("no rows rendered")
			}
			plain := make([]string, len(lines))
			for i, l := range lines {
				plain[i] = StripANSI(l)
			}
			header := plain[0]

			// Header start positions come from the shared layout.
			for token, want := range map[string]int{
				"STS":   4,
				"PRIO":  4 + col.statusW + 1,
				"TITLE": col.titleX,
				"DUE":   col.dueX,
			} {
				if got := cellIndex(header, token); got != want {
					t.Errorf("header %q starts at cell %d, want %d", token, got, want)
				}
			}
			if col.courseW > 0 {
				if got := cellIndex(header, "COURSE"); got != col.courseX {
					t.Errorf("header COURSE starts at cell %d, want %d", got, col.courseX)
				}
			}

			// Every task row must start its columns at the same cells.
			rowLines := plain[1:]
			if len(rowLines) != len(m.rows) {
				t.Fatalf("%d row lines for %d rows (grouping must be off for this fixture)", len(rowLines), len(m.rows))
			}
			for i, row := range m.rows {
				line := rowLines[i]
				if !strings.HasPrefix(line, "▌") {
					t.Fatalf("row %d line %q missing stripe", i, line)
				}
				if got := strings.TrimSpace(cellSlice(line, 4, col.statusW)); got != strings.ToLower(strings.TrimSpace(row.Task.Status)) {
					t.Errorf("row %d status cell = %q, want %q (header STS at cell 4)", i, got, strings.ToLower(row.Task.Status))
				}
				if got := strings.TrimSpace(cellSlice(line, 4+col.statusW+1, col.prioW)); got != strings.TrimSpace(priorityPlain(row.Task.Priority)) {
					t.Errorf("row %d prio cell = %q, want %q (header PRIO at cell %d)", i, got, strings.TrimSpace(priorityPlain(row.Task.Priority)), 4+col.statusW+1)
				}
				if row.Depth == 0 {
					// Title text starts exactly at titleX (no tree).
					got := strings.TrimLeft(cellSlice(line, col.titleX, runewidth.StringWidth(row.Task.Title)+4), " ")
					if len(row.Task.Title) > 0 && !strings.HasPrefix(got, string([]rune(row.Task.Title)[0])) {
						t.Errorf("row %d title cell starts at %q, want first glyph of %q at cell %d", i, got, row.Task.Title, col.titleX)
					}
					if strings.Contains(line, "↳") {
						t.Errorf("row %d (depth 0) unexpectedly carries a subtask tree marker", i)
					}
				} else {
					// Subtask rows: indent (2 cells per depth) inside the TITLE
					// cell only, tree marker at titleX+depth*2, title text after
					// it. STS and PRIO stay pinned.
					indent := row.Depth * 2
					if got := cellSlice(line, col.titleX, indent); strings.TrimSpace(got) != "" {
						t.Errorf("row %d indent cells = %q, want blanks", i, got)
					}
					if got := cellSlice(line, col.titleX+indent, 2); got != "↳ " {
						t.Errorf("row %d tree marker = %q at cell %d, want %q", i, got, col.titleX+indent, "↳ ")
					}
					titleCell := cellSlice(line, col.titleX+indent+2, 1)
					if len(row.Task.Title) > 0 && titleCell != string([]rune(row.Task.Title)[0]) {
						t.Errorf("row %d subtask title starts at %q, want %q at cell %d", i, titleCell, string([]rune(row.Task.Title)[0]), col.titleX+indent+2)
					}
				}
				// Due cell, when filled, starts at dueX (and DUE header above it).
				dueCell := cellSlice(line, col.dueX, col.dueW)
				if row.Task.Due != "" {
					if strings.TrimSpace(dueCell) == "" {
						t.Errorf("row %d has due %q but due cell is blank", i, row.Task.Due)
					}
				}
			}
		})
	}
}
