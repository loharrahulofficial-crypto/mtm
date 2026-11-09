package tui

import (
	"strings"
	"testing"

	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

// Spec 0b.5: every key shown in the help overlay must match the real
// keymap. This test diffs the rendered help text against the keymap in
// both directions: every binding is shown with its description, and every
// row in the overlay parses back to exactly one keymap binding whose
// description matches the keymap table — nothing stale, nothing missing.
func TestHelpOverlayMatchesKeymap(t *testing.T) {
	s := fixtures["populated"](t)
	defer func() { _ = s.Close() }()
	m := BuildDumpModel(dumpConfig(), s, sync.New(s.Root), "list", 120, 40, DumpFixedTime)

	bound := m.keymap.Binds()
	seqToAction := map[string]string{}
	for action, seq := range bound {
		seqToAction[seq] = action
	}

	help := StripANSI(renderHelpBox(m.keymap, m.styles, 120))

	// (a) every bound action is shown with its key and description
	for _, e := range HelpEntries(m.keymap) {
		if !strings.Contains(help, e[2]) {
			t.Errorf("help overlay missing key %q for action %s", e[2], e[0])
		}
		if !strings.Contains(help, e[1]) {
			t.Errorf("help overlay missing description %q for action %s", e[1], e[0])
		}
	}

	// (b) every row of the overlay parses to a keymap binding whose
	// description matches the keymap table exactly. The overlay draws a
	// rounded border with 2-cell padding, so side borders are dropped
	// before parsing.
	rows := 0
	for _, line := range strings.Split(help, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || !strings.HasPrefix(trimmed, "│") {
			continue // border edges and padding rows
		}
		inner := strings.TrimPrefix(trimmed, "│")
		inner = strings.TrimSuffix(inner, "│")
		inner = strings.TrimSpace(inner)
		if inner == "" || strings.EqualFold(inner, "KEYBINDINGS") {
			continue
		}
		if strings.HasPrefix(inner, "VIEW") || strings.HasPrefix(inner, "MOVE") ||
			strings.HasPrefix(inner, "FILTER") || strings.HasPrefix(inner, "TASK") ||
			strings.HasPrefix(inner, "NOTES") || strings.HasPrefix(inner, "SYNC") ||
			strings.HasPrefix(inner, "META") {
			continue
		}
		if strings.Contains(inner, "any key") || strings.Contains(inner, "config:") {
			continue
		}
		fields := strings.Fields(inner)
		if len(fields) < 2 {
			t.Errorf("unparsable help overlay line %q", inner)
			continue
		}
		seq := fields[0]
		desc := strings.Join(fields[1:], " ")
		action, ok := seqToAction[seq]
		if !ok {
			t.Errorf("help overlay shows key %q (%q) with no keymap binding (line %q)", seq, desc, trimmed)
			continue
		}
		if actionDescriptions[action] != desc {
			t.Errorf("help overlay line %q: description %q does not match keymap's %q for action %s",
				trimmed, desc, actionDescriptions[action], action)
		}
		rows++
	}
	if rows == 0 {
		t.Fatal("no keybinding rows parsed from the help overlay")
	}

	// (c) every bound action appears in actionOrder — the overlay only
	// renders actionOrder, so a binding missing from it is invisible
	inOrder := map[string]bool{}
	for _, a := range actionOrder {
		inOrder[a] = true
	}
	for action, seq := range bound {
		if !inOrder[action] {
			t.Errorf("action %q bound to %q is missing from actionOrder — invisible in the help overlay", action, seq)
		}
	}
}

// Spec 0b.5 (continued): the footer's view pills are labelled with the
// keymap's view bindings, not hardcoded digits.
func TestFooterPillsMatchKeymap(t *testing.T) {
	s := fixtures["populated"](t)
	defer func() { _ = s.Close() }()
	m := BuildDumpModel(dumpConfig(), s, sync.New(s.Root), "list", 120, 40, DumpFixedTime)
	pills := m.footerPillSegments(120)
	want := []string{
		m.keymap.Seq(aViewHome),
		m.keymap.Seq(aViewList),
		m.keymap.Seq(aViewBoard),
		m.keymap.Seq(aViewCalendar),
		m.keymap.Seq(aViewCourses),
	}
	if len(pills) != len(want) {
		t.Fatalf("pills = %d, want %d", len(pills), len(want))
	}
	for i, p := range pills {
		if !strings.Contains(StripANSI(p.rendered), want[i]) {
			t.Errorf("pill %d (view %d) missing keymap key %q: %q", i, p.view, want[i], StripANSI(p.rendered))
		}
	}
}

// Spec 0b.5 (known change): J/K navigate months on the calendar; H/L move
// cards on the board only and do not switch months.
func TestCalendarMonthKeysAreJK(t *testing.T) {
	if got := DefaultKeyMap().Seq(aMonthPrev); got != "J" {
		t.Errorf("month_prev = %q, want J", got)
	}
	if got := DefaultKeyMap().Seq(aMonthNext); got != "K" {
		t.Errorf("month_next = %q, want K", got)
	}
	m, _ := newTestModel(t, nil)
	m.view = viewCalendar
	before := m.calMonth
	m = press(t, m, "L") // the board binding must not switch months anymore
	if !m.calMonth.Equal(before) {
		t.Fatalf("L changed the month: %v → %v", before, m.calMonth)
	}
	m = press(t, m, "K") // K advances the month
	if got := m.calMonth; got.Year() != before.AddDate(0, 1, 0).Year() || got.Month() != before.AddDate(0, 1, 0).Month() {
		t.Fatalf("K did not advance the month: %v → %v", before, got)
	}
	m = press(t, m, "J")
	if got := m.calMonth; got.Year() != before.Year() || got.Month() != before.Month() {
		t.Fatalf("J did not go back a month: %v → %v", before, got)
	}
}
