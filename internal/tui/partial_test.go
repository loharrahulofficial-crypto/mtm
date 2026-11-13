package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/clock"
	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

// Item 3: an ambient tick repaints only the lines of the animated window.
// Outside the window rows the frame is byte-identical, and no non-scene
// panel re-renders (their cache keys are untouched).
func TestAmbientTickChangesOnlyWindowLines(t *testing.T) {
	m, s := newTestModel(t, []*core.Task{
		testTask("revise ch. 6", core.KindStudy),
	})
	defer func() { _ = s.Close() }()
	m.cfg.UI.Ambient = "both"
	m.cfg.UI.AmbientSidebar = true
	m.cfg.UI.Scene = "night"
	m.view = viewHome

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	frame1 := m.View()

	mainKey := m.panels.main.key
	detailKey := m.panels.detail.key
	footerKey := m.panels.footer.key
	sidebarKey := m.panels.sidebar.key

	// Advance the scene clock and tick.
	m2 := m
	m2.clock = clock.Fixed(m.clock.Now().Add(900 * time.Millisecond))
	updated, _ = m2.Update(ambientTickMsg{})
	m = updated.(Model)
	frame2 := m.View()

	if frame1 == frame2 {
		t.Fatal("ambient tick produced an identical frame — the scene did not advance?")
	}

	// The non-scene panels must not have re-rendered at all.
	if m.panels.main.key != mainKey {
		t.Error("main pane re-rendered on an ambient tick (want cache hit)")
	}
	if m.panels.detail.key != detailKey {
		t.Error("detail pane re-rendered on an ambient tick (want cache hit)")
	}
	if m.panels.footer.key != footerKey {
		t.Error("footer re-rendered on an ambient tick (want cache hit)")
	}
	if m.panels.sidebar.key != sidebarKey {
		t.Error("sidebar static rows re-rendered on an ambient tick (want cache hit)")
	}

	// Row-level: only the window rows may differ, and they must differ.
	w := sideWidth(m.width, m.sidebarHidden)
	contentH := m.height - 2
	innerW := w - 2
	innerH := contentH - 2
	_, rendered, _, winH := sidebarRowPlan(m, innerW, innerH)
	if winH < ambientSidebarMinRows {
		t.Fatalf("window does not fit (winH=%d) — fixture broken", winH)
	}
	// Frame rows: 1 border + 2 brand + rendered item rows → window.
	winTop := 1 + brandLines + rendered
	winBot := winTop + winH

	lines1 := strings.Split(frame1, "\n")
	lines2 := strings.Split(frame2, "\n")
	if len(lines1) != len(lines2) {
		t.Fatalf("frame height changed: %d vs %d", len(lines1), len(lines2))
	}
	changedOutside := []int{}
	changedInside := 0
	for i := range lines1 {
		if lines1[i] == lines2[i] {
			continue
		}
		if i < winTop || i >= winBot {
			changedOutside = append(changedOutside, i)
		} else {
			changedInside++
		}
	}
	if len(changedOutside) > 0 {
		t.Fatalf("ambient tick changed lines outside the window (rows %v; window rows %d..%d)",
			changedOutside, winTop, winBot-1)
	}
	if changedInside == 0 {
		t.Fatalf("ambient tick changed no line inside the window — frame diff empty?")
	}
}
