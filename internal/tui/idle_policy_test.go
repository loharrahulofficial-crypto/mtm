package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loharrahulofficial-crypto/mtm/internal/core"
)

// ambientPolicyModel builds a fresh model with a real sidebar window available.
func ambientPolicyModel(t *testing.T) Model {
	t.Helper()
	m, s := newTestModel(t, nil)
	t.Cleanup(func() { _ = s.Close() })
	m.cfg.UI.Ambient = "both"
	m.cfg.UI.AmbientSidebar = true
	m.cfg.UI.Scene = "night"
	return m
}

func seatIdleFor(t *testing.T, m Model, d time.Duration) Model {
	t.Helper()
	m.lastInput = m.clock.Now().Add(-d)
	return m
}

func TestAmbientFPSDefaultsAndCaps(t *testing.T) {
	m := ambientPolicyModel(t)
	if got := ambientFPS(m); got != 8 {
		t.Fatalf("default Home cap = %d, want 8", got)
	}
	m.cfg.UI.AmbientFPS = 12
	if got := ambientFPS(m); got != 8 {
		t.Fatalf("Home cap with fps=12 = %d, want 8 (cap lowered)", got)
	}
	m.cfg.UI.AmbientFPS = 24
	if got := ambientFPS(m); got != 8 {
		t.Fatalf("Home cap with fps=24 = %d, want 8 (cap beats user)", got)
	}
	m.mode = modeFocus
	m.timer = core.TimerView{Phase: core.TimerPhaseWork, Running: true}
	if got := ambientFPS(m); got != 24 {
		t.Fatalf("Focus cap with timer running = %d, want 24", got)
	}
	m.mode = modeSplash
	if got := ambientFPS(m); got != 24 {
		t.Fatalf("Splash cap = %d, want 24", got)
	}
}

func TestAmbientIdleSlowThenPaused(t *testing.T) {
	m := ambientPolicyModel(t)
	if got := m.ambientIdleState(); got != idleActive {
		t.Fatalf("fresh model idle = %v, want active", got)
	}
	m = seatIdleFor(t, m, 31*time.Second)
	if got := m.ambientIdleState(); got != idleSlow {
		t.Fatalf("31 s idle = %v, want slow", got)
	}
	if got := ambientFPS(m); got != idleSlowFPS {
		t.Fatalf("slow-idle FPS = %d, want %d", got, idleSlowFPS)
	}
	m = seatIdleFor(t, m, 6*time.Minute)
	if got := m.ambientIdleState(); got != idlePaused {
		t.Fatalf("6 min idle = %v, want paused", got)
	}
	if m.ambientActive() {
		t.Fatal("paused model still reports ambientActive")
	}
	if cmd := m.armAmbient(); cmd != nil {
		t.Fatal("paused model re-arms the ticker")
	}
}

func TestAmbientIdlePausePolicySkipsSlowStage(t *testing.T) {
	m := ambientPolicyModel(t)
	m.cfg.UI.AmbientIdle = "pause"
	m = seatIdleFor(t, m, 10*time.Second)
	if got := m.ambientIdleState(); got != idleActive {
		t.Fatalf("10 s idle pause policy = %v, want active", got)
	}
	m = seatIdleFor(t, m, 31*time.Second)
	if got := m.ambientIdleState(); got != idlePaused {
		t.Fatalf("31 s idle pause policy = %v, want paused (no slow stage)", got)
	}
}

func TestAmbientIdleOffNeverThrottles(t *testing.T) {
	m := ambientPolicyModel(t)
	m.cfg.UI.AmbientIdle = "off"
	m = seatIdleFor(t, m, 30*time.Minute)
	if got := m.ambientIdleState(); got != idleActive {
		t.Fatalf("off policy after 30 min = %v, want active", got)
	}
	if !m.ambientActive() {
		t.Fatal("off policy after 30 min: scene should still be active")
	}
}

func TestAmbientIdleResumesOnInput(t *testing.T) {
	m := ambientPolicyModel(t)
	m = seatIdleFor(t, m, 6*time.Minute)
	if m.ambientActive() {
		t.Fatal("expected paused at 6 min idle")
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(Model)
	if !m.ambientActive() {
		t.Fatal("a keypress did not resume the ambient scene")
	}
	if cmd == nil {
		t.Fatal("no arm command after resume")
	}
	m = seatIdleFor(t, m, 6*time.Minute)
	updated, _ = m.Update(tea.MouseMsg{Type: tea.MouseLeft, X: 3, Y: 3})
	m = updated.(Model)
	if !m.ambientActive() {
		t.Fatal("a mouse event did not resume the ambient scene")
	}
}

func TestAmbientIdleFocusExemptWhileTimerRuns(t *testing.T) {
	m := ambientPolicyModel(t)
	m.mode = modeFocus
	m.timer = core.TimerView{Phase: core.TimerPhaseWork, Running: true}
	m = seatIdleFor(t, m, 10*time.Minute)
	if got := m.ambientIdleState(); got != idleActive {
		t.Fatalf("Focus with running timer, 10 min idle = %v, want active (exempt)", got)
	}
	if !m.ambientActive() {
		t.Fatal("Focus with running timer should keep animating")
	}
	// Focus with an idle timer is subject to the policy.
	m.timer = core.TimerView{Phase: core.TimerPhaseIdle}
	m = seatIdleFor(t, m, 6*time.Minute)
	if got := m.ambientIdleState(); got != idlePaused {
		t.Fatalf("Focus with idle timer, 6 min idle = %v, want paused", got)
	}
	// Focus with a paused-but-not-idle timer is also subject.
	m.timer = core.TimerView{Phase: core.TimerPhaseWork, Running: false}
	if got := m.ambientIdleState(); got != idlePaused {
		t.Fatalf("Focus with paused timer, 6 min idle = %v, want paused", got)
	}
}

func TestAmbientIdleSplashExempt(t *testing.T) {
	m := ambientPolicyModel(t)
	m.mode = modeSplash
	m = seatIdleFor(t, m, 10*time.Minute)
	if got := m.ambientIdleState(); got != idleActive {
		t.Fatalf("splash after 10 min idle = %v, want active (exempt)", got)
	}
}

func TestAmbientPausesWhenUnfocused(t *testing.T) {
	m := ambientPolicyModel(t)
	if !m.ambientActive() {
		t.Fatal("precondition: scene should be active")
	}
	updated, _ := m.Update(tea.BlurMsg{})
	m = updated.(Model)
	if m.ambientActive() {
		t.Fatal("scene still active after the terminal blurred")
	}
	if cmd := m.armAmbient(); cmd != nil {
		t.Fatal("blurred model re-arms the ticker")
	}
	updated, _ = m.Update(tea.FocusMsg{})
	m = updated.(Model)
	if !m.ambientActive() {
		t.Fatal("scene did not resume when the terminal regained focus")
	}
}

func TestAmbientPausesWhenAnotherViewHidesWindow(t *testing.T) {
	m := ambientPolicyModel(t)
	m.projectPage = "college/dbms"
	if m.ambientActive() {
		t.Fatal("scene active while the project page hides the window")
	}
	if cmd := m.armAmbient(); cmd != nil {
		t.Fatal("project page re-arms the ambient ticker")
	}
}

func TestAmbientPausesWhenWindowCollapses(t *testing.T) {
	m := ambientPolicyModel(t)
	// Short terminal: the sidebar window no longer fits (<8 rows), so
	// the scene has no surface and pauses.
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 16})
	m = updated.(Model)
	_, _, ok := sidebarFreeRows(m)
	if ok {
		t.Skip("fixture: window still fits at 100x16 on this machine")
	}
	if m.ambientActive() {
		t.Fatal("scene active while the sidebar window collapsed")
	}
	if cmd := m.armAmbient(); cmd != nil {
		t.Fatal("collapsed window re-arms the ambient ticker")
	}
}
