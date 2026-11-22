package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// guideBodyText is every line the guide renders (titles, section text, keys).
func guideBodyText(secs []guideSection) []string {
	var out []string
	for _, s := range secs {
		out = append(out, s.title)
		out = append(out, s.lines...)
		for _, g := range s.groups {
			out = append(out, g.name)
			for _, r := range g.rows {
				out = append(out, r.keys, r.desc)
			}
		}
	}
	return out
}

func TestGuideSectionsContent(t *testing.T) {
	secs := guideSections()
	if len(secs) < 8 {
		t.Fatalf("only %d sections, want a comprehensive guide", len(secs))
	}
	titles := map[string]bool{}
	for _, s := range secs {
		if s.title == "" || (len(s.lines) == 0 && s.groups == nil) {
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
	// The keyboard section is an interactive grid (grouped key/desc rows).
	ks := secs[len(secs)-1]
	if ks.title != "Keyboard reference" || len(ks.groups) == 0 {
		t.Fatalf("last section = %q groups=%d, want Keyboard reference with groups", ks.title, len(ks.groups))
	}
	for _, g := range ks.groups {
		if strings.TrimSpace(g.name) == "" {
			t.Error("keyboard group without a name")
		}
		for _, r := range g.rows {
			if strings.TrimSpace(r.keys) == "" || strings.TrimSpace(r.desc) == "" {
				t.Errorf("keyboard row with empty keys or desc: %+v", r)
			}
		}
	}
	// A guide that teaches daily life should mention heatmap/Pomodoro habits.
	all := guideBodyText(secs)
	joined := strings.ToLower(strings.Join(all, "\n"))
	for _, k := range []string{"welcome to mtm", "quick-add", "mtm init", "pomodoro", "heatmap", "courses", ":help", "export", "mtm review", "esc", "focus mode", "7", "ideas"} {
		if !strings.Contains(joined, k) {
			t.Errorf("guide text missing %q", k)
		}
	}
}

func TestGuideOpenAndClose(t *testing.T) {
	m := newGuideTestModel(t)
	m.openGuide()
	if m.view != viewGuide {
		t.Fatal("openGuide did not enter viewGuide")
	}
	for _, k := range []string{"esc", "q"} {
		u, _ := m.Update(keyMsg(k))
		m = asModel(u)
		if m.view == viewGuide {
			t.Fatalf("%s should close the guide", k)
		}
	}
}

func TestGuideSectionSwitch(t *testing.T) {
	m := newGuideTestModel(t)
	m.openGuide()
	doc, starts := m.guideDocLines(m.guideContentWidth())
	maxStart := len(doc) - m.guideRoom()
	if maxStart < 0 {
		maxStart = 0
	}
	wantScroll := func(idx int) int { return clampInt(starts[idx], 0, maxStart) }

	m.guide.Scroll = 3
	u, _ := m.Update(keyMsg("7"))
	m = asModel(u)
	if m.view != viewGuide || m.guide.Section != 6 || m.guide.Scroll != wantScroll(6) {
		t.Fatalf("key 7: view=%v section=%d scroll=%d, want section 6, scroll %d", m.view, m.guide.Section, m.guide.Scroll, wantScroll(6))
	}
	u, _ = m.Update(keyMsg("h"))
	m = asModel(u)
	if m.guide.Section != 5 || m.guide.Scroll != wantScroll(5) {
		t.Fatalf("h from section 6 -> section=%d scroll=%d, want 5/%d", m.guide.Section, m.guide.Scroll, wantScroll(5))
	}
	// Key 1 jumps to the first section.
	u, _ = m.Update(keyMsg("1"))
	m = asModel(u)
	if m.guide.Section != 0 || m.guide.Scroll != 0 {
		t.Fatalf("key 1 -> section=%d scroll=%d, want section 0 scroll 0", m.guide.Section, m.guide.Scroll)
	}
	u, _ = m.Update(keyMsg("h"))
	m = asModel(u)
	if m.guide.Section != 0 {
		t.Fatalf("h at section 0 -> %d, want 0", m.guide.Section)
	}
	// Right cycles through sections.
	u, _ = m.Update(keyMsg("l"))
	m = asModel(u)
	if m.guide.Section != 1 || m.guide.Scroll != wantScroll(1) {
		t.Fatalf("l at section 0 -> section=%d scroll=%d, want 1/%d", m.guide.Section, m.guide.Scroll, wantScroll(1))
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
	m = apply("G") // jumps to end of the section body
	if m.guide.Scroll <= 0 {
		t.Fatal("G should move scroll forward")
	}
	m = apply("g") // jumps to top of the section
	if m.guide.Scroll != 0 {
		t.Fatal("g should scroll to the top")
	}
	// Never go negative.
	for i := 0; i < 50; i++ {
		m = apply("k")
	}
	if m.guide.Scroll != 0 {
		t.Fatal("scroll can go negative")
	}
}

func TestGuideKeyBadges(t *testing.T) {
	m := newGuideTestModel(t)
	lines := m.guideText("  `a`  add task", 60)
	got := strings.Join(lines, "\n")
	if !strings.Contains(got, "a") || strings.Contains(got, "`a`") {
		t.Fatalf("guideText did not render code spans as badges: %q", got)
	}
}

func TestGuideTextWraps(t *testing.T) {
	m := newGuideTestModel(t)
	long := "  MTM is your terminal task manager: keyboard-first, Pomodoro-native, with a calendar, a kanban board, courses/exams, and a heatmap so you can see your rhythm."
	lines := m.guideText(long, 50)
	if len(lines) < 2 {
		t.Fatalf("long copy should wrap, got %d line(s)", len(lines))
	}
	for _, l := range lines {
		if w := fastWidth(l); w > 50 {
			t.Errorf("wrapped line is %d cells, want <= 50: %q", w, l)
		}
		if !strings.HasPrefix(l, "  ") {
			t.Errorf("continuation lost its indent: %q", l)
		}
	}
	// Key badges stay whole across a wrap.
	lines = m.guideText("  run `mtm add \"title !priority\"` now", 24)
	joined := strings.Join(lines, "")
	if strings.Contains(joined, "`") {
		t.Fatalf("backticks leaked into wrapped output: %q", joined)
	}
}

func TestGuideClipAddsEllipsis(t *testing.T) {
	got := guideClip("MTM is your terminal task manager: keyboard-first, Pomodoro-native, with a calendar, a kanban board, courses/exams, and a heatmap so you can see your rhythm.", 40)
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("guideClip should append an ellipsis when truncating, got %q", got)
	}
	if w := fastWidth(got); w > 41 {
		t.Fatalf("guideClip produced %d cells, want <= 41", w)
	}
	if guideClip("short line", 40) != "short line" {
		t.Fatal("guideClip must pass through short lines unchanged")
	}
}

func TestGuideHelpOverlayDoesNotScrollGuide(t *testing.T) {
	m := newGuideTestModel(t)
	m.openGuide()
	m.guide.Scroll = 5
	u, _ := m.Update(keyMsg("?"))
	m = asModel(u)
	if m.mode != modeHelp {
		t.Fatalf("? should open help, mode=%v", m.mode)
	}
	before := m.guide.Scroll
	u, _ = m.Update(keyMsg("j"))
	m = asModel(u)
	if m.guide.Scroll != before {
		t.Fatal("guide scrolled under the help overlay")
	}
	u, _ = m.Update(keyMsg("esc"))
	m = asModel(u)
	if m.mode != modeNormal || m.view != viewGuide {
		t.Fatalf("closing help: mode=%v view=%v, want normal/guide", m.mode, m.view)
	}
}

func TestGuideChipHit(t *testing.T) {
	m := newGuideTestModel(t)
	m.width = 120
	w := m.guideContentWidth()
	if len(m.guideChips(w)) != 1 {
		t.Fatalf("chips should fit on one row at 120 cols, got %d rows", len(m.guideChips(w)))
	}
	// Walk the row and find the cell that opens section 3.
	found := false
	for x := 0; x < w; x++ {
		if idx, ok := m.guideChipHit(x, 0, w); ok && idx == 2 {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("no chip cell maps to section 3")
	}
	if _, ok := m.guideChipHit(0, 9, w); ok {
		t.Fatal("a row past the chip block must not hit a chip")
	}
}

func TestGuideDocIsContinuous(t *testing.T) {
	m := newGuideTestModel(t)
	m.width, m.height = 120, 40
	doc, starts := m.guideDocLines(m.guideContentWidth())
	if len(starts) != len(guideSections()) {
		t.Fatalf("got %d section starts, want %d", len(starts), len(guideSections()))
	}
	for i := 1; i < len(starts); i++ {
		if starts[i] <= starts[i-1] {
			t.Fatalf("section starts not increasing: %v", starts)
		}
	}
	joined := strings.Join(doc, "\n")
	for _, sec := range guideSections() {
		if !strings.Contains(joined, strings.ToUpper(sec.title)) {
			t.Errorf("continuous doc missing section %q", sec.title)
		}
	}
	if len(doc) <= m.guideRoom() {
		t.Fatalf("document (%d rows) should exceed one screen (%d)", len(doc), m.guideRoom())
	}
}

func TestGuideScrollRowsFitPane(t *testing.T) {
	m := newGuideTestModel(t)
	m.width, m.height = 100, 30
	w := m.guideContentWidth()
	doc, _ := m.guideDocLines(w)
	room := m.guideRoom()
	rows := m.guideScrollRows(doc, 0, room, w)
	if len(rows) != room {
		t.Fatalf("scroll rows = %d, want %d", len(rows), room)
	}
	for i, r := range rows {
		if got := fastWidth(r); got != w+2 {
			t.Fatalf("row %d is %d cells, want %d (text %d + gutter 2)", i, got, w+2, w)
		}
	}
}

func TestGuidePaneFitsAndShowsKeys(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}, {200, 50}} {
		m := newGuideTestModel(t)
		m.width, m.height = size[0], size[1]
		m.openGuide()
		m.guideJump(len(guideSections()) - 1) // keyboard reference
		pane := m.renderGuidePane(m.width, m.height-2, true)
		lines := strings.Split(pane, "\n")
		if len(lines) != m.height-2 {
			t.Fatalf("%dx%d: pane has %d rows, want %d", size[0], size[1], len(lines), m.height-2)
		}
		for i, l := range lines {
			if got := fastWidth(l); got != m.width {
				t.Fatalf("%dx%d: row %d is %d cells, want %d", size[0], size[1], i, got, m.width)
			}
		}
		plain := ansi.Strip(pane)
		for _, want := range []string{"KEYBOARD REFERENCE", "VIEWS", "j/k"} {
			if !strings.Contains(plain, want) {
				t.Errorf("%dx%d: keyboard section missing %q", size[0], size[1], want)
			}
		}
		// The tail of the section (SYSTEM group) is reachable by scrolling.
		m.guide.Scroll = 1 << 30
		m.guideSync()
		tail := ansi.Strip(m.renderGuidePane(m.width, m.height-2, true))
		for _, want := range []string{"SYSTEM", "?", "9/9"} {
			if !strings.Contains(tail, want) {
				t.Errorf("%dx%d: keyboard tail missing %q", size[0], size[1], want)
			}
		}
	}
}

func TestGuideShortSectionsCarryTip(t *testing.T) {
	secs := guideSections()
	if secs[0].tip == "" {
		t.Fatal("Welcome section missing its tip")
	}
	lines := newGuideTestModel(t).guideSectionLines(secs[0], 80)
	found := false
	for _, l := range lines {
		if strings.Contains(ansi.Strip(l), secs[0].tip) {
			found = true
		}
	}
	if !found {
		t.Fatalf("Welcome section body missing tip %q (lines: %v)", secs[0].tip, lines)
	}
}

func TestGuideKeyGridLaysOutColumns(t *testing.T) {
	m := newGuideTestModel(t)
	m.guide.Section = len(guideSections()) - 1
	body := m.guideSectionLines(guideSections()[len(guideSections())-1], 60)
	joined := strings.Join(body, "\n")
	for _, want := range []string{"VIEWS", "0-7", "tab", "MOVE", "j/k", "TASKS", "SYSTEM", "?"} {
		if !strings.Contains(joined, want) {
			t.Errorf("guide key grid missing %q", want)
		}
	}
}

func TestGuidePaletteAndCommand(t *testing.T) {
	m := newGuideTestModel(t)
	if !knownCommands["guide"] {
		t.Fatal("guide missing from knownCommands")
	}
	// execCommand("guide") opens it.
	u, _ := m.execCommand("guide")
	if u.(Model).view != viewGuide {
		t.Fatal(":guide should open the guide tab")
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
