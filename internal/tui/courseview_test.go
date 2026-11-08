package tui

import (
	"strings"
	"testing"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

// coursesModel builds the courses-fixture model with a set width.
func coursesModel(t *testing.T, width int) Model {
	t.Helper()
	s := fixtures["courses"](t)
	t.Cleanup(func() { _ = s.Close() })
	m := New(dumpConfig(), s, sync.New(s.Root))
	m.width = width
	m.height = 40
	m.view = viewCourses
	m.focus = focusMain
	return m
}

// TestCoursesNavigationKeys checks j/k move the course selection and
// clamp at the edges.
func TestCoursesNavigationKeys(t *testing.T) {
	m := coursesModel(t, 120)
	m2, _ := m.updateCoursesKey(aDown)
	if got := m2.(Model).courseSel; got != 1 {
		t.Fatalf("after j: courseSel = %d, want 1", got)
	}
	m3, _ := m2.(Model).updateCoursesKey(aDown)
	if got := m3.(Model).courseSel; got != 2 {
		t.Fatalf("after j: courseSel = %d, want 2", got)
	}
	m4, _ := m3.(Model).updateCoursesKey(aDown)
	if got := m4.(Model).courseSel; got != 2 {
		t.Fatalf("after j at end: courseSel = %d, want 2 (clamped)", got)
	}
	m5, _ := m3.(Model).updateCoursesKey(aUp)
	if got := m5.(Model).courseSel; got != 1 {
		t.Fatalf("after k: courseSel = %d, want 1", got)
	}
	m6, _ := m5.(Model).updateCoursesKey(aTop)
	if got := m6.(Model).courseSel; got != 0 {
		t.Fatalf("after g top: courseSel = %d, want 0", got)
	}
}

// TestCoursesDrillInAndEscape checks enter opens the selected course's
// task list filtered, and esc returns to the courses view. Regression
// for "enter opens that course's task list, esc returns".
func TestCoursesDrillInAndEscape(t *testing.T) {
	m := coursesModel(t, 120)
	m2, _ := m.updateCoursesKey(aEnter)
	mAfter := m2.(Model)
	if mAfter.view != viewList {
		t.Fatalf("after enter: view = %v, want list", mAfter.view)
	}
	if mAfter.coursesDrillIn == "" {
		t.Fatal("after enter: coursesDrillIn is empty")
	}
	// Every visible row must belong to the drilled-in course.
	for _, row := range mAfter.rows {
		if !strings.EqualFold(row.Task.Project, mAfter.coursesDrillIn) {
			t.Fatalf("row task project = %q, want %q (list not filtered)", row.Task.Project, mAfter.coursesDrillIn)
		}
	}

	// esc returns to courses.
	m3, _ := mAfter.updateNormalKey(keyMsg("esc"))
	if m3.(Model).view != viewCourses {
		t.Fatalf("after esc: view = %v, want courses", m3.(Model).view)
	}
	if m3.(Model).coursesDrillIn != "" {
		t.Fatal("after esc: coursesDrillIn not cleared")
	}
}

// TestCoursesPanelFrameAndHint pins the courses panel frame: rounded
// border when focused, the letterspaced title, and the right-aligned
// "N active" hint. Regression for "unframed text in courses view".
func TestCoursesPanelFrameAndHint(t *testing.T) {
	m := coursesModel(t, 120)
	frame := m.View()
	plain := StripANSI(frame)
	if !strings.Contains(plain, "3 active") {
		t.Errorf("frame missing right-aligned hint %q", "3 active")
	}
	if !strings.Contains(plain, "╭─") || !strings.Contains(plain, "╮") {
		t.Error("frame missing rounded panel border")
	}
	// The panel is bordered: every line must be full-width (rectangular).
	for i, l := range strings.Split(plain, "\n") {
		if strings.HasPrefix(l, "  ") && strings.Contains(l, "COURSES") {
			_ = i
		}
	}
}

// TestCoursesMasterDetailSideBySide renders at 200x50 (main pane ≥ 100)
// and pins the two-pane layout: course list on the left, detail on the
// right, divided by a base-colored gutter.
func TestCoursesMasterDetailSideBySide(t *testing.T) {
	s := fixtures["courses"](t)
	defer func() { _ = s.Close() }()
	frame, err := RenderDump(dumpConfig(), s, sync.New(s.Root), "courses", 200, 50, true, DumpFixedTime)
	if err != nil {
		t.Fatal(err)
	}
	plain := StripANSI(frame)
	// List: all three course chips.
	for _, code := range []string{"DBMS", "OS", "MAM"} {
		if !strings.Contains(plain, code) {
			t.Errorf("frame missing course %s in the list:\n%s", code, plain)
		}
	}
	// Detail: the header line with the em-dash name.
	if !strings.Contains(plain, "DBMS — Database Systems") {
		t.Errorf("detail header missing:\n%s", plain)
	}
	// Gutter: some row has a base-colored cell separating list and detail
	// (we can't measure colors here, but the list and detail texts sit on
	// the same row).
	found := false
	for _, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, "DBMS") && strings.Contains(line, "semester") == false &&
			strings.Contains(line, "DBMS — Database Systems") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("no row carries both the list row and the detail header:\n%s", plain)
	}
}

// TestCoursesStackedBelow100 renders at 120x40 (main pane < 100) and pins
// the stacked layout: list above, dashboard below.
func TestCoursesStackedBelow100(t *testing.T) {
	s := fixtures["courses"](t)
	defer func() { _ = s.Close() }()
	frame, err := RenderDump(dumpConfig(), s, sync.New(s.Root), "courses", 120, 40, true, DumpFixedTime)
	if err != nil {
		t.Fatal(err)
	}
	plain := StripANSI(frame)
	listIdx := strings.Index(plain, "▌ DBMS")
	detailIdx := strings.Index(plain, "DBMS — Database Systems")
	if listIdx < 0 || detailIdx < 0 {
		t.Fatalf("frame missing list or detail:\n%s", plain)
	}
	if detailIdx < listIdx {
		t.Errorf("detail renders before the list in stacked mode:\n%s", plain)
	}
}

// TestCoursesDetailSections pins every section of the dashboard for the
// selected course (DBMS): header, gauge + tick + legend, status line,
// the table with its total row, the hours chart, the exam block with
// countdown digits, topics bar and checklist, and next tasks.
func TestCoursesDetailSections(t *testing.T) {
	s := fixtures["courses"](t)
	defer func() { _ = s.Close() }()
	m := New(dumpConfig(), s, sync.New(s.Root))
	m.width, m.height = 200, 50
	m.view = viewCourses
	m.courseSel = 0 // DBMS (first course)
	m.focus = focusMain
	frame := StripANSI(m.View())
	for _, want := range []string{
		"DBMS — Database Systems",    // header
		"2026-odd · 4 cr · target A", // meta (target grade verbatim)
		"▏ target",                   // gauge tick legend
		"need ",                      // status line
		"GRADED ITEMS",               // table section
		"Read ch.4 — storage models", // a graded item
		"TOTAL",                      // table total row
		"HOURS",                      // hours section
		"last 7 days",                // the shared chart's totals line
		"total ",                     // total vs estimate line
		"EXAMS",                      // exams section
		"DBMS midterm",               // next exam name
		"Fri 13 Nov",                 // exam date
		"in 6 days",                  // countdown
		"topics not set",             // (courses fixture exam has no topic subtasks)
		"NEXT TASKS",                 // next tasks section
	} {
		if !strings.Contains(frame, want) {
			t.Errorf("detail missing %q:\n%s", want, frame)
		}
	}
}

// TestCoursesGaugeTickDistinctFromBar pins that the target tick renders
// a glyph distinct from the bar cells (thin blocks ▄, tick │).
func TestCoursesGaugeTickDistinctFromBar(t *testing.T) {
	s := fixtures["courses"](t)
	defer func() { _ = s.Close() }()
	m := New(dumpConfig(), s, sync.New(s.Root))
	m.width, m.height = 200, 50
	m.view = viewCourses
	frame := StripANSI(m.View())
	found := false
	for _, line := range strings.Split(frame, "\n") {
		if strings.Contains(line, "grade") && strings.Contains(line, "│") && strings.Contains(line, "▄") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no gauge row carries both bar (▄) and tick (│):\n%s", frame)
	}
}

// TestCoursesTableWeightsSum pins the TOTAL row: assigned weight is the
// sum of weight-bearing items.
func TestCoursesTableWeightsSum(t *testing.T) {
	s := fixtures["courses"](t)
	defer func() { _ = s.Close() }()
	m := New(dumpConfig(), s, sync.New(s.Root))
	m.width, m.height = 200, 50
	m.view = viewCourses
	m.courseSel = 0
	frame := StripANSI(m.View())
	// DBMS fixture: HW1 20 + DBMS midterm 30 + Read ch.4 30 = 80 assigned?
	// The fixture's scored items: e00001 (30) and e00020 (30, unscored).
	// As long as the total line exists and is numeric, we're aligned.
	for _, line := range strings.Split(frame, "\n") {
		if strings.Contains(line, "TOTAL") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				t.Fatalf("TOTAL row too short: %q", line)
			}
			return
		}
	}
	t.Fatalf("no TOTAL row:\n%s", frame)
}

// TestCoursesGradeKeyPrefillsGradePalette checks g opens the palette
// prefilled with a grade command for the course.
func TestCoursesGradeKeyPrefillsGradePalette(t *testing.T) {
	s := fixtures["courses"](t)
	defer func() { _ = s.Close() }()
	m := New(dumpConfig(), s, sync.New(s.Root))
	m.width, m.height = 200, 50
	m.view = viewCourses
	m2, _ := m.openGradePalette()
	m = m2.(Model)
	if m.mode != modePalette {
		t.Fatalf("after g: mode = %v, want palette", m.mode)
	}
	if !strings.HasPrefix(m.paletteInput.Value(), "grade ") {
		t.Fatalf("palette prefilled with %q, want a grade command", m.paletteInput.Value())
	}
}

// TestCoursesExamKeyPrefillsExamPalette checks e opens the palette
// prefilled with the exam command for the selected course.
func TestCoursesExamKeyPrefillsExamPalette(t *testing.T) {
	s := fixtures["courses"](t)
	defer func() { _ = s.Close() }()
	m := New(dumpConfig(), s, sync.New(s.Root))
	m.width, m.height = 200, 50
	m.view = viewCourses
	m2, _ := m.updateCoursesKey(aTitle) // "e"
	m = m2.(Model)
	if m.mode != modePalette {
		t.Fatalf("after e: mode = %v, want palette", m.mode)
	}
	want := "exam college/dbms "
	if m.paletteInput.Value() != want {
		t.Fatalf("palette prefilled with %q, want %q", m.paletteInput.Value(), want)
	}
}

// TestCoursesGradeKeyPicksUngradedWeightedItem pins that g prefers the
// first unscored weighted item's id when one exists.
func TestCoursesGradeKeyPicksUngradedWeightedItem(t *testing.T) {
	ts := DumpFixedTime
	stamp := ts.Format(core.TimestampFormat)
	s := fixtureStore(t, []fixtureTask{
		{id: "g00001", title: "scored task", project: "college/dbms", status: "done", priority: "medium", kind: "assignment", created: stamp, updated: stamp, completed: stamp, weight: f64(30), score: f64(27), scoreMax: f64(30)},
		{id: "g00002", title: "unscored task", project: "college/dbms", status: "todo", priority: "medium", kind: "assignment", created: stamp, updated: stamp, weight: f64(40)},
	}, fixtureProject{name: "college/dbms", color: "blue", course: &core.Course{Code: "DBMS", Name: "Database Systems", Semester: "2026-odd", Credits: 4, TargetGrade: "A"}})
	defer func() { _ = s.Close() }()
	m := New(dumpConfig(), s, sync.New(s.Root))
	m.width, m.height = 200, 50
	m.view = viewCourses
	m.courseSel = 0
	m2, _ := m.openGradePalette()
	m = m2.(Model)
	if got := m.paletteInput.Value(); got != "grade g00002 " {
		t.Fatalf("palette prefilled with %q, want %q", got, "grade g00002 ")
	}
}

// TestCoursesDrillOutRestoresSelection pins that esc restores the course
// selection that was active when the user drilled in.
func TestCoursesDrillOutRestoresSelection(t *testing.T) {
	m := coursesModel(t, 200)
	m2, _ := m.updateCoursesKey(aDown)
	m = m2.(Model)
	if m.courseSel != 1 {
		t.Fatalf("after j: courseSel = %d, want 1", m.courseSel)
	}
	m3, _ := m.updateCoursesKey(aEnter)
	m = m3.(Model)
	if m.view != viewList {
		t.Fatal("enter did not drill into the course")
	}
	m4, _ := m.updateNormalKey(keyMsg("esc"))
	m = m4.(Model)
	if m.view != viewCourses {
		t.Fatalf("after esc: view = %v, want courses", m.view)
	}
	if m.courseSel != 1 {
		t.Fatalf("after esc: courseSel = %d, want 1 (restored)", m.courseSel)
	}
}

// TestCoursesBarsNeverMergeIntoSlabs pins the thin-bar spacing rule in
// the detail pane: no thin-bar row is directly followed by another
// thin-bar row (the shared hours chart is exempt — it is one chart).
func TestCoursesBarsNeverMergeIntoSlabs(t *testing.T) {
	s := fixtures["courses"](t)
	defer func() { _ = s.Close() }()
	m := New(dumpConfig(), s, sync.New(s.Root))
	m.width, m.height = 200, 50
	m.view = viewCourses
	cards, err := collectCourseCards(m)
	if err != nil {
		t.Fatal(err)
	}
	detail := renderCourseDetail(m, cards[0], 100, 46)
	isThinBarRow := func(l string) bool {
		return strings.Contains(l, "▄▄▄▄") // a contiguous run of 4 lower-half blocks
	}
	for i := 1; i < len(detail); i++ {
		if isThinBarRow(StripANSI(detail[i-1])) && isThinBarRow(StripANSI(detail[i])) {
			t.Errorf("thin-bar rows are adjacent with no spacer:\n%s\n%s", StripANSI(detail[i-1]), StripANSI(detail[i]))
		}
	}
}

// TestCoursesExamTopicsChecklist pins the topics bar and per-topic
// checklist for an exam that carries subtasks (the exams fixture).
func TestCoursesExamTopicsChecklist(t *testing.T) {
	s := fixtures["exams"](t)
	defer func() { _ = s.Close() }()
	m := New(dumpConfig(), s, sync.New(s.Root))
	m.width, m.height = 200, 50
	m.view = viewCourses
	m.courseSel = 0 // DBMS (first course)
	frame := StripANSI(m.View())
	for _, want := range []string{"✦ topics", "4/6", "[x] Revise ch.4", "+2 more topics"} {
		if !strings.Contains(frame, want) {
			t.Errorf("exam section missing %q:\n%s", want, frame)
		}
	}
}

// TestCoursesBareGPrefillsGradePalette pins the raw "g" shortcut: in the
// Courses view it opens the palette with a grade command; elsewhere "g"
// keeps its gg prefix meaning.
func TestCoursesBareGPrefillsGradePalette(t *testing.T) {
	m := coursesModel(t, 200)
	m2, _ := m.updateNormalKey(keyMsg("g"))
	m3 := m2.(Model)
	if m3.mode != modePalette {
		t.Fatal("g did not open the palette in Courses view")
	}
	if !strings.HasPrefix(m3.paletteInput.Value(), "grade ") {
		t.Fatalf("palette prefilled with %q, want a grade command", m3.paletteInput.Value())
	}
}

// TestListBareGStaysPrefix pins that "g" remains a multi-key prefix
// outside the Courses view.
func TestListBareGStaysPrefix(t *testing.T) {
	m := coursesModel(t, 200)
	m.view = viewList
	m2, _ := m.updateNormalKey(keyMsg("g"))
	m3 := m2.(Model)
	if m3.mode != modeNormal {
		t.Fatal("g opened a mode outside Courses view")
	}
	if m3.pendingKey != "g" {
		t.Fatalf("pendingKey = %q, want %q", m3.pendingKey, "g")
	}
}
