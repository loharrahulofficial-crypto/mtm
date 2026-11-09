package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/muesli/termenv"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
	"github.com/loharrahulofficial-crypto/mtm/internal/theme"
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
	head := StripANSI(body[0])
	if got := cellIndex(head, "MON"); got != 1 {
		t.Errorf("MON starts at cell %d, want 1 (left-aligned with day numbers); header %q", got, head)
	}
}

// TestCalendarGridFillsExactWidth asserts grid lines sum to w cells with
// the gutters: six internal 1-cell gutters plus the trailing right gutter
// (no Sunday-to-border contact), no leftover strip. Regression for "grid
// did not fill the width" and for the missing right gutter.
func TestCalendarGridFillsExactWidth(t *testing.T) {
	for _, w := range []int{60, 98, 133, 190} {
		cols := calendarColWidths(w)
		sum := 7 // six internal gaps + one right gutter
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

// TestCalendarCellOverflowShowsMore pins the busy-cell rule: cell top-aligns
// up to five titles; when the tail does not fit, "+N more" takes the last
// line — never a silent drop.
func TestCalendarCellOverflowShowsMore(t *testing.T) {
	ts := DumpFixedTime
	stamp := ts.Format(core.TimestampFormat)
	day := ts.Format("2006-01-02")
	tasks := []fixtureTask{}
	for i := 0; i < 8; i++ {
		tasks = append(tasks, fixtureTask{
			id:       fmt.Sprintf("k%05d", i),
			title:    fmt.Sprintf("busy day task number %d with some length", i),
			project:  "inbox",
			status:   "todo",
			priority: "medium",
			kind:     "task",
			due:      day + "T10:00",
			created:  stamp,
			updated:  stamp,
		})
	}
	s := fixtureStore(t, tasks)
	defer func() { _ = s.Close() }()
	m := New(dumpConfig(), s, sync.New(s.Root))
	today := dayStartT(m.clock.Now())
	lines := calendarCell(m, today, 30, 6, today)
	plain := make([]string, 0, len(lines))
	for _, l := range lines {
		plain = append(plain, StripANSI(l))
	}
	last := plain[len(plain)-1]
	if !strings.Contains(last, "+") || !strings.Contains(last, "more") {
		t.Fatalf("overflow line missing '+N more':\n%s", strings.Join(plain, "\n"))
	}
	titleRows := 0
	for _, l := range plain[1:] {
		if strings.Contains(l, "busy day task") {
			titleRows++
		}
	}
	if titleRows < 4 {
		t.Fatalf("only %d title rows shown, want at least 4 (cell fills its height):\n%s", titleRows, strings.Join(plain, "\n"))
	}
}

// TestCalendarCellMostlyEmptyKeepsSubtleEdge pins the bottom-edge rule: an
// empty non-selected cell renders a subtle rule on its last row; a selected
// cell carries the accent rule instead.
func TestCalendarCellMostlyEmptyKeepsSubtleEdge(t *testing.T) {
	m := calModel(t)
	day := dayStartT(m.clock.Now().AddDate(0, 0, 20)) // nothing due then
	lines := calendarCell(m, day, 24, 6, dayStartT(m.clock.Now()))
	last := StripANSI(lines[len(lines)-1])
	if !strings.Contains(last, "─") {
		t.Fatalf("mostly-empty cell keeps no bottom edge: %q", last)
	}
}

// TestCalendarLayoutReservesAgendaMinimum pins the agenda's 5-row minimum
// and the 8-row cell cap: the grid yields the leftover to the agenda.
func TestCalendarLayoutReservesAgendaMinimum(t *testing.T) {
	m := calModel(t)
	// Point the calendar at a month whose grid takes only 5 rows
	// (February 2026) so the 8-row cell cap is the binding constraint.
	m.calMonth = time.Date(2026, 2, 1, 0, 0, 0, 0, m.calDay.Location())
	m.calDay = time.Date(2026, 2, 10, 0, 0, 0, 0, m.calDay.Location())
	_, cellH, _, _ := calendarLayout(m, 118, 50)
	if cellH != 8 {
		t.Fatalf("tall calendar: cellH = %d, want 8 (cap)", cellH)
	}
	_, cellH, _, _ = calendarLayout(m, 118, 40)
	if cellH > 8 {
		t.Fatalf("cellH = %d, want ≤ 8", cellH)
	}
	if cellH < 2 {
		t.Fatalf("cellH = %d, want ≥ 2", cellH)
	}
}

// TestCalendarAgendaShowsFourEntriesWithoutMore pins the agenda's minimum
// height: with four or fewer tasks due, nothing is hidden and no "+N more"
// row appears — the agenda takes the grid's leftover space.
func TestCalendarAgendaShowsFourEntriesWithoutMore(t *testing.T) {
	ts := DumpFixedTime
	stamp := ts.Format(core.TimestampFormat)
	today := ts.Format("2006-01-02")
	tasks := []fixtureTask{}
	for i := 0; i < 4; i++ {
		tasks = append(tasks, fixtureTask{
			id:       fmt.Sprintf("a%05d", i),
			title:    fmt.Sprintf("agenda task number %d with a reasonably long title", i),
			project:  "inbox",
			status:   "todo",
			priority: "medium",
			kind:     "task",
			due:      today + "T10:00",
			created:  stamp,
			updated:  stamp,
		})
	}
	s := fixtureStore(t, tasks)
	defer func() { _ = s.Close() }()
	m := New(dumpConfig(), s, sync.New(s.Root))
	m.calDay = dayStartT(m.clock.Now())
	m.calMonth = time.Date(m.calDay.Year(), m.calDay.Month(), 1, 0, 0, 0, 0, m.calDay.Location())
	for _, size := range []struct{ w, h int }{{120, 40}, {200, 50}, {80, 24}} {
		body := StripANSI(strings.Join(renderCalendarBody(m, size.w-2, size.h-3), "\n"))
		for i := 0; i < 4; i++ {
			if !strings.Contains(body, fmt.Sprintf("agenda task number %d", i)) {
				t.Errorf("%dx%d: agenda entry %d missing:\n%s", size.w, size.h, i, body)
			}
		}
		if strings.Contains(body, "+") && strings.Contains(body, "more") {
			t.Errorf("%dx%d: phantom +N more in agenda:\n%s", size.w, size.h, body)
		}
	}
}

// TestCalendarAgendaOverflowOnlyBeyondMin pins that "+N more" appears only
// when entries exceed the visible slots — here, past the 4-entry minimum.
func TestCalendarAgendaOverflowOnlyBeyondMin(t *testing.T) {
	ts := DumpFixedTime
	stamp := ts.Format(core.TimestampFormat)
	today := ts.Format("2006-01-02")
	tasks := []fixtureTask{}
	for i := 0; i < 7; i++ {
		tasks = append(tasks, fixtureTask{
			id:       fmt.Sprintf("b%05d", i),
			title:    fmt.Sprintf("overflow task number %d with a long title", i),
			project:  "inbox",
			status:   "todo",
			priority: "medium",
			kind:     "task",
			due:      today + "T10:00",
			created:  stamp,
			updated:  stamp,
		})
	}
	s := fixtureStore(t, tasks)
	defer func() { _ = s.Close() }()
	m := New(dumpConfig(), s, sync.New(s.Root))
	m.calDay = dayStartT(m.clock.Now())
	m.calMonth = time.Date(m.calDay.Year(), m.calDay.Month(), 1, 0, 0, 0, 0, m.calDay.Location())
	m.view = viewCalendar
	body := StripANSI(strings.Join(renderCalendarBody(m, 118, 37), "\n"))
	if !strings.Contains(body, "+") {
		t.Fatalf("overflowing agenda shows no +N more:\n%s", body)
	}
}

// TestCalendarRightGutter pins the 1-cell right gutter: Sunday's column
// never touches the panel border; every grid row ends on the base gutter
// (same 1-cell base fill used between all columns).
func TestCalendarRightGutter(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	m := calModel(t)
	for _, w := range []int{60, 98, 133, 190} {
		body := renderCalendarBody(m, w, 40)
		grid := parseFrame(strings.Join(body, "\n"))
		base := renderedBGProbe(theme.Defaults().Base)
		if base == "" {
			t.Fatal("could not probe the base background")
		}
		// Row 0 is the month title; row 1 the weekday header; grid rows
		// follow. All of them end on the base gutter.
		for r := 1; ; r++ {
			if r >= len(grid.ch) {
				break
			}
			// Grid + header rows end with the base gutter; agenda rows
			// end on surface. The agenda starts with the blank row after
			// the grid: stop at the first row whose final cell is not on
			// base after the grid ends.
			_, bg := grid.cell(r, w-1)
			if bg == base {
				continue
			}
			break
		}
		// The Sunday cell itself must not reach the border: the panel
		// border lives at x=0/w-1 of the full frame; the grid's last cell
		// ends at w-2 (the gutter sits at w-1).
		for r := 2; r < 2+8*6 && r < len(grid.ch); r++ {
			ch, _ := grid.cell(r, w-1)
			if ch != " " {
				t.Errorf("w=%d row %d: right gutter cell = %q, want blank base gutter", w, r, ch)
				break
			}
		}
	}
}

// TestCalendarAgendaCapEightEntries pins the agenda's 8-entry cap: ten
// tasks due today render at most eight entries plus one "+2 more" row.
func TestCalendarAgendaCapEightEntries(t *testing.T) {
	ts := DumpFixedTime
	stamp := ts.Format(core.TimestampFormat)
	today := ts.Format("2006-01-02")
	tasks := []fixtureTask{}
	for i := 0; i < 10; i++ {
		tasks = append(tasks, fixtureTask{
			id:       fmt.Sprintf("e%05d", i),
			title:    fmt.Sprintf("agenda cap task number %02d with a long descriptive title", i),
			project:  "inbox",
			status:   "todo",
			priority: "medium",
			kind:     "task",
			due:      today + "T10:00",
			created:  stamp,
			updated:  stamp,
		})
	}
	s := fixtureStore(t, tasks)
	defer func() { _ = s.Close() }()
	m := New(dumpConfig(), s, sync.New(s.Root))
	m.calDay = dayStartT(m.clock.Now())
	m.calMonth = time.Date(m.calDay.Year(), m.calDay.Month(), 1, 0, 0, 0, 0, m.calDay.Location())
	m.view = viewCalendar
	body := StripANSI(strings.Join(renderCalendarBody(m, 118, 37), "\n"))
	// Exactly eight entry lines (titled rows before the "+N more" hint).
	shown := strings.Count(body, "agenda cap task number")
	if shown != 8 {
		t.Errorf("agenda shows %d entries, want exactly 8:\n%s", shown, body)
	}
	if !strings.Contains(body, "+2 more") {
		t.Errorf("agenda hides 2 entries without the +N more hint:\n%s", body)
	}
	if strings.Count(body, "+2 more") > 1 {
		t.Errorf("+N more appears more than once:\n%s", body)
	}
}

// TestCalendarCellShowsTimePrefixForTimedTasks pins the time prefix in
// day cells: a timed task renders "16:00 <title>", an untimed task has no
// prefix, and the title still ends (truncated) inside the cell.
func TestCalendarCellShowsTimePrefixForTimedTasks(t *testing.T) {
	ts := DumpFixedTime
	stamp := ts.Format(core.TimestampFormat)
	today := ts.Format("2006-01-02")
	s := fixtureStore(t, []fixtureTask{
		{id: "p00001", title: "Normalize the orders table", project: "inbox", status: "todo", priority: "high", kind: "task", due: today + "T16:00", created: stamp, updated: stamp},
		{id: "p00002", title: "Untimed background task", project: "inbox", status: "todo", priority: "low", kind: "task", due: today, created: stamp, updated: stamp},
	})
	defer func() { _ = s.Close() }()
	m := New(dumpConfig(), s, sync.New(s.Root))
	m.calDay = dayStartT(m.clock.Now())
	m.calMonth = time.Date(m.calDay.Year(), m.calDay.Month(), 1, 0, 0, 0, 0, m.calDay.Location())
	m.view = viewCalendar
	day := dayStartT(m.clock.Now())
	cell := calendarCell(m, day, 24, 7, dayStartT(m.clock.Now()))
	plain := StripANSI(strings.Join(cell, "\n"))
	if !strings.Contains(plain, "16:00") {
		t.Errorf("timed task carries no time prefix in its cell:\n%s", plain)
	}
	if strings.Count(plain, "16:00") != 1 {
		t.Errorf("time prefix rendered %d times in the cell, want 1:\n%s", strings.Count(plain, "16:00"), plain)
	}
	// The untimed title has no HH:MM prefix.
	for _, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, "Untimed") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "16:00") || strings.HasPrefix(trimmed, "09:00") {
				t.Errorf("untimed task picked up a time prefix: %q", trimmed)
			}
		}
	}
}

// Spec 0b.4: narrow cells (< 12 columns) show a count chip in addition to
// the dots when titles can't fit.
func TestCalendarNarrowCellCountChip(t *testing.T) {
	due := DumpFixedTime.Format("2006-01-02")
	tasks := []fixtureTask{
		{id: "n00001", title: "task one", project: "inbox", status: "todo", priority: "high", kind: "task", due: due + "T09:00"},
		{id: "n00002", title: "task two", project: "inbox", status: "todo", priority: "medium", kind: "assignment", due: due + "T11:00"},
		{id: "n00003", title: "task three", project: "inbox", status: "todo", priority: "low", kind: "study", due: due + "T15:00"},
		{id: "n00004", title: "task four", project: "inbox", status: "todo", priority: "low", kind: "task", due: due + "T17:00"},
	}
	s := fixtureStore(t, tasks)
	defer func() { _ = s.Close() }()
	m := BuildDumpModel(dumpConfig(), s, sync.New(s.Root), "calendar", 80, 24, DumpFixedTime)
	m.calDay = DumpFixedTime

	cellH := 4
	day := DumpFixedTime

	narrow := calendarCell(m, day, 9, cellH, day)
	joined := StripANSI(strings.Join(narrow, "\n"))
	if !strings.Contains(joined, "●") {
		t.Errorf("narrow cell missing dots:\n%s", joined)
	}
	if !strings.Contains(joined, "4") {
		t.Errorf("narrow cell missing count chip (4 due):\n%s", joined)
	}

	wide := calendarCell(m, day, 30, cellH, day)
	wideJoined := StripANSI(strings.Join(wide, "\n"))
	if strings.Contains(wideJoined, " 4 ") {
		t.Errorf("wide cell shows the count chip; only narrow cells (<12 cols) should:\n%s", wideJoined)
	}
}
