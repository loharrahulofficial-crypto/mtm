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

// An empty day in the calendar must show a day summary in the detail pane,
// not a stale task from the previously selected day.
func TestCalendarDetailFollowsSelectionEmptyDay(t *testing.T) {
	m, _ := newTestModel(t, []*core.Task{
		{Title: "task today", Project: "inbox", Status: "todo", Kind: core.KindTask, Due: time.Now().Format("2006-01-02")},
	})
	m.view = viewCalendar
	m.calDay = m.clock.Today()
	detail := m.renderDetail(36, 30)
	plain := stripANSI(detail)
	if !strings.Contains(plain, "task today") {
		t.Fatalf("detail should show today's task, got:\n%s", plain)
	}
	// Move to a day with nothing due; the detail must not show the task.
	m.calDay = m.clock.Today().AddDate(0, 0, 9)
	detail = m.renderDetail(36, 30)
	plain = stripANSI(detail)
	if strings.Contains(plain, "task today") {
		t.Fatalf("stale task leaked into the empty day's detail:\n%s", plain)
	}
	if !strings.Contains(plain, "Nothing due") {
		t.Fatalf("empty day should show the Nothing-due summary, got:\n%s", plain)
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
	// An overdue task exists but must not steal the today pill or flip an
	// out-of-month cell to hot. (The probe-day check below covers the
	// overdue branch deterministically.)
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

	// A weekday in the displayed month, different from today: exercises the
	// plain-cell and the overdue-cell branches deterministically.
	probeDay := func() time.Time {
		base := time.Date(m.calMonth.Year(), m.calMonth.Month(), 1, 0, 0, 0, 0, m.calMonth.Location())
		for i := 0; i < 28; i++ {
			d := base.AddDate(0, 0, i)
			if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
				continue
			}
			if !dayStartT(d).Equal(dayStartT(m.calDay)) {
				return d
			}
		}
		return time.Time{}
	}()
	if !probeDay.IsZero() {
		if sty := calendarCellStyle(m, probeDay); !sameRender(sty, m.styles.base) {
			t.Fatalf("probe day %s should be plain, got different style", probeDay.Format("2006-01-02"))
		}
		// Overdue task on the probe day makes its cell hot.
		overdueDay := probeDay.Format("2006-01-02")
		if err := s.Create(&core.Task{Title: "expired probe", Project: "inbox", Status: "todo", Kind: core.KindTask, Due: overdueDay}); err != nil {
			t.Fatal(err)
		}
		m.refreshTasks()
		if sty := calendarCellStyle(m, probeDay); !sameRender(sty, m.styles.overdue) {
			t.Fatalf("probe day %s with an overdue task should be hot", overdueDay)
		}
	}
}

func sameRender(a, b lipgloss.Style) bool {
	// Compare resolved attributes, not rendered bytes: under a color profile
	// that drops attributes (e.g. Ascii when piped), distinct styles
	// render identically, so byte comparison can't see the difference.
	return a.GetForeground() == b.GetForeground() &&
		a.GetBackground() == b.GetBackground() &&
		a.GetBold() == b.GetBold() &&
		a.GetItalic() == b.GetItalic() &&
		a.GetUnderline() == b.GetUnderline() &&
		a.GetStrikethrough() == b.GetStrikethrough()
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
