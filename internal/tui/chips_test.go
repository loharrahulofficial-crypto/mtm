package tui

import (
	"strings"
	"testing"

	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

func TestListFilterChipsParseTokens(t *testing.T) {
	m := listModelWithQuery(t, "priority:high tag:exam read ch")
	got := listFilterChips(m, 0)
	joined := strings.Join(got, "|")
	for _, want := range []string{"priority:high", "#exam", "“read ch”"} {
		if !strings.Contains(joined, want) {
			t.Errorf("chips %v missing %q", got, want)
		}
	}
}

func TestListFilterChipsFreeTextOnly(t *testing.T) {
	m := listModelWithQuery(t, "normalization")
	got := listFilterChips(m, 0)
	if len(got) != 1 || !strings.Contains(got[0], "normalization") {
		t.Fatalf("chips = %v, want one free-text chip", got)
	}
}

func TestListFilterChipsCollapseOverflow(t *testing.T) {
	m := listModelWithQuery(t, "priority:high project:college/dbms tag:exam status:todo due:today")
	got := listFilterChips(m, 24)
	last := got[len(got)-1]
	if !strings.HasPrefix(last, "+") {
		t.Fatalf("chips = %v, want a +N collapse chip at the end", got)
	}
}

func TestListFilterChipsEmpty(t *testing.T) {
	m := listModelWithQuery(t, "")
	if got := listFilterChips(m, 0); len(got) != 0 {
		t.Fatalf("chips = %v, want none for empty query", got)
	}
}

func listModelWithQuery(t *testing.T, query string) Model {
	t.Helper()
	s := twoTasksFixture(t)
	t.Cleanup(func() { _ = s.Close() })
	m := New(dumpConfig(), s, sync.New(s.Root))
	m.width, m.height = 120, 40
	m.view = viewList
	m.searchQuery = query
	m.refreshTasks()
	return m
}

// TestListPanelRendersChipsAndTitle pins that the list panel title carries
// the view label and the active filter chips sit next to it.
func TestListPanelRendersChipsAndTitle(t *testing.T) {
	m := listModelWithQuery(t, "priority:high")
	frame := strings.ReplaceAll(StripANSI(m.View()), "\u2009", " ")
	for _, want := range []string{"priority:high", "T A S K S", "All"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("frame missing %q:\n%s", want, frame)
		}
	}
}

// TestCalendarPanelTitle pins the calendar panel border title.
func TestCalendarPanelTitle(t *testing.T) {
	s := fixtures["populated"](t)
	defer func() { _ = s.Close() }()
	frame, err := RenderDump(dumpConfig(), s, sync.New(s.Root), "calendar", 120, 40, true, DumpFixedTime)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ReplaceAll(StripANSI(frame), "\u2009", " "), "C A L E N D A R") {
		t.Fatalf("calendar frame missing its panel title:\n%s", frame)
	}
}
