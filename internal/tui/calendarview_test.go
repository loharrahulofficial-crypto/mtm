package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-runewidth"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

func calModel(t *testing.T) Model {
	t.Helper()
	ts := DumpFixedTime
	stamp := ts.Format(core.TimestampFormat)
	today := ts.Format("2006-01-02")
	s := fixtureStore(t, []fixtureTask{
		{id: "f00001", title: "finished thing one", project: "inbox", status: "done", priority: "medium", kind: "task", due: today + "T09:00", created: stamp, updated: stamp, completed: stamp},
		{id: "f00002", title: "open thing two", project: "inbox", status: "todo", priority: "medium", kind: "task", due: today + "T10:00", created: stamp, updated: stamp},
	})
	m := New(dumpConfig(), s, sync.New(s.Root))
	m.calDay = dayStartT(m.clock.Now())
	m.calMonth = time.Date(m.calDay.Year(), m.calDay.Month(), 1, 0, 0, 0, 0, m.calDay.Location())
	return m
}

// TestCalendarDoneHiddenInCellsByDefault pins the `d` toggle: completed
// tasks are hidden from day cells unless the toggle is on. Regression
// for "calendar showed completed tasks".
func TestCalendarDoneHiddenInCellsByDefault(t *testing.T) {
	m := calModel(t)
	day := dayStartT(m.clock.Now())
	got := tasksDueOn(m, day)
	if len(got) != 1 || got[0].ID != "f00002" {
		t.Fatalf("cells show %d tasks (done included?), want only the open one", len(got))
	}
	m.calShowDone = true
	got = tasksDueOn(m, day)
	if len(got) != 2 {
		t.Fatalf("with calShowDone, cells show %d tasks, want 2", len(got))
	}
}

// TestCalendarToggleDoneKeyFlipsState presses the calendar `d` binding.
func TestCalendarToggleDoneKeyFlipsState(t *testing.T) {
	m := calModel(t)
	m2, _ := m.updateCalendarKey(aToggleCalendarDone)
	if m2.(Model).calShowDone == m.calShowDone {
		t.Fatal("d binding did not toggle calShowDone")
	}
}

// TestCalendarDotsOnNumberLine renders a day cell and asserts dots sit
// on row 0 (right of the day number) and never on the bottom row.
// Regression for "calendar dots looked like the next row".
func TestCalendarDotsOnNumberLine(t *testing.T) {
	m := calModel(t)
	day := dayStartT(m.clock.Now())
	lines := calendarCell(m, day, 20, 4, dayStartT(m.clock.Now()))
	if len(lines) != 4 {
		t.Fatalf("cell is %d lines, want 4", len(lines))
	}
	plain0 := StripANSI(lines[0])
	if !strings.Contains(plain0, "●") {
		t.Errorf("row 0 carries no dots: %q", plain0)
	}
	if strings.Contains(StripANSI(lines[3]), "●") {
		t.Errorf("bottom row carries dots: %q", StripANSI(lines[3]))
	}
}

// TestCalendarTitlesReplaceDots asserts that when the cell is tall enough
// to show titles, dots are dropped entirely.
func TestCalendarTitlesReplaceDots(t *testing.T) {
	m := calModel(t)
	day := dayStartT(m.clock.Now())
	lines := calendarCell(m, day, 20, 6, dayStartT(m.clock.Now()))
	for i, l := range lines {
		if strings.Contains(StripANSI(l), "●") {
			t.Errorf("line %d shows dots in a tall cell: %q", i, StripANSI(l))
		}
	}
}

// TestCalendarWeekdayHeadersLeftAligned asserts the weekday caps start at
// the same cell as the day numbers (column 1, after the leading fill
// space). Regression for "weekday headers were centered".
func TestCalendarWeekdayHeadersLeftAligned(t *testing.T) {
	m := calModel(t)
	w := 98
	body := renderCalendarBody(m, w, 40)
	if len(body) < 2 {
		t.Fatal("no body rows")
	}
	head := StripANSI(body[1])
	if got := cellIndex(head, "MON"); got != 1 {
		t.Errorf("MON starts at cell %d, want 1 (left-aligned with day numbers); header %q", got, head)
	}
}

// TestCalendarGridFillsExactWidth asserts grid lines sum to w cells with
// the gutters (no leftover strip). Regression for "grid did not fill the
// width".
func TestCalendarGridFillsExactWidth(t *testing.T) {
	for _, w := range []int{60, 98, 133, 190} {
		cols := calendarColWidths(w)
		sum := 6
		for _, c := range cols {
			sum += c
		}
		if sum != w {
			t.Errorf("w=%d: columns+gutters sum to %d, want %d", w, sum, w)
		}
		m := calModel(t)
		body := renderCalendarBody(m, w, 40)
		for i, l := range body {
			if got := runewidth.StringWidth(StripANSI(l)); got > w {
				t.Errorf("w=%d line %d is %d cells wide (>w)", w, i, got)
			}
		}
	}
}
