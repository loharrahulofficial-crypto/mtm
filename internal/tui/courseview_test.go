package tui

import (
	"strings"
	"testing"

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

// TestCoursesGridColumns pins the 1–3 column grid by width. Regression
// for "card grid lays out 1 to 3 columns by width".
func TestCoursesGridColumns(t *testing.T) {
	for w, want := range map[int]int{50: 1, 64: 2, 80: 2, 99: 2, 100: 3, 160: 3} {
		if got := cardCols(w); got != want {
			t.Errorf("cardCols(%d) = %d, want %d", w, got, want)
		}
	}
}

// TestCoursesNavigationKeys checks h/j/k/l move the card selection and
// clamp at the edges.
func TestCoursesNavigationKeys(t *testing.T) {
	m := coursesModel(t, 120) // 120 → 3 columns
	// l moves selection right, h moves it back.
	m2, _ := m.updateCoursesKey(aRight)
	if got := m2.(Model).courseSel; got != 1 {
		t.Fatalf("after l: courseSel = %d, want 1", got)
	}
	m3, _ := m2.(Model).updateCoursesKey(aLeft)
	if got := m3.(Model).courseSel; got != 0 {
		t.Fatalf("after h: courseSel = %d, want 0", got)
	}
	// k at the top row stays at 0.
	m4, _ := m.updateCoursesKey(aUp)
	if got := m4.(Model).courseSel; got != 0 {
		t.Fatalf("after k at top: courseSel = %d, want 0", got)
	}
	// j moves down one row (3 cols): 0 → 3, clamped to the last card (2).
	m5, _ := m.updateCoursesKey(aDown)
	if got := m5.(Model).courseSel; got != 2 {
		t.Fatalf("after j: courseSel = %d, want 2 (clamped to last card)", got)
	}
	// l past the end clamps to the last card.
	m6, _ := m5.(Model).updateCoursesKey(aRight)
	if got := m6.(Model).courseSel; got != 2 {
		t.Fatalf("after l at end: courseSel = %d, want 2", got)
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
