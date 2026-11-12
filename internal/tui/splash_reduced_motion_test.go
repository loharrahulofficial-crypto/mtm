package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/clock"
)

// splash_reduced_motion_test.go — reduced motion renders the splash as a
// static still: no fade-in, no choreography, no dissolve.

func TestSplashReducedMotionIsStatic(t *testing.T) {
	m, s := newTestModel(t, nil)
	defer func() { _ = s.Close() }()
	m.clock = clock.Fixed(DumpFixedTime)
	m.cfg.UI.Ambient = "both"
	m.cfg.UI.ReducedMotion = true
	m.ambientSceneName = "night"
	m.startSplashIfEnabled(true)
	up, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = up.(Model)

	m.ambientT = time.Duration(100) * time.Millisecond
	early := m.View()
	m.ambientT = time.Duration(2900) * time.Millisecond
	late := m.View()
	if early != late {
		t.Fatal("reduced motion splash changed across time — expected a static still")
	}
}

func TestSplashReducedMotionShowsWordmarkImmediately(t *testing.T) {
	m, s := newTestModel(t, nil)
	defer func() { _ = s.Close() }()
	m.clock = clock.Fixed(DumpFixedTime)
	m.cfg.UI.Ambient = "both"
	m.cfg.UI.ReducedMotion = true
	m.ambientSceneName = "night"
	m.startSplashIfEnabled(true)
	up, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = up.(Model)
	m.ambientT = time.Duration(10) * time.Millisecond
	grid := parseFrame(m.View())
	n := 0
	for y := 3; y < 20; y++ {
		for x := 15; x < 85; x++ {
			if fg := grid.foreground(y, x); fg != "" && fgCellLum(fg) > 0.55 {
				n++
			}
		}
	}
	if n < 40 {
		t.Fatalf("reduced motion splash shows %d bright cells in the wordmark band, want >= 40 (fully-lit letters from the first frame)", n)
	}
}
