package tui

import (
	"testing"
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
