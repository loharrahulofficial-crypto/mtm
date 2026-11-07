package tui

import (
	"strings"
	"testing"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

func TestFuzzyScore(t *testing.T) {
	if _, ok := fuzzyScore("abc", "acb"); ok {
		t.Fatal("out-of-order subsequence matched")
	}
	if _, ok := fuzzyScore("abc", "abc"); !ok {
		t.Fatal("perfect subsequence rejected")
	}
	if _, ok := fuzzyScore("abc", "aXbc"); !ok {
		t.Fatal("gapped subsequence rejected")
	}
	contiguous, _ := fuzzyScore("ab", "abc")
	gapped, _ := fuzzyScore("ab", "aXb")
	if contiguous <= gapped {
		t.Errorf("contiguous score %d <= gapped %d", contiguous, gapped)
	}
}

func TestPaletteMatchEmptyQueryRecentsFirst(t *testing.T) {
	items := []paletteItem{
		{id: "a", label: "Alpha", group: "Commands"},
		{id: "b", label: "Beta", group: "Commands"},
		{id: "c", label: "Gamma", group: "Commands"},
	}
	got := paletteMatch(items, "", []string{"c", "a"})
	if len(got) != 3 {
		t.Fatalf("got %d items, want 3", len(got))
	}
	if got[0].id != "c" || got[1].id != "a" || got[2].id != "b" {
		t.Fatalf("recents not first: %v", []string{got[0].id, got[1].id, got[2].id})
	}
}

func TestPaletteMatchFuzzyFiltersAndRanks(t *testing.T) {
	items := []paletteItem{
		{id: "view:list", label: "Go to List", group: "Commands"},
		{id: "view:calendar", label: "Go to Calendar", group: "Commands"},
		{id: "task:zzz", label: "Submit problem set", group: "Tasks"},
	}
	got := paletteMatch(items, "cal", nil)
	if len(got) != 1 || got[0].id != "view:calendar" {
		t.Fatalf("cal matched: %+v", got)
	}
	got = paletteMatch(items, "submit", nil)
	if len(got) != 1 || got[0].id != "task:zzz" {
		t.Fatalf("submit matched: %+v", got)
	}
	got = paletteMatch(items, "zzzqqq", nil)
	if len(got) != 0 {
		t.Fatalf("bogus query matched: %+v", got)
	}
}

func TestPaletteRecentsCapAndOrder(t *testing.T) {
	recents := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	sel := "i"
	recents = append([]string{sel}, filterOut(recents, sel)...)
	if len(recents) > 9 {
		t.Fatalf("recents = %d, want <= 9", len(recents))
	}
	if recents[0] != "i" {
		t.Fatalf("most recent = %q", recents[0])
	}
}

func TestPaletteArgvDetection(t *testing.T) {
	m := Model{}
	m.paletteInput.Prompt = ""
	m.paletteInput.CharLimit = 128
	m.paletteInput.SetValue("done")
	if !paletteCommandArgv(m) {
		t.Fatal("'done' not detected as argv command")
	}
	m.paletteInput.SetValue("exam college/dbms DBMS midterm 2026-11-19")
	if !paletteCommandArgv(m) {
		t.Fatal("'exam …' not detected as argv command")
	}
	m.paletteInput.SetValue("Done ABC123")
	if !paletteCommandArgv(m) {
		t.Fatal("'Done ABC123' not detected as argv command")
	}
	m.paletteInput.SetValue("do")
	if paletteCommandArgv(m) {
		t.Fatal("'do' wrongly detected as argv command")
	}
	m.paletteInput.SetValue("done!")
	if paletteCommandArgv(m) {
		t.Fatal("'done!' wrongly detected as argv command")
	}
}

func TestExamCourseSuggestions(t *testing.T) {
	m, s := newTestModel(t, nil)
	if err := s.SaveProject(core.DefaultProject("college/dbms")); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveProject(core.DefaultProject("college/os")); err != nil {
		t.Fatal(err)
	}

	// Bare "exam" lists all courses.
	sugg := examCourseSuggestions(m, "exam")
	if len(sugg) != 3 { // inbox + two courses
		t.Fatalf("bare 'exam' suggestions = %d, want 3", len(sugg))
	}
	for _, it := range sugg {
		if !strings.HasPrefix(it.id, "examcourse:") {
			t.Fatalf("suggestion id %q missing examcourse: prefix", it.id)
		}
	}
	// Partial course token filters to a match.
	sugg = examCourseSuggestions(m, "exam dbms")
	if len(sugg) != 1 || sugg[0].id != "examcourse:college/dbms" {
		t.Fatalf("'exam dbms' suggestions = %+v, want [college/dbms]", sugg)
	}
	// A fuzzy subsequence match works too ("cs" ⊂ "college/os").
	sugg = examCourseSuggestions(m, "exam col/os")
	if len(sugg) != 1 || sugg[0].id != "examcourse:college/os" {
		t.Fatalf("'exam col/os' suggestions = %+v, want [college/os]", sugg)
	}
	// Past the course token: no suggestions, plain fuzzy/argv takes over.
	if sugg := examCourseSuggestions(m, "exam college/dbms DBMS midterm friday"); sugg != nil {
		t.Fatalf("full command query returned suggestions: %+v", sugg)
	}
	// paletteItemsFor routes exam queries to suggestions.
	items := paletteItemsFor(m, "exam db")
	if len(items) == 0 || !strings.HasPrefix(items[0].id, "examcourse:") {
		t.Fatalf("paletteItemsFor('exam db') = %+v, want course suggestions", items)
	}
	// A non-exam query gets normal items.
	items = paletteItemsFor(m, "home")
	foundHome := false
	for _, it := range items {
		if it.id == "view:home" {
			foundHome = true
		}
	}
	if !foundHome {
		t.Fatal("paletteItemsFor('home') should offer the Go to Home command")
	}
}

func TestExamCoursePalettePickFillsInput(t *testing.T) {
	m, s := newTestModel(t, nil)
	if err := s.SaveProject(core.DefaultProject("college/dbms")); err != nil {
		t.Fatal(err)
	}
	upd := m.openPalette()
	_ = upd
	m.paletteInput.SetValue("exam db")
	m.paletteQuery = "exam db"
	m.paletteItems = paletteItemsFor(m, m.paletteQuery)
	m.paletteSel = 0
	out, _ := m.updatePaletteKey(keyMsg("enter"))
	m2 := out.(Model)
	if m2.mode != modePalette {
		t.Fatal("picking a course should keep the palette open")
	}
	if got := m2.paletteInput.Value(); got != "exam college/dbms " {
		t.Fatalf("input after course pick = %q, want %q", got, "exam college/dbms ")
	}
	// Typing the rest of the command and pressing enter runs it.
	m2.paletteInput.SetValue("exam college/dbms DBMS midterm 2026-11-19")
	m2.paletteQuery = m2.paletteInput.Value()
	m2.paletteItems = paletteItemsFor(m2, m2.paletteQuery)
	out, _ = m2.updatePaletteKey(keyMsg("enter"))
	m3 := out.(Model)
	if m3.mode == modePalette {
		t.Fatal("enter on a full exam command should close the palette")
	}
	if len(m3.store.Tasks()) != 1 {
		t.Fatal("exam was not created via the palette")
	}
}
