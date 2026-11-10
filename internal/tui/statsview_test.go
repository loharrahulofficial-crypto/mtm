package tui

import (
	"strings"
	"testing"

	"github.com/loharrahulofficial-crypto/mtm/internal/sync"
)

// Spec 0b.1 (scrollable like the Courses pane) applies to the Stats
// details too: ctrl+d/ctrl+u and the wheel scroll; ▲/▼ more indicators
// mark clipped edges; sections stay in fixed order.
func TestStatsDetailScrollReachesBottom(t *testing.T) {
	s := fixtures["exams"](t)
	defer func() { _ = s.Close() }()
	m := BuildDumpModel(dumpConfig(), s, sync.New(s.Root), "stats", 120, 40, DumpFixedTime)
	m.focus = focusMain
	bottom := m.statsScroll
	frames := 0
	for i := 0; i < 40; i++ {
		updated, _ := m.Update(keyMsg("ctrl+d"))
		m = updated.(Model)
		frames++
		if m.statsScroll == bottom {
			break
		}
		bottom = m.statsScroll
	}
	if bottom == 0 {
		t.Fatal("ctrl+d did not scroll the stats detail")
	}
	frame := StripANSI(m.View())
	if strings.Contains(frame, "▼ more") {
		t.Fatalf("at the bottom the ▼ more indicator is still shown:\n%s", frame)
	}
	// Back up to the top.
	for i := 0; i < 40; i++ {
		updated, _ := m.Update(keyMsg("ctrl+u"))
		m = updated.(Model)
		if m.statsScroll == 0 {
			break
		}
	}
	if m.statsScroll != 0 {
		t.Fatalf("ctrl+u did not return to the top (scroll=%d)", m.statsScroll)
	}
	frame = StripANSI(m.View())
	if strings.Contains(frame, "▲ more") {
		t.Fatalf("at the top the ▲ more indicator is still shown:\n%s", frame)
	}
}

// The semester scope is greyed out with a hint when ui.semester_start
// is not configured.
func TestStatsSemesterHintWhenUnset(t *testing.T) {
	s := fixtures["populated"](t)
	defer func() { _ = s.Close() }()
	m := BuildDumpModel(dumpConfig(), s, sync.New(s.Root), "stats", 120, 40, DumpFixedTime)
	m.cfg.UI.SemesterStart = ""
	// Select THIS SEMESTER (index 2).
	m.statsSel = statsScopeSemester
	frame := StripANSI(strings.Join(renderStatsDetail(m, 90, 40), "\n"))
	if !strings.Contains(frame, "ui.semester_start") {
		t.Fatalf("semester scope with unset anchor should show the config hint:\n%s", frame)
	}
}

// The scope list renders and the counts are per-scope.
func TestStatsScopeSelection(t *testing.T) {
	s := fixtures["exams"](t)
	defer func() { _ = s.Close() }()
	m := BuildDumpModel(dumpConfig(), s, sync.New(s.Root), "stats", 120, 40, DumpFixedTime)
	if m.statsSel != 0 {
		t.Fatalf("statsSel = %d, want 0 (THIS WEEK)", m.statsSel)
	}
	m2, _ := m.updateStatsKey(aDown)
	m = m2.(Model)
	m2, _ = m.updateStatsKey(aDown)
	m = m2.(Model)
	if m.statsSel != 2 {
		t.Fatalf("statsSel = %d after two downs, want 2", m.statsSel)
	}
	// Selecting wraps/clamps at the bounds.
	for i := 0; i < 10; i++ {
		m2, _ = m.updateStatsKey(aDown)
		m = m2.(Model)
	}
	if m.statsSel != statsScopeCount-1 {
		t.Fatalf("statsSel = %d after many downs, want %d", m.statsSel, statsScopeCount-1)
	}
}
