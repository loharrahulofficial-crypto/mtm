package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestGuideSectionsContent(t *testing.T) {
	secs := guideSections()
	if len(secs) < 8 {
		t.Fatalf("only %d sections, want a comprehensive guide", len(secs))
	}
	titles := map[string]bool{}
	for _, s := range secs {
		if s.title == "" || len(s.lines) == 0 {
			t.Errorf("section %q empty or untitled", s.title)
		}
		titles[s.title] = true
	}
	for _, want := range []string{"Welcome to MTM", "quick-add syntax", "Pomodoro", "Improving your daily life", "Keyboard reference", "daily life"} {
		found := false
		for t := range titles {
			if strings.Contains(t, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("missing section %q (have: %v)", want, titles)
		}
	}
	// A guide that teaches daily life should mention heatmap/Pomodoro habits.
	all := guideLines()
	joined := strings.ToLower(strings.Join(all, "\n"))
	for _, k := range []string{"welcome to mtm", "quick-add", "mtm init", "pomodoro", "heatmap", "courses", ":help", "export", "mtm review", "esc or q", "focus mode"} {
		if !strings.Contains(joined, k) {
			t.Errorf("guide text missing %q", k)
		}
	}
}

func TestGuideOpenAndClose(t *testing.T) {
	m := newGuideTestModel(t)
	m.openGuide()
	if m.mode != modeGuide {
		t.Fatal("openGuide did not enter modeGuide")
	}
	for _, k := range []string{"esc", "q"} {
		u, _ := m.Update(keyMsg(k))
		m = asModel(u)
		if m.mode == modeGuide {
			t.Fatalf("%s should close the guide", k)
		}
	}
}

func TestGuideScrollKeys(t *testing.T) {
	m := newGuideTestModel(t)
	m.openGuide()
	apply := func(k string) Model {
		u, _ := m.Update(keyMsg(k))
		return asModel(u)
	}
	m = apply("j")
	if m.guide.Scroll <= 0 {
		t.Fatal("j did not scroll down")
	}
	start := m.guide.Scroll
	m = apply("k")
	if m.guide.Scroll >= start {
		t.Fatal("k did not scroll up")
	}
	m = apply("G") // jumps to end
	if m.guide.Scroll < guideLineCount()/2 {
		t.Fatal("G should jump near the end")
	}
	m = apply("0") // resets to top
	if m.guide.Scroll != 0 {
		t.Fatal("0 should scroll to the top")
	}
	// Never go negative.
	for i := 0; i < 50; i++ {
		m = apply("k")
	}
	if m.guide.Scroll != 0 {
		t.Fatal("scroll can go negative")
	}
}

func TestGuidePaletteAndCommand(t *testing.T) {
	m := newGuideTestModel(t)
	if !knownCommands["guide"] {
		t.Fatal("guide missing from knownCommands")
	}
	// execCommand("guide") opens it.
	u, _ := m.execCommand("guide")
	if u.(Model).mode != modeGuide {
		t.Fatal(":guide should open the guide")
	}
	// The palette offers the entry.
	m2 := newGuideTestModel(t)
	m2.mode = modePalette
	m2.paletteInput.SetValue("guide")
	m2.paletteItems = paletteItemsFor(m2, "guide")
	if !hasPaletteItem(m2.paletteItems, "guide") {
		t.Fatal("no palette item with id 'guide'")
	}
}

func hasPaletteItem(items []paletteItem, id string) bool {
	for _, it := range items {
		if it.id == id {
			return true
		}
	}
	return false
}

func newGuideTestModel(t *testing.T) Model {
	t.Helper()
	m, _ := newTestModel(t, nil)
	return m
}

func asModel(u tea.Model) Model {
	if v, ok := u.(Model); ok {
		return v
	}
	return *u.(*Model)
}
