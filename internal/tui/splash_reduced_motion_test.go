package tui

import (
	"strings"
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
	plain := StripANSI(m.View())
	// the wordmark is present from the first frame
	if !strings.Contains(plain, "██   ██") && !strings.Contains(plain, "█▀") {
		t.Fatal("reduced motion splash did not show the wordmark immediately")
	}
}
