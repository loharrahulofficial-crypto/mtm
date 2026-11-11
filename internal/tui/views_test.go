package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/charmbracelet/lipgloss"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
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
	// Spec 0b.3: the month heading lives only in the panel title; the
	// body's first row is the weekday header.
	if strings.Contains(cal, strings.ToUpper(calendarMonthTitle(m))) {
		t.Fatalf("in-body month heading is still rendered:\n%s", firstLines(cal, 3))
	}
	if !strings.Contains(cal, "MON") {
		t.Fatalf("weekday header missing from the body:\n%s", firstLines(cal, 3))
	}
}

// An exam due on the selected day must wear ✦ next to its title in the
// agenda, not only the cell dot; ordinary tasks must not.
func TestCalendarAgendaExamMark(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	m, _ := newTestModel(t, []*core.Task{
		{Title: "DBMS midterm — algebra", Project: "inbox", Status: "todo", Kind: core.KindExam, Due: today + "T09:00"},
		{Title: "regular chore", Project: "inbox", Status: "todo", Kind: core.KindTask, Due: today + "T10:00"},
	})
	m.view = viewCalendar
	m.calDay = m.clock.Today()
	cal := stripANSI(strings.Join(renderCalendarBody(m, m.width-2, m.height-3), "\n"))
	if !strings.Contains(cal, "✦ DBMS midterm") {
		t.Fatalf("exam ✦ missing next to title in agenda:\n%s", cal)
	}
	if strings.Contains(cal, "✦ regular chore") {
		t.Fatal("✦ leaked to a non-exam task")
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

// ":exam <course> <name> <date>" creates an exam (kind=exam) targeted at the
// course project — the Home empty state points users at it. Exams are
// course events: an unknown course is an error, never the default project.
func TestExecCommandExamCreatesExamKind(t *testing.T) {
	m, s := newTestModel(t, nil)
	if err := s.SaveProject(core.DefaultProject("college/dbms")); err != nil {
		t.Fatal(err)
	}
	out, _ := m.execCommand("exam college/dbms DBMS midterm 2026-11-20")
	m2, ok := out.(Model)
	if !ok {
		t.Fatalf("execCommand returned %T", out)
	}
	found := false
	for _, tk := range m2.store.Tasks() {
		if tk.Kind == "exam" && strings.Contains(tk.Title, "DBMS midterm") {
			found = true
			if tk.Project != "college/dbms" {
				t.Fatalf("exam landed in project %q, want college/dbms", tk.Project)
			}
			if tk.Due != "2026-11-20" {
				t.Fatalf("exam due %q, want 2026-11-20", tk.Due)
			}
		}
	}
	if !found {
		t.Fatal(":exam did not create an exam-kind task")
	}
}

func TestExecCommandExamCourseRequired(t *testing.T) {
	m, s := newTestModel(t, nil)
	if err := s.SaveProject(core.DefaultProject("college/dbms")); err != nil {
		t.Fatal(err)
	}
	out, _ := m.execCommand("exam inbox DBMS midterm 2026-11-20") // ok: inbox exists
	if m2 := out.(Model); len(m2.store.Tasks()) != 1 {
		t.Fatalf("exam targeting inbox should have been created, got %d tasks", len(m2.store.Tasks()))
	}
	out, _ = m.execCommand("exam nosuchcourse DBMS midterm 2026-11-20")
	m2 := out.(Model)
	if len(m2.store.Tasks()) != 1 {
		t.Fatal("unknown course must not create a task")
	}
	if !strings.Contains(m2.statusMsg, "no course") {
		t.Fatalf("statusMsg = %q, want a no-course error", m2.statusMsg)
	}
	// Date is required — ":exam <course> <name>" without one is an error.
	m3 := m2
	out, _ = m3.execCommand("exam college/dbms DBMS midterm")
	m4 := out.(Model)
	if len(m4.store.Tasks()) != 1 {
		t.Fatal(":exam without a date must not create a task")
	}
	if !strings.Contains(m4.statusMsg, "date") {
		t.Fatalf("statusMsg = %q, want a missing-date error", m4.statusMsg)
	}
	// Course match is case-insensitive.
	out, _ = m.execCommand("exam College/DBMS DBMS midterm 2026-11-20")
	if m5 := out.(Model); len(m5.store.Tasks()) != 2 {
		t.Fatalf("case-insensitive course should create the exam, got %d tasks", len(m5.store.Tasks()))
	}
	// Natural-language date parses through the quick-add grammar.
	out, _ = m.execCommand("exam college/dbms OS quiz 2030-05-05")
	if m6 := out.(Model); len(m6.store.Tasks()) != 3 {
		t.Fatal("ISO date should create the exam")
	}
}

// The Home section label must read "THIS WEEK" — letterspaced with thin
// spaces between letters and a three-space word gap — with no extra, missing
// or merged letters, however the panel renders it.
func TestThisWeekLabelRendersExactly(t *testing.T) {
	thin := "\u2009"
	want := "T" + thin + "H" + thin + "I" + thin + "S" + "   " + "W" + thin + "E" + thin + "E" + thin + "K"
	if got := letterspace("THIS WEEK"); got != want {
		t.Fatalf("letterspace(THIS WEEK) = %q, want %q", got, want)
	}
	m, _ := newTestModel(t, nil)
	body := stripANSI(strings.Join(renderHomeBody(m, 120, 39), "\n"))
	if !strings.Contains(body, want) {
		t.Fatalf("home body missing the THIS WEEK label %q", want)
	}
	for _, bad := range []string{"W E E K S", "WEEKS", "THISWEEK", "W E K"} {
		if strings.Contains(body, bad) {
			t.Fatalf("home body contains %q — the THIS WEEK label has extra letters", bad)
		}
	}
}

// Multi-word section labels must keep readable gaps between words: three
// plain spaces, or the label collapses into one long letter-run.
func TestLetterspaceWordGap(t *testing.T) {
	got := letterspace("THIS WEEK")
	if !strings.Contains(got, "S   W") {
		t.Fatalf("word gap is not three spaces: %q", got)
	}
	if strings.Contains(got, "S W") {
		t.Fatalf("word gap collapsed to one space: %q", got)
	}
	// Single words are unchanged in shape.
	if got := letterspace("TODAY"); !strings.HasPrefix(got, "T"+thinSpace+"O") {
		t.Fatalf("single-word label mangled: %q", got)
	}
}

// Empty detail pane: centered, muted, no border.
func TestDetailEmptyStateIsCenteredBorderless(t *testing.T) {
	m, _ := newTestModel(t, nil)
	detail := m.renderDetail(36, 30)
	plain := stripANSI(detail)
	if !strings.Contains(plain, "Nothing selected") {
		t.Fatalf("empty state must name itself, got:\n%s", plain)
	}
	for _, g := range []string{"╭", "╮", "╰", "╯", "│"} {
		if strings.Contains(plain, g) {
			t.Fatalf("empty state must have no border, found %q in:\n%s", g, plain)
		}
	}
	// Centered: the text starts well inside the pane, not at the left edge.
	for _, l := range strings.Split(plain, "\n") {
		if idx := strings.Index(l, "Nothing selected"); idx >= 0 {
			if idx < 5 {
				t.Fatalf("empty state not centered (starts at col %d):\n%s", idx, plain)
			}
			return
		}
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
	m = press(t, m, "1")          // list view
	m.viewFadeStart = time.Time{} // settle the tab crossfade before inspecting the frame
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
	pressed := press(t, upd.(Model), "1")
	pressed.viewFadeStart = time.Time{} // settle the tab crossfade before inspecting the frame
	list := renderContents(pressed)
	if !strings.Contains(stripANSI(list), far.Format("Jan 2")) {
		t.Fatalf("due date missing from list: %q", stripANSI(list))
	}
}

func renderContents(m Model) string { return m.View() }

// TestSectionLabelTextLetterspacesLabelWordOnly pins polish 4 item 6:
// letter-spacing applies to the label word only; anything after a "·"
// separator renders as normal-case text.
func TestSectionLabelTextLetterspacesLabelWordOnly(t *testing.T) {
	s := emptyFixture(t)
	defer func() { _ = s.Close() }()
	m := New(dumpConfig(), s, sync.New(s.Root))
	for _, tc := range []struct{ label, wantBase, wantRest string }{
		{"TASKS · All", "TASKS", "All"},
		{"CALENDAR · Nov 2026", "CALENDAR", "Nov 2026"},
		{"BOARD", "BOARD", ""},
		{"TASKS · All · extra", "TASKS", "All · extra"},
		{"THIS WEEK", "THIS WEEK", ""},
	} {
		got := sectionLabelText(m, tc.label)
		base := m.styles.sectionLabel.Render(letterspace(strings.ToUpper(tc.wantBase)))
		want := base
		if tc.wantRest != "" {
			want += m.styles.sectionLabel.Render(" · " + tc.wantRest)
		}
		if got != want {
			t.Errorf("label %q:\n got %q\nwant %q", tc.label, got, want)
		}
		// The remainder must not carry thin spaces between its letters.
		if strings.Contains(StripANSI(got), letterspace("All")) {
			t.Errorf("remainder %q got letterspaced: %q", tc.wantRest, StripANSI(got))
		}
	}
}

// TestSectionLabelTextKeepsWordGaps pins spec 0.3: multi-word labels keep
// exactly three spaces between words after letterspacing — the project page
// previously passed hand-spaced strings that collapsed the gap.
func TestSectionLabelTextKeepsWordGaps(t *testing.T) {
	s := emptyFixture(t)
	defer func() { _ = s.Close() }()
	m := New(dumpConfig(), s, sync.New(s.Root))
	for _, tc := range []struct{ label, want string }{
		{"NEXT MILESTONE", "N\u2009E\u2009X\u2009T   M\u2009I\u2009L\u2009E\u2009S\u2009T\u2009O\u2009N\u2009E"},
		{"VELOCITY · 12 WEEKS", "V\u2009E\u2009L\u2009O\u2009C\u2009I\u2009T\u2009Y · 12 WEEKS"},
		{"THIS WEEK", "T\u2009H\u2009I\u2009S   W\u2009E\u2009E\u2009K"},
	} {
		got := StripANSI(sectionLabelText(m, tc.label))
		if got != tc.want {
			t.Errorf("label %q:\n got %q\nwant %q", tc.label, got, tc.want)
		}
	}
}

// TestCalendarFrameShowsMonthSuffix pins the calendar panel title:
// "C A L E N D A R · Nov 2026" — letterspaced label, normal-case month.
func TestCalendarFrameShowsMonthSuffix(t *testing.T) {
	s := fixtures["populated"](t)
	defer func() { _ = s.Close() }()
	frame, err := RenderDump(dumpConfig(), s, sync.New(s.Root), "calendar", 120, 40, true, DumpFixedTime)
	if err != nil {
		t.Fatal(err)
	}
	plain := StripANSI(frame)
	want := letterspace("CALENDAR") + " · Nov 2026"
	if !strings.Contains(plain, want) {
		t.Errorf("calendar title missing %q:\n%s", want, plain)
	}
	if strings.Contains(plain, "N O V") {
		t.Error("the month got letterspaced — must stay normal case")
	}
}

// TestListFrameShowsAllSuffix pins the list title: "T A S K S · All".
func TestListFrameShowsAllSuffix(t *testing.T) {
	s := fixtures["populated"](t)
	defer func() { _ = s.Close() }()
	frame, err := RenderDump(dumpConfig(), s, sync.New(s.Root), "list", 120, 40, true, DumpFixedTime)
	if err != nil {
		t.Fatal(err)
	}
	plain := StripANSI(frame)
	want := letterspace("TASKS") + " · All"
	if !strings.Contains(plain, want) {
		t.Errorf("list title missing %q:\n%s", want, plain)
	}
}
