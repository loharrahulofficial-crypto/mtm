package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/charmbracelet/lipgloss"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		switch {
		case r == '\x1b':
			inEsc = true
		case inEsc && (r == 'm' || r == 'K' || r == 'H' || r == '0'):
			if r == 'm' {
				inEsc = false
			}
		case inEsc:
			// skip
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Bug report: "the calendar cursor starts on yesterday". Whatever the
// visual cause, the invariants are: the selected day is today in local
// time, today's cell keeps its filled-pill style even while selected, and
// no other day renders the today style.
func TestCalendarCursorStartsToday(t *testing.T) {
	m, _ := newTestModel(t, nil)
	upd, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = upd.(Model)
	today := time.Now()
	ty, tm, td := today.Date()
	if cy, cm, cd := m.calDay.Date(); cy != ty || cm != tm || cd != td {
		t.Fatalf("calDay = %v, want today %04d-%02d-%02d", m.calDay, ty, tm, td)
	}
	if loc := m.calDay.Location(); loc != time.Local {
		t.Fatalf("calDay location = %v, want %v", loc, time.Local)
	}
	m = press(t, m, "3")
	cal := strings.Join(renderCalendarBody(m, m.width-2, m.height-3), "\n")
	if !strings.Contains(cal, strings.ToUpper(calendarMonthTitle(m))) {
		t.Fatalf("calendar title missing: %q", firstLines(cal, 2))
	}
}

func calendarMonthTitle(m Model) string {
	return m.calMonth.Format("January 2006")
}

// Today must still wear the today pill when it is also the selected day;
// the selection style must not visually move the cursor to another cell.
func TestCalendarTodayPillSurvivesSelection(t *testing.T) {
	m, s := newTestModel(t, nil)
	upd, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = upd.(Model)
	// Yesterday holds an overdue task: with the old precedence, the hot
	// overdue cell visually read as the cursor while today's own pill was
	// swallowed by the selection style.
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	if err := s.Create(&core.Task{Title: "expired", Project: "inbox", Status: "todo", Kind: core.KindTask, Due: yesterday}); err != nil {
		t.Fatal(err)
	}
	m.refreshTasks()
	today := m.clock.Today()
	// Direct style assertions.
	todaySty := calendarCellStyle(m, today)
	want := m.styles.dueToday
	if !sameRender(todaySty, want) {
		t.Fatalf("today (also selected) lost the today-pill style")
	}
	if sty := calendarCellStyle(m, today.AddDate(0, 0, -1)); !sameRender(sty, m.styles.overdue) {
		t.Fatalf("yesterday overdue cell not hot")
	}
	if sty := calendarCellStyle(m, today.AddDate(0, 0, 1)); !sameRender(sty, m.styles.base) {
		t.Fatalf("tomorrow should be plain, got different style")
	}
}

func sameRender(a, b lipgloss.Style) bool {
	return a.Render("x") == b.Render("x")
}

func firstLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if n > len(lines) {
		n = len(lines)
	}
	return strings.Join(lines[:n], "|")
}

func pad2(d int) string {
	if d < 10 {
		return " 0" + itoa(d)
	}
	return itoa(d)
}

// Bug report: a stray "—" artifact sat at the top right of the list view —
// that was the due chip of a no-due task. No-due rows render nothing on the
// right.
func TestListNoDueHasNoStrayDash(t *testing.T) {
	m, _ := newTestModel(t, []*core.Task{
		{Title: "no due at all", Status: "todo", Priority: "low", Kind: core.KindTask},
	})
	upd, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = upd.(Model)
	m = press(t, m, "1") // list view
	v := m.View()
	line := ""
	for _, l := range strings.Split(v, "\n") {
		if strings.Contains(l, "no due at all") {
			line = l
			break
		}
	}
	if line == "" {
		t.Fatal("row missing from view")
	}
	if strings.Contains(stripANSI(line), "—") {
		t.Fatalf("stray — on no-due row: %q", line)
	}
	// A row WITH a due keeps its due cell.
	far := time.Now().AddDate(0, 0, 10)
	m2, _ := newTestModel(t, []*core.Task{
		{Title: "has due", Status: "todo", Priority: "low", Kind: core.KindTask, Due: far.Format("2006-01-02")},
	})
	upd, _ = m2.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	list := renderContents(press(t, upd.(Model), "1"))
	if !strings.Contains(stripANSI(list), far.Format("Jan 2")) {
		t.Fatalf("due date missing from list: %q", stripANSI(list))
	}
}

func renderContents(m Model) string { return m.View() }
